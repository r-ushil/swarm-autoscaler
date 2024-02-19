package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"os"
)

func main() {
	containerID := "df82c3d455c96dc80c3d871da6ced03ff3108bb951e9d2ad96151751de0f4267"
	direction := "under" // "over" to increase, "under" to decrease replicas

	// Create a new Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Printf("Error creating Docker client: %s\n", err)
		os.Exit(1)
	}

	// Find the service ID from the container ID
	serviceID, err := findServiceIDFromContainer(cli, containerID)
	if err != nil {
		fmt.Printf("Error finding service ID from container: %s\n", err)
		os.Exit(1)
	}

	// Update the service based on the direction
	if err := changeServiceReplicas(cli, serviceID, direction); err != nil {
		fmt.Printf("Error changing service replicas: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("Service replica count changed successfully.")
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
func changeServiceReplicas(cli *client.Client, serviceID string, direction string) error {
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
				return fmt.Errorf("service already has the minimum number of replicas: 1")
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
	response, err := cli.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, updateOpts)
	if err != nil {
		return err
	}

	fmt.Printf("Service update response: %+v\n", response)
	return nil
}
