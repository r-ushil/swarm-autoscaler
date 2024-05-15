package main

import (
	"bpf_port_listen"
	"context"
	"flag"
	"fmt"
	"logging"
	"os"
	"path/filepath"
	"scale"
	"server"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	LowerCPU         float64           `yaml:"lower-cpu"`
	UpperCPU         float64           `yaml:"upper-cpu"`
	LowerMB          int64             `yaml:"lower-mm"`
	UpperMB          int64             `yaml:"upper-mm"`
	LowerGB          int64             `yaml:"lower-mg"`
	UpperGB          int64             `yaml:"upper-mg"`
	CollectionPeriod string            `yaml:"collection-period"`
	KeepAlive        string            `yaml:"keep-alive"`
	Iface            string            `yaml:"iface"`
	Managers         map[string]string `yaml:"managers"`
	Workers          map[string]string `yaml:"workers"`
	Logging 		 map[string]bool   `yaml:"logging"`
}

type Resource interface {
	Monitor(ctx context.Context, containerID string, collectionPeriod time.Duration, swarmNodeInfo *server.SwarmNodeInfo)
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
	configPath := flag.String("config", "", "Path to the configuration file")

	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		logging.AddEventLog(fmt.Sprintf("Failed to load configuration: %v", err))
		os.Exit(1)
	}

	collectionPeriod, err := time.ParseDuration(config.CollectionPeriod)
	if err != nil {
		logging.AddEventLog(fmt.Sprintf("Failed to parse collection period: %v", err))
		os.Exit(1)
	}

	swarmNodeInfo, err := createswarmNodeInfo(config)
	if err != nil {
		logging.AddEventLog(fmt.Sprintf("Failed to create swarm node info: %v", err))
		os.Exit(1)
	}

	var resource Resource
	memoryLimitSpecified := config.LowerMB >= 0 || config.UpperMB >= 0 || config.LowerGB >= 0 || config.UpperGB >= 0
	cpuMonitoringEnabled := config.LowerCPU >= 0 || config.UpperCPU >= 0

	if cpuMonitoringEnabled && memoryLimitSpecified {
		logging.AddEventLog("Both CPU and memory monitoring are enabled. Please specify only one.")
		os.Exit(1)
	}

	if cpuMonitoringEnabled {
		resource = &CPUResource{LowerUtil: config.LowerCPU, UpperUtil: config.UpperCPU}
	} else if memoryLimitSpecified {
		var lowerLimit, upperLimit int64
		// Explicitly choose GB over MB if both are provided, instead of summing them
		if config.LowerGB > 0 {
			lowerLimit = config.LowerGB * 1024
		} else {
			lowerLimit = config.LowerMB
		}
		if config.UpperGB > 0 {
			upperLimit = config.UpperGB * 1024
		} else {
			upperLimit = config.UpperMB
		}
		resource = &MemoryResource{LowerLimit: lowerLimit, UpperLimit: upperLimit}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scaler := scale.GetScaler()
	portListener, err := bpf_port_listen.GetBPFListener(config.Iface)
	if err != nil {
		logging.AddEventLog(fmt.Sprintf("Failed to setup BPF listener: %v", err))
		os.Exit(1)
	}

	scaler.SetPortListener(portListener)
	portListener.SetScaler(scaler)

	portListener.SetNodeInfo(*swarmNodeInfo)
	scaler.SetNodeInfo(*swarmNodeInfo)

	eventNotifier := scaler.NewEventNotifier()
	go eventNotifier.ListenForEvents(ctx)

	runningContainers, err := scaler.GetRunningContainers(ctx)
	if err != nil {
		logging.AddEventLog(fmt.Sprintf("Failed to get running containers: %v", err))
		os.Exit(1)
	}

	for _, containerID := range runningContainers {
		logging.AddEventLog(fmt.Sprintf("Monitoring container: %s", containerID))
		startMonitoring(ctx, containerID, resource, collectionPeriod, swarmNodeInfo)
	}

	// Handle container start and stop events
	go func() {
		for {
			select {
			case containerID := <-eventNotifier.StartChan:
				// Start monitoring for the new container
				logging.AddEventLog(fmt.Sprintf("Monitoring container: %s", containerID))
				startMonitoring(ctx, containerID, resource, collectionPeriod, swarmNodeInfo)
			case containerID := <-eventNotifier.StopChan:
				// Stop monitoring for the stopped container
				logging.AddEventLog(fmt.Sprintf("Stopping monitoring for container: %s", containerID))
				if cancelFunc, exists := monitoringCtxMap.Load(containerID); exists {
					cancelFunc.(context.CancelFunc)()
					monitoringCtxMap.Delete(containerID)
				}
			}
		}
	}()

	// Start HTTP server for scaling requests if manager node
	if swarmNodeInfo.AutoscalerManager {
		go func() {
			server.ScaleServer(scale.ChangeServiceReplicas)
		}()
	}

	// Start server for port listener requests
	go func() {
		server.PortServer(portListener.ListenOnPort, portListener.RemovePort)
	}()

	if config.Logging["enable"] {
		go func() {
			for {
				os.MkdirAll("logging", 0755)
				logging.WriteLogs(config.Logging["events"])
				time.Sleep(1 * time.Second)
			}
		}()
	}

	// Wait for a signal to terminate
	<-ctx.Done()

}

func startMonitoring(parentCtx context.Context, containerID string, resource Resource, collectionPeriod time.Duration, swarmNodeInfo *server.SwarmNodeInfo) {
	if _, exists := monitoringCtxMap.Load(containerID); !exists {
		monitorCtx, monitorCancel := context.WithCancel(parentCtx)
		monitoringCtxMap.Store(containerID, monitorCancel)

		go resource.Monitor(monitorCtx, containerID, collectionPeriod, swarmNodeInfo)
	}
}

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

func loadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// set default values to avoid nil pointers
	config := Config{
		LowerCPU:         -1,
		UpperCPU:         -1,
		LowerMB:          -1,
		UpperMB:          -1,
		LowerGB:          -1,
		UpperGB:          -1,
		KeepAlive:        "5s",
		CollectionPeriod: "10s",
		Iface:            "eth0",
		Managers:         make(map[string]string),
		Workers:          make(map[string]string),
		Logging:          make(map[string]bool),
	}

	if err := yaml.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func createswarmNodeInfo(config *Config) (*server.SwarmNodeInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	swarmNodeInfo := server.SwarmNodeInfo{
		AutoscalerManager: false,
		OtherNodes:        make([]server.SwarmNode, 0),
	}

	keepAlive, err := time.ParseDuration(config.KeepAlive)
	if err != nil {
		logging.AddEventLog(fmt.Sprintf("Failed to parse keep alive: %v", err))
		os.Exit(1)
	} else {
		swarmNodeInfo.KeepAlive = keepAlive
	}

	// Check if this node is a manager
	if _, ok := config.Managers[hostname]; ok {
		swarmNodeInfo.AutoscalerManager = true
	}

	// Add all nodes to OtherNodes list
	for name, ip := range config.Managers {
		if name != hostname {
			swarmNodeInfo.OtherNodes = append(swarmNodeInfo.OtherNodes, server.SwarmNode{Hostname: name, IP: ip, Manager: true})
		}
	}
	for name, ip := range config.Workers {
		swarmNodeInfo.OtherNodes = append(swarmNodeInfo.OtherNodes, server.SwarmNode{Hostname: name, IP: ip, Manager: false})
	}

	return &swarmNodeInfo, nil
}
