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
	desiredReplicas := uint64(1)                                                      // The desired number of replicas

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

	// Update the service using the service ID
	if err := updateServiceReplicas(cli, serviceID, desiredReplicas); err != nil {
		fmt.Printf("Error updating service replicas: %s\n", err)
		os.Exit(1)
	}

	fmt.Println("Service updated successfully.")
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

// updateServiceReplicas updates the number of replicas for the given service ID.
func updateServiceReplicas(cli *client.Client, serviceID string, replicas uint64) error {
	ctx := context.Background()

	// Get the service by ID
	service, _, err := cli.ServiceInspectWithRaw(ctx, serviceID, types.ServiceInspectOptions{})
	if err != nil {
		return err
	}

	// Update the number of replicas in the service spec
	if service.Spec.Mode.Replicated != nil {
		service.Spec.Mode.Replicated.Replicas = &replicas
	} else {
		return fmt.Errorf("service mode is not replicated")
	}

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
