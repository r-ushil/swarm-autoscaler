package scale

import (
	"context"
	"fmt"
	"logging"
	"os"
	"server"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"golang.org/x/sys/unix"
)

type PortListener interface {
	ListenOnPort(port uint32, serviceID string) error
}

type ScaleManager struct {
	cli          *client.Client
	portListener PortListener
	nodeInfo     server.SwarmNodeInfo
	keepAliveOps map[string]chan bool
}

var instance *ScaleManager
var once sync.Once

func GetScaler() *ScaleManager {
	once.Do(func() {
		instance = &ScaleManager{}
		instance.initScaler()
	})
	return instance
}

func (manager *ScaleManager) SetPortListener(listener PortListener) {
	manager.portListener = listener
}

func (manager *ScaleManager) SetNodeInfo(nodeInfo server.SwarmNodeInfo) {
	manager.nodeInfo = nodeInfo
}

func (manager *ScaleManager) initScaler() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic("Failed to create Docker client: " + err.Error())
	}
	manager.cli = cli
	manager.portListener = nil
	manager.nodeInfo = server.SwarmNodeInfo{}
	manager.keepAliveOps = make(map[string]chan bool)
}

// ScaleService adjusts the service replicas based on the direction for the given container ID.
func ScaleService(containerID string, direction string) error {
	serviceID, err := FindServiceIDFromContainer(containerID)
	if err != nil {
		return fmt.Errorf("error finding service ID from container: %w", err)
	}

	err = ChangeServiceReplicas(serviceID, direction)
	if err != nil {
		return fmt.Errorf("error changing service replicas: %w", err)
	}

	return nil
}

// FindServiceIDFromContainer inspects the container to find its associated service ID.
func FindServiceIDFromContainer(containerID string) (string, error) {
	ctx := context.Background()
	cli := instance.cli
	container, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}

	// Retrieve the service ID from the container's labels
	serviceID, ok := container.Config.Labels["com.docker.swarm.service.id"]
	if !ok {
		return "", fmt.Errorf("service ID label not found on container")
	}

	return serviceID, nil
}

func updateServiceConstraints(service swarm.Service, add bool) error {
	ctx := context.Background()
	cli := instance.cli

	hostName, exists := service.Spec.Labels["autoscaler.handlerNode"]
	if !exists {
		// Return nil when using implicit ownership
		return nil
	}

	// Update the service with the new constraints
	serviceSpec := service.Spec
	if add {
		logging.AddEventLog(fmt.Sprintf("Adding constraint to service %s to run on hostname %s", service.ID, hostName))
		serviceSpec.TaskTemplate.Placement.Constraints = []string{"node.hostname==" + hostName}
	} else {
		if serviceSpec.TaskTemplate.Placement.Constraints != nil {
			logging.AddEventLog(fmt.Sprintf("Removing constraint from service %s to run on hostname %s", service.ID, hostName))
			serviceSpec.TaskTemplate.Placement.Constraints = nil
		}
	}

	_, err := cli.ServiceUpdate(ctx, service.ID, service.Version, serviceSpec, types.ServiceUpdateOptions{})

	return err
}

// ChangeServiceReplicas changes the number of replicas for the given service ID based on direction.
// only runs on manager node
func ChangeServiceReplicas(serviceID string, direction string) error {
	// should never be called on a non-manager node
	if !instance.nodeInfo.AutoscalerManager {
		return fmt.Errorf("changeServiceReplicas should only be called on manager node")
	}

	ctx := context.Background()
	cli := instance.cli

	// Get the service by ID
	service, _, err := cli.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	if err != nil {
		return err
	}

	// Determine the new desired number of replicas
	var newReplicas uint64
	if service.Spec.Mode.Replicated != nil && service.Spec.Mode.Replicated.Replicas != nil {
		currentReplicas := *service.Spec.Mode.Replicated.Replicas
		if direction == "over" {
			if currentReplicas == 1 {
				// Remove the constraint to run on the specified hostname
				if err := updateServiceConstraints(service, false); err != nil {
					return fmt.Errorf("error removing constraint from service: %w", err)
				}
			}
			newReplicas = currentReplicas + 1

			// Cancel keep-alive operation if exists
			if keepAliveCh, exists := instance.keepAliveOps[serviceID]; exists {
				keepAliveCh <- true
				delete(instance.keepAliveOps, serviceID)
				logging.AddEventLog(fmt.Sprintf("Cancelled KeepAlive operation for service %s due to scaling up", serviceID))
			}

		} else if direction == "under" {
			// Ensure we don't go below 1 replica
			if currentReplicas > 1 {
				newReplicas = currentReplicas - 1
				if newReplicas == 1 {
					// Add a constraint to run on the specified hostname
					if err := updateServiceConstraints(service, true); err != nil {
						return fmt.Errorf("error adding constraint to service: %w", err)
					}
				}
			} else {
				if _, exists := instance.keepAliveOps[serviceID]; exists {
					logging.AddEventLog(fmt.Sprintf("Ignoring scaling request for service %s due to existing KeepAlive operation", serviceID))
					return nil
				}

				// Create a channel to signal keep-alive operation
				keepAliveCh := make(chan bool)
				instance.keepAliveOps[serviceID] = keepAliveCh

				// Start the keep-alive goroutine
				go keepAliveAndScaleDown(serviceID, keepAliveCh)

				logging.AddEventLog(fmt.Sprintf("Started KeepAlive operation for service %s", serviceID))
				return nil
			}
		} else {
			return fmt.Errorf("invalid direction: %s", direction)
		}
	} else {
		return fmt.Errorf("service mode is not replicated or replicas are not set")
	}

	return scaleTo(serviceID, newReplicas)
}

