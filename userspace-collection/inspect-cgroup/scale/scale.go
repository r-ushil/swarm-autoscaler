package scale

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"time"
	"cgroup_net_listen"
)

// ScaleService adjusts the service replicas based on the direction for the given container ID.
func ScaleService(containerID string, direction string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("error creating Docker client: %w", err)
	}

	err = changeServiceReplicas(cli, containerID, direction)
	if err != nil {
		return fmt.Errorf("error changing service replicas: %w", err)
	}

	fmt.Println("Service replica count changed successfully.")
	return nil
}

// findServiceIDFromContainer inspects the container to find its associated service ID.
func findServiceIDFromContainer(cli *client.Client, containerID string) (string, error) {
	ctx := context.Background()
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
func changeServiceReplicas(cli *client.Client, containerID string, direction string) error {

	serviceID, err := findServiceIDFromContainer(cli, containerID)
	if err != nil {
		return fmt.Errorf("error finding service ID from container: %w", err)
	}

	ctx := context.Background()

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
				scaleToZero(cli, containerID)
				return nil				
			}
		} else {
			return fmt.Errorf("invalid direction: %s", direction)
		}
	} else {
		return fmt.Errorf("service mode is not replicated or replicas are not set")
	}

	// Update the number of replicas in the service spec
	service.Spec.Mode.Replicated.Replicas = &newReplicas

	// Create an update options struct
	updateOpts := types.ServiceUpdateOptions{}

	// Update the service
	_, err = cli.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, updateOpts)
	if err != nil {
		return err
	}

	return nil
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
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Println("Error creating Docker client:", err)
		return
	}

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
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return nil, fmt.Errorf("error creating Docker client: %w", err)
    }

    containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
    if err != nil {
        return nil, fmt.Errorf("error listing Docker containers: %w", err)
    }

    var containerIDs []string
    for _, container := range containers {
        containerIDs = append(containerIDs, container.ID)
    }

    return containerIDs, nil
}


func pauseContainer(cli *client.Client, containerID string) error {
	ctx := context.Background()

	// Pause the container
	if err := cli.ContainerPause(ctx, containerID); err != nil {
		return err
	}

	return nil
}

func unpauseContainer(cli *client.Client, containerID string) error {
	ctx := context.Background()

	// Unpause the container
	if err := cli.ContainerUnpause(ctx, containerID); err != nil {
		return err
	}
	return nil
}


// there should be a function called ScaleToZero here

func scaleToZero(cli *client.Client, containerID string) error {
	// Pause the container
	fmt.Println("Scaling to zero: %s", containerID)
	if err := pauseContainer(cli, containerID); err != nil {
		return fmt.Errorf("error pausing container: %w", err)
	}

	cgroup_net_listen.SetupBPFListener(containerID)


	// Unpause the container
	if err := unpauseContainer(cli, containerID); err != nil {
		return fmt.Errorf("error unpausing container: %w", err)
	}


	fmt.Println("Scaling back to one from zero for container: %s", containerID)

	// Sleep for 5 seconds to avoid reloading BPF program straight away
	time.Sleep(5 * time.Second)

	return nil
}
