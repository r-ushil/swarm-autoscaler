package main

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go bpf ./bpf/cgroup-ingress-perf.c -- -I/usr/include/bpf -O2

import (
	"context"
	"fmt"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/rlimit"
	"github.com/docker/docker/client"
	"log"
	"os"
)


const (
	cgroupPath = "/sys/fs/cgroup/system.slice"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <containerID>", os.Args[0])
	}
	containerID := os.Args[1]

	fmt.Println("Starting the program...")

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Error creating Docker client: %v", err)
	}

	fmt.Println("Pausing the container:", containerID)
	if err := pauseContainer(ctx, cli, containerID); err != nil {
		log.Fatalf("Failed to pause container: %v", err)
	}

	fmt.Println("Removing memlock rlimit to allow eBPF program to be loaded...")
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Loading eBPF program and attaching it to the cgroup...")
	objs := bpfObjects{}
	if err := loadBpfObjects(&objs, nil); err != nil {
		log.Fatalf("loading objects: %v", err)
	}
	defer objs.Close()

	cgroupFd, err := os.Open(cgroupPath + "/docker-" + containerID + ".scope")
	if err != nil {
		log.Fatalf("Failed to open cgroup: %v", err)
	}
	defer cgroupFd.Close()

	fmt.Println("Attaching eBPF program to cgroup...")
	l, err := link.AttachCgroup(link.CgroupOptions{
		Path:    cgroupFd.Name(),
		Attach:  ebpf.AttachCGroupInetIngress,
		Program: objs.DetectFirstPacket,
	})
	if err != nil {
		log.Fatalf("Failed to attach eBPF program to cgroup: %v", err)
	}
	defer func() {
		fmt.Println("Detaching eBPF program from cgroup...")
		l.Close()
	}()

	// Listen for the first packet event
	listenForPacketPerfEvent(&objs, containerID, ctx, cli)

	fmt.Println("Program completed.")
}

func unpauseContainer(ctx context.Context, cli *client.Client, containerID string) error {
	fmt.Println("Unpausing the container:", containerID)
	if err := cli.ContainerUnpause(ctx, containerID); err != nil {
		return fmt.Errorf("failed to unpause container: %v", err)
	}
	fmt.Println("Container unpaused successfully.")
	return nil
}

func pauseContainer(ctx context.Context, cli *client.Client, containerID string) error {
	fmt.Println("Pausing the container:", containerID)
	if err := cli.ContainerPause(ctx, containerID); err != nil {
		return fmt.Errorf("failed to pause container: %v", err)
	}
	fmt.Println("Container paused successfully.")
	return nil
}

func listenForPacketPerfEvent(objs *bpfObjects, containerID string, ctx context.Context, cli *client.Client) {
    fmt.Println("Setting up perf event reader...")
	rd, err := perf.NewReader(objs.PerfEventMap, os.Getpagesize())
	if err != nil {
		log.Fatalf("Failed to create perf event reader: %v", err)
	}

	defer func() {
		fmt.Println("Closing perf event reader...")
		rd.Close()
	}()

	fmt.Println("Listening for the first packet event...")
	for {
		record, err := rd.Read()
		if err != nil {
			if err == perf.ErrClosed {
				fmt.Println("Perf event reader closed, exiting goroutine...")
				return // Exit if reader is closed
			}
			log.Printf("Error reading perf event: %v", err)
			continue
		}

		if record.LostSamples != 0 {
			log.Printf("Lost %d samples", record.LostSamples)
		}

		fmt.Println("First packet received, performing actions...")
		if err := unpauseContainer(ctx, cli, containerID); err != nil {
			log.Printf("Error unpausing container: %v", err)
		}
		break // Exit after handling the first event
	}

}
