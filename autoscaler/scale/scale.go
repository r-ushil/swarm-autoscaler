package scale

import (
	"context"
	"fmt"
	"time"
	"sync"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"bpf_port_listen"
)



type ScaleManager struct {
	cli *client.Client
	mu sync.Mutex
}

var instance *ScaleManager
var once sync.Once

func GetClient() *client.Client {
	once.Do(func() {
		instance = &ScaleManager{}
		instance.initClient()
	})
	return instance.cli
}

func (manager *ScaleManager) initClient() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic("Failed to create Docker client: " + err.Error()) // handle the error appropriately for your application
	}
	manager.cli = cli
}


// ScaleService adjusts the service replicas based on the direction for the given container ID.
func ScaleService(containerID string, direction string) error {
	err := changeServiceReplicas(containerID, direction)
	if err != nil {
		return fmt.Errorf("error changing service replicas: %w", err)
	}

	fmt.Println("Service replica count changed successfully.")
	return nil
}

// findServiceIDFromContainer inspects the container to find its associated service ID.
func findServiceIDFromContainer(containerID string) (string, error) {
	ctx := context.Background()
	cli := GetClient()
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

// changeServiceReplicas changes the number of replicas for the given service ID based on direction.
func changeServiceReplicas(containerID string, direction string) error {
	ctx := context.Background()
	cli := GetClient()

	serviceID, err := findServiceIDFromContainer(containerID)
	if err != nil {
		return fmt.Errorf("error finding service ID from container: %w", err)
	}

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
			newReplicas = currentReplicas + 1
		} else if direction == "under" {
			// Ensure we don't go below 1 replica
			if currentReplicas > 1 {
				newReplicas = currentReplicas - 1
			} else {
				// scale to 0
				ScaleTo(serviceID, 0)

				port, err := getPublishedPort(serviceID)
				if err != nil {
					return err
				}

				fmt.Printf("Adding port %d from BPF listener\n", port)

				// get BPF port listener instance
				listener, err := bpf_port_listen.GetBPFListener("wlp60s0")
				if err != nil {
					return err
				}

				// add port to BPF listener
				if err := listener.AddPort(port, serviceID); err != nil {
					return err
				}

				fmt.Printf("Added port %d to BPF listener\n", port)	
				
				return nil
			}
		} else {
			return fmt.Errorf("invalid direction: %s", direction)
		}
	} else {
		return fmt.Errorf("service mode is not replicated or replicas are not set")
	}

	ScaleTo(serviceID, newReplicas)

	return nil
}


// scale service to number of replicas
func ScaleTo(serviceID string, replicas uint64) error {
	ctx := context.Background()
	cli := GetClient()

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
	cli := GetClient()

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
func NewEventNotifier() *EventNotifier {
	return &EventNotifier{
		StartChan: make(chan string, 10), // Buffered channels for start and stop events
		StopChan:  make(chan string, 10),
	}
}

// ListenForEvents starts listening for Docker container start and stop events.
func (en *EventNotifier) ListenForEvents(ctx context.Context) {
	cli := GetClient()

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
func GetRunningContainers(ctx context.Context) ([]string, error) {
    cli := GetClient()

    containers, err := cli.ContainerList(ctx, container.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("error listing Docker containers: %w", err)
    }

    var containerIDs []string
    for _, container := range containers {
        containerIDs = append(containerIDs, container.ID)
    }

    return containerIDs, nil
}
