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
)

type PortListener interface {
	ListenOnPort(port uint32, serviceID string) error
}

type ScaleManager struct {
	cli          *client.Client
	portListener PortListener
	nodeInfo     server.SwarmNodeInfo
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
		// return fmt.Errorf("handlerNode label not found on service %s in updateServiceConstraints", service.ID)
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

// changeServiceReplicas changes the number of replicas for the given service ID based on direction.
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
					return fmt.Errorf("error adding constraint to service: %w", err)
				}
			}
			newReplicas = currentReplicas + 1
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

				port, err := getPublishedPort(serviceID)
				if err != nil {
					return err
				}

				if err := instance.portListener.ListenOnPort(port, serviceID); err != nil {
					return fmt.Errorf("failed to listen on port: %w", err)
				}

				// add port to all listeners by making a request to the server
				err = server.SendListenRequestToAllNodes(instance.nodeInfo, port, serviceID)

				if err != nil {
					return fmt.Errorf("error sending listen request to all nodes: %w", err)
				}

				// scale to 0
				scaleTo(serviceID, 0)

				return nil
			}
		} else {
			return fmt.Errorf("invalid direction: %s", direction)
		}
	} else {
		return fmt.Errorf("service mode is not replicated or replicas are not set")
	}

	scaleTo(serviceID, newReplicas)

	return nil
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

func getPublishedPort(serviceID string) (uint32, error) {
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