func (s *ScaleManager) ScaleTo(serviceID string, replicas uint64) error {
	return scaleTo(serviceID, replicas)
}

// scale service to number of replicas
func scaleTo(serviceID string, replicas uint64) error {
	ctx := context.Background()
	cli := instance.cli

	service, _, err := cli.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	if err != nil {
		return err
	}

	// Set the replicas to the desired number
	service.Spec.Mode.Replicated.Replicas = &replicas

	updateOpts := types.ServiceUpdateOptions{}
	_, err = cli.ServiceUpdate(ctx, serviceID, service.Version, service.Spec, updateOpts)
	if err != nil {
		return err
	}

	logging.AddServiceLog(serviceID, uint32(replicas))

	logging.AddEventLog(fmt.Sprintf("Scaled service %s to %d replicas", serviceID, replicas))

	return nil
}

func GetPublishedPort(serviceID string) (uint32, error) {
	ctx := context.Background()
	cli := instance.cli

	service, _, err := cli.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	if err != nil {
		return 0, err
	}

	var publishedPort uint32
	if len(service.Endpoint.Ports) > 0 {
		// Assuming we are interested in the first port
		publishedPort = service.Endpoint.Ports[0].PublishedPort
	} else {
		return 0, fmt.Errorf("no published ports found for service %s", serviceID)
	}

	return publishedPort, nil
}

// keepAliveAndScaleDown handles the keep-alive logic and scales down the service after the keep-alive period
func keepAliveAndScaleDown(serviceID string, keepAliveCh chan bool) {
	select {
	case <-time.After(instance.nodeInfo.KeepAlive):
		if _, exists := instance.keepAliveOps[serviceID]; exists {
			logging.AddEventLog(fmt.Sprintf("Completed KeepAlive operation for service %s", serviceID))
			port, err := GetPublishedPort(serviceID)
			if err != nil {
				logging.AddEventLog(fmt.Sprintf("Error getting published port for service %s: %v", serviceID, err))
				return
			}

			if err := instance.portListener.ListenOnPort(port, serviceID); err != nil {
				logging.AddEventLog(fmt.Sprintf("Failed to listen on port for service %s: %v", serviceID, err))
				return
			}

			err = server.SendListenRequestToAllNodes(instance.nodeInfo, port, serviceID)
			if err != nil {
				logging.AddEventLog(fmt.Sprintf("Error sending listen request to all nodes for service %s: %v", serviceID, err))
				return
			}

			err = scaleTo(serviceID, 0)
			if err != nil {
				logging.AddEventLog(fmt.Sprintf("Error scaling service %s to 0: %v", serviceID, err))
				return
			}
			
			// Remove the keep-alive operation from the map
			delete(instance.keepAliveOps, serviceID)
			
		}
	case <-keepAliveCh:
		// Keep-alive operation was canceled
		logging.AddEventLog(fmt.Sprintf("KeepAlive operation for service %s was canceled", serviceID))
	}
}

// EventNotifier notifies about container start and stop events.
type EventNotifier struct {
	StartChan chan string
	StopChan  chan string
}

// NewEventNotifier creates and returns a new EventNotifier.
func (s *ScaleManager) NewEventNotifier() *EventNotifier {
	return &EventNotifier{
		StartChan: make(chan string, 10), // Buffered channels for start and stop events
		StopChan:  make(chan string, 10),
	}
}

