package scale

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	cli *client.Client
	portListener PortListener
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

func (manager *ScaleManager) initScaler() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic("Failed to create Docker client: " + err.Error()) // handle the error appropriately for your application
	}
	manager.cli = cli
	manager.portListener = nil
}


// ScaleService adjusts the service replicas based on the direction for the given container ID.
func ScaleService(containerID string, direction string) error {

	serviceID, err := findServiceIDFromContainer(containerID)
	if err != nil {
		return fmt.Errorf("error finding service ID from container: %w", err)
	}

	err = ChangeServiceReplicas(serviceID, direction)
	if err != nil {
		return fmt.Errorf("error changing service replicas: %w", err)
	}

	fmt.Println("Service replica count changed successfully.")
	return nil
}

// findServiceIDFromContainer inspects the container to find its associated service ID.
func findServiceIDFromContainer(containerID string) (string, error) {
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


// send scale request to manager node from worker node
func SendScaleRequest(containerID string, direction string, managerIP string) error {
	serviceId, err := findServiceIDFromContainer(containerID)

	if err != nil {
		return fmt.Errorf("error finding service ID from container: %w", err)
	}

	data := map[string]string{"serviceId": serviceId, "direction": direction}
	jsonData, err := json.Marshal(data)

	if err != nil {
		return fmt.Errorf("error marshalling JSON data: %w", err)
	}

	resp, err := http.Post("http://"+managerIP+":4567/scale", "application/json", bytes.NewBuffer(jsonData))

	if err != nil {
		return fmt.Errorf("error sending scale request to manager node: %w", err)
	}

	defer resp.Body.Close()

	return nil

}


func updateServiceConstraints(service swarm.Service, add bool) error {
	ctx := context.Background()
	cli := instance.cli

	hostName, exists := service.Spec.Labels["autoscaler.handlerNode"]
	if !exists {
		return fmt.Errorf("handlerNode label not found on service %s in updateServiceConstraints", service.ID)
	}

	// Update the service with the new constraints
	serviceSpec := service.Spec
	if add {
		fmt.Printf("Adding constraint to service %s to run on hostname %s\n", service.ID, hostName)
		serviceSpec.TaskTemplate.Placement.Constraints = []string{"node.hostname=="+hostName}
	} else {
		if serviceSpec.TaskTemplate.Placement.Constraints != nil {
			fmt.Printf("Removing constraint from service %s to run on hostname %s\n", service.ID, hostName)
			serviceSpec.TaskTemplate.Placement.Constraints = nil
		}
	}

	_, err := cli.ServiceUpdate(ctx, service.ID, service.Version, serviceSpec, types.ServiceUpdateOptions{})

	return err
}

// changeServiceReplicas changes the number of replicas for the given service ID based on direction.
func ChangeServiceReplicas(serviceID string, direction string) error {
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

				// add port to listener
				if err := instance.portListener.ListenOnPort(port, serviceID); err != nil {
					return fmt.Errorf("failed to listen on port: %w", err)
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

    fmt.Printf("Service %s scaled to %d replicas\n", serviceID, replicas)

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

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("Error getting hostname in ListenForEvents:", err)
		os.Exit(1)
	}

	filters := filters.NewArgs(
		filters.Arg("type", "container"),
		filters.Arg("label", "autoscaler.handlerNode="+hostname),
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
				fmt.Printf("Container started: %s\n", event.ID)
				en.StartChan <- event.ID
			case "die":
				fmt.Printf("Container stopped: %s\n", event.ID)
				en.StopChan <- event.ID
			}
		case err := <-errsCh:
			if err != nil {
				fmt.Println("Error receiving Docker events:", err)
				return
			}
		case <-ctx.Done():
			fmt.Println("Stopped listening for Docker events.")
			return
		}
	}
}

// GetRunningContainers returns a slice of container IDs for all currently running containers.
func (s ScaleManager) GetRunningContainers(ctx context.Context) ([]string, error) {
    cli := instance.cli

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println("Error getting hostname in GetRunningContainers:", err)
		os.Exit(1)
	}

	filters := filters.NewArgs()
	filters.Add("label", "autoscaler.handlerNode="+hostname)

	listOptions := container.ListOptions{
		Filters: filters,
	}

    containers, err := cli.ContainerList(ctx, listOptions)
    if err != nil {
        return nil, fmt.Errorf("error listing Docker containers: %w", err)
    }

    var containerIDs []string
    for _, container := range containers {

		containerIDs = append(containerIDs, container.ID)

    }

	fmt.Printf("Found %d running containers\n", len(containerIDs))

    return containerIDs, nil
}
