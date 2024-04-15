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
	"scale"
	"bpf_port_listen"
)

type Resource interface {
    Monitor(ctx context.Context, containerID string, collectionPeriod time.Duration)
}

type CPUResource struct {
    LowerUtil float64
    UpperUtil float64
}

type MemoryResource struct {
    LowerLimit int64
    UpperLimit int64
}


const cgroupDir = "/sys/fs/cgroup/system.slice" // Path to the Docker cgroup directory

var (
	monitoringCtxMap sync.Map // map[containerID]context.CancelFunc for dynamic monitoring
)

func main() {
	lowerMBPtr := flag.Int64("lower-mm", -1, "Lower memory threshold in MB")
	upperMBPtr := flag.Int64("upper-mm", -1, "Upper memory threshold in MB")
	lowerGBPtr := flag.Int64("lower-mg", -1, "Lower memory threshold in GB")
	upperGBPtr := flag.Int64("upper-mg", -1, "Upper memory threshold in GB")
	lowerCPUUtil := flag.Float64("lower-cpu", -1.0, "Lower CPU utilization threshold in percentage")
	upperCPUUtil := flag.Float64("upper-cpu", -1.0, "Upper CPU utilization threshold in percentage")
	collectionPeriodPtr := flag.Duration("collection-period", 10*time.Second, "Period between memory usage checks for each container")

	flag.Parse()

	var resource Resource
    memoryLimitSpecified := *lowerMBPtr >= 0 || *upperMBPtr >= 0 || *lowerGBPtr >= 0 || *upperGBPtr >= 0
    cpuMonitoringEnabled := *lowerCPUUtil >= 0 || *upperCPUUtil >= 0

    if cpuMonitoringEnabled && memoryLimitSpecified {
        fmt.Println("Please specify thresholds for either CPU or memory, not both.")
        os.Exit(1)
    }

    if cpuMonitoringEnabled {
		resource = &CPUResource{LowerUtil: *lowerCPUUtil, UpperUtil: *upperCPUUtil}
	} else if memoryLimitSpecified {
		var lowerLimit, upperLimit int64
		// Explicitly choose GB over MB if both are provided, instead of summing them
		if *lowerGBPtr > 0 {
			lowerLimit = *lowerGBPtr * 1024
		} else {
			lowerLimit = *lowerMBPtr
		}
		if *upperGBPtr > 0 {
			upperLimit = *upperGBPtr * 1024
		} else {
			upperLimit = *upperMBPtr
		}
		resource = &MemoryResource{LowerLimit: lowerLimit, UpperLimit: upperLimit}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scale := scale.GetScaler()
	portListener, err := bpf_port_listen.GetBPFListener("wlp60s0")
	if err != nil {
		fmt.Printf("Failed to setup BPF listener: %v\n", err)
		os.Exit(1)
	}

	scale.SetPortListener(portListener)
	portListener.SetScaler(scale)
		

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
		startMonitoring(ctx, containerID, resource, *collectionPeriodPtr)
	}

	// Handle container start and stop events
	go func() {
		for {
			select {
			case containerID := <-eventNotifier.StartChan:
				// Start monitoring for the new container
				startMonitoring(ctx, containerID, resource, *collectionPeriodPtr)
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

func startMonitoring(parentCtx context.Context, containerID string, resource Resource, collectionPeriod time.Duration) {
    if _, exists := monitoringCtxMap.Load(containerID); !exists {
        monitorCtx, monitorCancel := context.WithCancel(parentCtx)
        monitoringCtxMap.Store(containerID, monitorCancel)

		// Wait before we immediately start monitoring / scaling
		time.Sleep(5 * time.Second)
        
        // Execute the Monitor method as a goroutine
        go resource.Monitor(monitorCtx, containerID, collectionPeriod)
    }
}

func (cpu *CPUResource) Monitor(ctx context.Context, containerID string, collectionPeriod time.Duration) {
	lastUsageUsec, err := readCPUUsage(containerID) // Initial read before loop
	if err != nil {
		fmt.Printf("Initial CPU usage read error for container %s: %v\n", containerID, err)
		return
	}

	ticker := time.NewTicker(collectionPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Stopped monitoring CPU for container %s\n", containerID)
			return
		case <-ticker.C:
			currentUsageUsec, err := readCPUUsage(containerID)
			if err != nil {
				fmt.Printf("Error reading CPU usage for container %s: %v\n", containerID, err)
				continue
			}
			usageDeltaUsec := currentUsageUsec - lastUsageUsec
			cpuUtilization := (float64(usageDeltaUsec) / collectionPeriod.Seconds()) / 1e6 * 100

			direction := determineScalingDirection(cpuUtilization, cpu.LowerUtil, cpu.UpperUtil)
            if direction != "" {
                fmt.Printf("ContainerID: %s, CPU Utilization: %.2f%%, Direction: %s\n", containerID, cpuUtilization, direction)
                if err := scale.ScaleService(containerID, direction); err != nil {
                    fmt.Printf("Error scaling service for container %s: %v\n", containerID, err)
                }
            }

			lastUsageUsec = currentUsageUsec
		}
	}
}

func (mem *MemoryResource) Monitor(ctx context.Context, containerID string, collectionPeriod time.Duration) {
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

			direction := determineScalingDirection(float64(memUsage), float64(mem.LowerLimit), float64(mem.UpperLimit))

			// Perform the scaling action if needed
			if direction != "" {
				fmt.Printf("ContainerID: %s, Direction: %s, Memory Used: %d MB\n", containerID, direction, memUsage)
				// Assuming ScaleService handles its own context and error handling
				if err := scale.ScaleService(containerID, direction); err != nil {
					fmt.Printf("Error scaling service for container %s: %v\n", containerID, err)
				}
			}
		}
	}
}

// determineScalingDirection decides the scaling direction based on usage and thresholds.
func determineScalingDirection(currentValue float64, lowerThreshold, upperThreshold float64) string {
    if currentValue > upperThreshold {
        return "over"
    } else if currentValue < lowerThreshold {
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

    // Parse the memory usage value from the file content
    memUsageBytes, parseErr := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 64)
    if parseErr != nil {
        return 0, parseErr
    }

    // Convert the memory usage from bytes to MB and return
    memUsageMB := memUsageBytes / (1024 * 1024)
    return memUsageMB, nil
}


func readCPUUsage(containerID string) (int64, error) {
	cpuStatPath := filepath.Join(cgroupDir, fmt.Sprintf("docker-%s.scope/cpu.stat", containerID))
	content, err := os.ReadFile(cpuStatPath)
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "usage_usec") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				usageUsec, err := strconv.ParseInt(parts[1], 10, 64)
				if err != nil {
					return 0, err
				}
				return usageUsec, nil
			}
		}
	}

	return 0, fmt.Errorf("usage_usec not found in cpu.stat for container %s", containerID)
}
