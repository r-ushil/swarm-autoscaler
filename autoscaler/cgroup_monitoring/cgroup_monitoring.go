package cgroup_monitoring

import (
	"context"
	"fmt"
	"logging"
	"os"
	"path/filepath"
	"scale"
	"server"
	"strconv"
	"strings"
	"time"

)

type CPUResource struct {
	LowerUtil float64
	UpperUtil float64
}

type MemoryResource struct {
	LowerLimit int64
	UpperLimit int64
}

const cgroupDir = "/sys/fs/cgroup/system.slice" // Path to the Docker cgroup directory

func (cpu *CPUResource) Monitor(ctx context.Context, containerID string, collectionPeriod time.Duration, swarmNodeInfo *server.SwarmNodeInfo) {
	lastUsageUsec, err := readCPUUsage(containerID) // Initial read before loop
	if err != nil {
		logging.AddEventLog(fmt.Sprintf("Initial CPU usage read error for container %s: %v", containerID, err))
		return
	}

	logging.AddEventLog(fmt.Sprintf("Started monitoring CPU for container %s", containerID))

	ticker := time.NewTicker(collectionPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentUsageUsec, err := readCPUUsage(containerID)
			if err != nil {
				logging.AddEventLog(fmt.Sprintf("Error reading CPU usage for container %s: %v", containerID, err))
				continue
			}
			usageDeltaUsec := currentUsageUsec - lastUsageUsec
			cpuUtilization := (float64(usageDeltaUsec) / collectionPeriod.Seconds()) / 1e6 * 100

			direction := determineScalingDirection(cpuUtilization, cpu.LowerUtil, cpu.UpperUtil)
			if direction != "" {
				// logging.AddScalingLog(direction)
				logging.AddContainerLog(containerID, cpuUtilization)
				if swarmNodeInfo.AutoscalerManager {
					if err := scale.ScaleService(containerID, direction); err != nil {
						logging.AddEventLog(fmt.Sprintf("Error scaling service for container %s: %v", containerID, err))
					}
				} else {
					managerNode, err := server.GetManagerNode(swarmNodeInfo.OtherNodes)
					if err != nil {
						logging.AddEventLog(fmt.Sprintf("Error getting manager node: %v", err))
						return
					}
					serviceId, err := scale.FindServiceIDFromContainer(containerID)

					if err != nil {
						logging.AddEventLog(fmt.Sprintf("error finding service ID from container: %v", err))
						return
					}

					if err := server.SendScaleRequest(serviceId, direction, managerNode.IP); err != nil {
						logging.AddEventLog(fmt.Sprintf("Error sending scale request to manager node: %v", err))
					}
				}
			}

			lastUsageUsec = currentUsageUsec
		}
	}
}

func (mem *MemoryResource) Monitor(ctx context.Context, containerID string, collectionPeriod time.Duration, swarmNodeInfo *server.SwarmNodeInfo) {
	ticker := time.NewTicker(collectionPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context has been canceled, stop monitoring this container
			logging.AddEventLog(fmt.Sprintf("Stopped monitoring container %s due to context cancellation", containerID))
			return
		case <-ticker.C:
			// Proceed with memory usage check
			memUsage, err := readMemoryUsage(containerID)
			if err != nil {
				logging.AddEventLog(fmt.Sprintf("Error reading memory usage for container %s: %v", containerID, err))
				continue
			}

			direction := determineScalingDirection(float64(memUsage), float64(mem.LowerLimit), float64(mem.UpperLimit))

			// Perform the scaling action if needed
			if direction != "" {
				logging.AddContainerLog(containerID, float64(memUsage))
				// Assuming ScaleService handles its own context and error handling
				if swarmNodeInfo.AutoscalerManager {
					if err := scale.ScaleService(containerID, direction); err != nil {
						logging.AddEventLog(fmt.Sprintf("Error scaling service for container %s: %v", containerID, err))
					}
				} else {
					managerNode, err := server.GetManagerNode(swarmNodeInfo.OtherNodes)
					if err != nil {
						logging.AddEventLog(fmt.Sprintf("Error getting manager node: %v", err))
						return
					}
					serviceId, err := scale.FindServiceIDFromContainer(containerID)

					if err != nil {
						logging.AddEventLog(fmt.Sprintf("error finding service ID from container: %v", err))
						return
					}

					if err := server.SendScaleRequest(serviceId, direction, managerNode.IP); err != nil {
						logging.AddEventLog(fmt.Sprintf("Error sending scale request to manager node: %v\n", err))
					}
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
