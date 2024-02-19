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
    LowerValue int64
    UpperValue int64
    Unit       string // "MB" or "GB"
}

func main() {

	// Parse CLI arguments

	lowerMBPtr := flag.Int("lower-mm", 0, "Lower memory threshold in MB")
	upperMBPtr := flag.Int("upper-mm", 0, "Upper memory threshold in MB")
	lowerGBPtr := flag.Int("lower-mg", 0, "Lower memory threshold in GB")
	upperGBPtr := flag.Int("upper-mg", 0, "Upper memory threshold in GB")

	flag.Parse()

	var threshold MemoryThreshold
	if *lowerMBPtr > 0 || *upperMBPtr > 0 {
		threshold = MemoryThreshold{LowerValue: int64(*lowerMBPtr), UpperValue: int64(*upperMBPtr), Unit: "MB"}
	} else if *lowerGBPtr > 0 || *upperGBPtr > 0 {
		threshold = MemoryThreshold{LowerValue: int64(*lowerGBPtr), UpperValue: int64(*upperGBPtr), Unit: "GB"}
	} else {
		fmt.Println("Please provide memory thresholds using the -lower-mm/-upper-mm or -lower-mg/-upper-mg flags")
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

        memUsageConverted, _ := convertMemoryUsage(memUsage, threshold.Unit)

        // Determine the scaling direction based on memory thresholds
        var direction string
        if int64(memUsageConverted) > threshold.UpperValue {
            direction = "over"
        } else if int64(memUsageConverted) < threshold.LowerValue {
            direction = "under"
        } else {
            continue // No scaling action needed
        }

        // Call ScaleService in a goroutine with the determined direction
        go func(containerID, direction string) {
            err := scale.ScaleService(containerID, direction)
            if err != nil {
                fmt.Printf("Error scaling service for container %s: %v\n", containerID, err)
            }
        }(containerID, direction)

        // This print statement might need adjustment to reflect the new logic
        fmt.Printf("ContainerID: %s, Direction: %s, Memory Used: %.2f %s\n", containerID, direction, memUsageConverted, threshold.Unit)
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
