package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"userspace-collection/scale" // Assuming scale.go is in this package
)

type MemoryThreshold struct {
	LowerValue int64
	UpperValue int64
	Unit       string // "MB" or "GB"
}

const cgroupDir = "/sys/fs/cgroup/system.slice" // Path to the Docker cgroup directory

var (
	monitoringCtxMap sync.Map // map[containerID]context.CancelFunc for dynamic monitoring
)

func main() {
	lowerMBPtr := flag.Int("lower-mm", 0, "Lower memory threshold in MB")
	upperMBPtr := flag.Int("upper-mm", 0, "Upper memory threshold in MB")
	lowerGBPtr := flag.Int("lower-mg", 0, "Lower memory threshold in GB")
	upperGBPtr := flag.Int("upper-mg", 0, "Upper memory threshold in GB")
	collectionPeriodPtr := flag.Duration("collection-period", 10*time.Second, "Period between memory usage checks for each container")

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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize and start listening for Docker container events
	eventNotifier := scale.NewEventNotifier()
	go eventNotifier.ListenForEvents(ctx)

	// Initialize monitoring for already running containers
	runningContainers, err := scale.GetRunningContainers(ctx)
	if err != nil {
		fmt.Printf("Failed to get running containers: %v\n", err)
		os.Exit(1)
	}

	for _, containerID := range runningContainers {
		fmt.Printf("Monitoring container: %s\n", containerID)
		startMonitoring(ctx, containerID, threshold, *collectionPeriodPtr)
	}

	// Handle container start and stop events
	go func() {
		for {
			select {
			case containerID := <-eventNotifier.StartChan:
				// Start monitoring for the new container
				startMonitoring(ctx, containerID, threshold, *collectionPeriodPtr)
			case containerID := <-eventNotifier.StopChan:
				// Stop monitoring for the stopped container
				if cancelFunc, exists := monitoringCtxMap.Load(containerID); exists {
					cancelFunc.(context.CancelFunc)()
					monitoringCtxMap.Delete(containerID)
				}
			}
		}
	}()

	// Wait for a signal to terminate
	<-ctx.Done()
}

func startMonitoring(parentCtx context.Context, containerID string, threshold MemoryThreshold, collectionPeriod time.Duration) {
	if _, exists := monitoringCtxMap.Load(containerID); !exists {
		monitorCtx, monitorCancel := context.WithCancel(parentCtx)
		monitoringCtxMap.Store(containerID, monitorCancel)
		go monitorMemoryUsage(monitorCtx, containerID, threshold, collectionPeriod)
	}
}

// Implement monitorMemoryUsage here as previously described,
// making sure it uses the provided context for cancellation.

// Implement readMemoryUsage, convertMemoryUsage as before.

func monitorMemoryUsage(ctx context.Context, containerID string, threshold MemoryThreshold, collectionPeriod time.Duration) {
	ticker := time.NewTicker(collectionPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context has been canceled, stop monitoring this container
			fmt.Printf("Stopped monitoring container %s due to context cancellation\n", containerID)
			return
		case <-ticker.C:
			// Proceed with memory usage check
			memUsage, err := readMemoryUsage(containerID)
			if err != nil {
				fmt.Printf("Error reading memory usage for container %s: %v\n", containerID, err)
				continue
			}

			memUsageConverted, memUnit := convertMemoryUsage(memUsage, threshold.Unit)
			direction := determineScalingDirection(memUsageConverted, threshold)

			// Perform the scaling action if needed
			if direction != "" {
				fmt.Printf("ContainerID: %s, Direction: %s, Memory Used: %.2f %s\n", containerID, direction, memUsageConverted, memUnit)
				// Assuming ScaleService handles its own context and error handling
				if err := scale.ScaleService(containerID, direction); err != nil {
					fmt.Printf("Error scaling service for container %s: %v\n", containerID, err)
				}
			}
		}
	}
}

// determineScalingDirection decides the scaling direction based on memory usage and thresholds.
func determineScalingDirection(memUsageConverted float64, threshold MemoryThreshold) string {
	if memUsageConverted > float64(threshold.UpperValue) {
		return "over"
	} else if memUsageConverted < float64(threshold.LowerValue) {
		return "under"
	}
	return "" // No action needed if within thresholds
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
