package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"userspace-collection/scale"
)

const (
	cgroupDir        = "/sys/fs/cgroup/system.slice"
	collectionPeriod = 10 * time.Second // Adjust the collection period as needed
)

type MemoryThreshold struct {
	Value int64
	Unit  string // "MB" or "GB"
}

func main() {

	// Parse CLI arguments

	mbPtr := flag.Int("mm", 0, "Memory threshold in MB")
	gbPtr := flag.Int("mg", 0, "Memory threshold in GB")
	flag.Parse()

	var threshold MemoryThreshold
	if *mbPtr > 0 {
		threshold = MemoryThreshold{Value: int64(*mbPtr), Unit: "MB"}
	} else if *gbPtr > 0 {
		threshold = MemoryThreshold{Value: int64(*gbPtr), Unit: "GB"}
	} else {
		fmt.Println("Please provide a memory threshold using the -mm or -mg flag")
		os.Exit(1)
	}

	files, err := os.ReadDir(cgroupDir)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "docker-") && strings.HasSuffix(file.Name(), ".scope") {
			containerID := strings.TrimPrefix(strings.TrimSuffix(file.Name(), ".scope"), "docker-")
			go monitorMemoryUsage(containerID, threshold)
		}
	}

	// Prevent the main goroutine from exiting immediately.
	select {}
}

func monitorMemoryUsage(containerID string, threshold MemoryThreshold) {
	for range time.Tick(collectionPeriod) {
		memUsage, err := readMemoryUsage(containerID)
		if err != nil {
			fmt.Printf("Error reading memory usage for container %s: %v\n", containerID, err)
			continue
		}

		memUsageConverted, memUnit := convertMemoryUsage(memUsage, threshold.Unit)

		// Determine the scaling direction based on the memory threshold
		var direction string
		if int64(memUsageConverted) > threshold.Value {
			direction = "over"
		} else {
			direction = "under"
		}

		// Call ScaleService in a goroutine
		go func(containerID, direction string) {
			err := scale.ScaleService(containerID, direction)
			if err != nil {
				fmt.Printf("Error scaling service for container %s: %v\n", containerID, err)
			} else {
				fmt.Printf("Scaled service for container %s, direction: %s\n", containerID, direction)
			}
		}(containerID, direction)

		fmt.Printf("ContainerID: %s, Status: %s memory threshold, Memory Used: %.2f %s\n", containerID, direction, memUsageConverted, memUnit)
	}
}

func readMemoryUsage(containerID string) (int64, error) {
	memCurrentPath := filepath.Join(cgroupDir, fmt.Sprintf("docker-%s.scope/memory.current", containerID))
	content, err := os.ReadFile(memCurrentPath)
	if err != nil {
		return 0, err
	}

	return strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
}

func convertMemoryUsage(memUsage int64, unit string) (float64, string) {
	switch unit {
	case "MB":
		return float64(memUsage) / 1024 / 1024, "MB"
	case "GB":
		return float64(memUsage) / 1024 / 1024 / 1024, "GB"
	default:
		return float64(memUsage), "B"
	}
}