// ListenForEvents starts listening for Docker container start and stop events.
func (en *EventNotifier) ListenForEvents(ctx context.Context) {
	cli := instance.cli

	filters := filters.NewArgs(
		filters.Arg("type", "container"),
		filters.Arg("event", "start"),
		filters.Arg("event", "die"),
	)

	options := types.EventsOptions{
		Filters: filters,
		Since:   fmt.Sprintf("%d", time.Now().Unix()),
	}

	eventsCh, errsCh := cli.Events(ctx, options)

	for {
		select {
		case event := <-eventsCh:
			switch event.Action {
			case "start":
				owned, err := CheckOwnedContainer(event.ID)
				if err != nil {
					logging.AddEventLog(fmt.Sprintf("Error checking ownership of container %s: %v", event.ID, err))
					continue
				}
				if owned {
					logging.AddContainerLog(event.ID, 0.0)
					logging.AddEventLog(fmt.Sprintf("Container started: %s", event.ID))
					en.StartChan <- event.ID
				}
			case "die":
				owned, err := CheckOwnedContainer(event.ID)
				if err != nil {
					logging.AddEventLog(fmt.Sprintf("Error checking ownership of container %s: %v", event.ID, err))
					continue
				}

				if owned {
					logging.AddEventLog(fmt.Sprintf("Container stopped: %s", event.ID))
					logging.RemoveContainerLog(event.ID)
					en.StopChan <- event.ID
				}
			}
		case err := <-errsCh:
			if err != nil {
				logging.AddEventLog(fmt.Sprintf("Error receiving Docker events: %v", err))
				return
			}
		case <-ctx.Done():
			logging.AddEventLog("Stopped listening for Docker events.")
			return
		}
	}
}

// GetRunningContainers returns a slice of container IDs for all currently running containers.
func (s ScaleManager) GetRunningContainers(ctx context.Context) ([]string, error) {
	cli := instance.cli
	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing Docker containers: %w", err)
	}

	var containerIDs []string
	for _, container := range containers {
		owned, err := CheckOwnedContainer(container.ID)
		if err != nil {
			logging.AddEventLog(fmt.Sprintf("Error checking ownership of container %s: %v", container.ID, err))
			continue
		}
		if owned {
			containerIDs = append(containerIDs, container.ID)
		}
	}

	logging.AddEventLog(fmt.Sprintf("Found %d running containers", len(containerIDs)))

	return containerIDs, nil
}

func CheckOwnedContainer(containerID string) (bool, error) {
	ctx := context.Background()
	cli := instance.cli

	hostname, err := os.Hostname()
	if err != nil {
		logging.AddEventLog(fmt.Sprintf("Error getting hostname in CheckOwnedContainer: %v", err))
		return false, err
	}

	container, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}

	// check if explicit ownership
	hostnameLabel, exists := container.Config.Labels["autoscaler.handlerNode"]
	if exists {
		return hostnameLabel == hostname, nil
	}

	// check if implicit ownership
	taskName, exists := container.Config.Labels["com.docker.swarm.task.name"]
	if exists && strings.Contains(taskName, ".1.") {
		return true, nil
	}

	return false, nil
}

func GetContainerNamespace(containerID string) (uint32, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        return 0, err
    }

    inspect, err := cli.ContainerInspect(context.Background(), containerID)
    if err != nil {
        return 0, err
    }

	pid := inspect.State.Pid
	var procPath string

    // Check if /host_proc exists
    if _, err := os.Stat("/host_proc"); err == nil {
        // Use /host_proc if it exists
        procPath = fmt.Sprintf("/host_proc/%d/ns/net", pid)
    } else {
        // Fallback to /proc
        procPath = fmt.Sprintf("/proc/%d/ns/net", pid)
    }

    // Check if the file exists
    if _, err := os.Stat(procPath); os.IsNotExist(err) {
        fmt.Printf("Network namespace path does not exist: %v\n", err)
        return 0, err
    }

    var stat unix.Stat_t
    if err := unix.Stat(procPath, &stat); err != nil {
        fmt.Printf("Failed to stat network namespace path: %v\n", err)
        if err == unix.ENOENT {
            fmt.Println("Error: No such file or directory")
        } else if err == unix.EACCES {
            fmt.Println("Error: Permission denied")
        } else {
            fmt.Printf("Unexpected error: %v\n", err)
        }
        return 0, err
    }

    return uint32(stat.Ino), nil
}