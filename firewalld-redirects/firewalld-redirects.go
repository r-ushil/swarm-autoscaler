package main

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"io"
	"net"
	"os/exec"
	"strings"
	"sync"
)

var (
	bufferMutex   sync.Mutex
	bufferedConns []net.Conn
)

func main() {
	const (
		serviceName = "webserver"
		containerID = "cb76f781df58f459a52623045d40eb7c5cd29fd7625a50bf86e0a58d5e11eb79"
		listenPort  = "5555"
	)

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Printf("Error creating Docker client: %v\n", err)
		return
	}

	containerIP, targetPort, err := getSwarmServiceNetworkInfo(cli, serviceName)
	if err != nil {
		fmt.Printf("Error fetching container network info: %v\n", err)
		return
	}
	fmt.Printf("Container IP: %s, Target Port: %s\n", containerIP, targetPort)

	if err := addFirewalldRule(targetPort, listenPort); err != nil {
		fmt.Printf("Error setting up firewalld rule: %v\n", err)
		return
	}

	if err := reloadFirewalld(); err != nil {
		fmt.Printf("Error reloading firewalld: %v\n", err)
		return
	}

	fmt.Println("Pausing the container...")
	if err := pauseContainer(containerID); err != nil {
		fmt.Printf("Error pausing container: %v\n", err)
		return
	}

	go tcpProxy(listenPort, cli, containerID, containerIP, targetPort)

	select {}
}

func tcpProxy(listenPort string, cli *client.Client, containerID, containerIP, targetPort string) {
	ln, err := net.Listen("tcp", ":"+listenPort)
	if err != nil {
		panic(fmt.Sprintf("Failed to set up listener: %v", err))
	}
	defer ln.Close()

	fmt.Println("TCP proxy listening for incoming connections...")
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}

		bufferMutex.Lock()
		bufferedConns = append(bufferedConns, conn)
		fmt.Println("Connection buffered.")
		if len(bufferedConns) == 1 {
			// Check if container is paused, then proceed
			isPaused, err := isContainerPaused(cli, containerID)
			if err != nil {
				fmt.Printf("Error checking container status: %v\n", err)
				bufferMutex.Unlock()
				continue
			}
			if isPaused {
				fmt.Println("Unpausing the container...")
				if err := unpauseContainer(containerID); err != nil {
					fmt.Printf("Error unpausing container: %v\n", err)
					bufferMutex.Unlock()
					continue
				}

				fmt.Println("Removing firewalld rule...")
				if err := removeFirewalldRule(targetPort, listenPort); err != nil {
					fmt.Printf("Error removing firewalld rule: %v\n", err)
				}

				fmt.Println("Reloading firewalld...")
				if err := reloadFirewalld(); err != nil {
					fmt.Printf("Error reloading firewalld: %v\n", err)
				}

				fmt.Println("Flushing buffered connections to the container...")
				go flushBufferedConnections(containerIP, targetPort)
			}
		}
		bufferMutex.Unlock()
	}
}

func flushBufferedConnections(containerIP, containerPort string) {
	bufferMutex.Lock()
	conns := bufferedConns
	bufferedConns = nil
	bufferMutex.Unlock()

	for _, conn := range conns {
		go func(c net.Conn) {
			defer c.Close()
			containerConn, err := net.Dial("tcp", fmt.Sprintf("%s:%s", containerIP, containerPort))
			if err != nil {
				fmt.Println("Error connecting to container:", err)
				return
			}
			defer containerConn.Close()

			io.Copy(containerConn, c)
		}(conn)
	}
}

func getSwarmServiceNetworkInfo(cli *client.Client, serviceNameOrID string) (string, string, error) {
	ctx := context.Background()

	// Attempt to get the service by ID directly
	service, _, err := cli.ServiceInspectWithRaw(ctx, serviceNameOrID, types.ServiceInspectOptions{})
	if err != nil {
		// If not found by ID, try to filter by name
		serviceFilter := filters.NewArgs()
		serviceFilter.Add("name", serviceNameOrID)
		services, err := cli.ServiceList(ctx, types.ServiceListOptions{Filters: serviceFilter})
		if err != nil {
			return "", "", fmt.Errorf("failed to list services: %w", err)
		}
		if len(services) == 0 {
			return "", "", fmt.Errorf("no service found with ID or name %s", serviceNameOrID)
		}
		service = services[0] // Use the first matching service
	}

	// Extract the published port from the service
	var targetPort string
	if len(service.Endpoint.Ports) > 0 {
		targetPort = fmt.Sprintf("%d", service.Endpoint.Ports[0].PublishedPort)
	}

	// Get the task (container) IP address
	taskFilter := filters.NewArgs()
	taskFilter.Add("service", service.ID)
	tasks, err := cli.TaskList(ctx, types.TaskListOptions{Filters: taskFilter})
	if err != nil {
		return "", "", fmt.Errorf("failed to list tasks for service %s: %w", service.ID, err)
	}

	for _, task := range tasks {
		if task.Status.State == swarm.TaskStateRunning {
			// Assuming the first running task is the target
			for _, networkAttachment := range task.NetworksAttachments {
				// This check is simplified to just grab the first available IP address
				if len(networkAttachment.Addresses) > 0 {
					ipCIDR := networkAttachment.Addresses[0]
					ip := ipCIDR[:strings.Index(ipCIDR, "/")]
					return ip, targetPort, nil
				}
			}
		}
	}

	return "", "", fmt.Errorf("no running tasks found for service %s", serviceNameOrID)
}


func isContainerPaused(cli *client.Client, containerID string) (bool, error) {
	ctx := context.Background()
	containerJSON, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}

	return containerJSON.State.Paused, nil
}

func pauseContainer(containerID string) error {
	cmd := exec.Command("docker", "pause", containerID)
	return cmd.Run()
}

func unpauseContainer(containerID string) error {
	cmd := exec.Command("docker", "unpause", containerID)
	return cmd.Run()
}

func addFirewalldRule(fromPort, toPort string) error {
	cmd := exec.Command("firewall-cmd", "--zone=docker", "--add-forward-port=port="+fromPort+":proto=tcp:toport="+toPort)
	return cmd.Run()
}

func reloadFirewalld() error {
	cmd := exec.Command("firewall-cmd", "--reload")
	return cmd.Run()
}

func removeFirewalldRule(fromPort, toPort string) error {
	cmd := exec.Command("firewall-cmd", "--zone=docker", "--remove-forward-port=port="+fromPort+":proto=tcp:toport="+toPort)
	return cmd.Run()
}
