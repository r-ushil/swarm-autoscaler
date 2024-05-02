package main

import (
	"bpf_port_listen"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"scale"
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
	Iface            string            `yaml:"iface"`
	Managers         map[string]string `yaml:"managers"`
	Workers          map[string]string `yaml:"workers"`
}

// SwarmNodeInfo represents the node-specific information
type SwarmNodeInfo struct {
	AutoscalerManager bool
	OtherNodes        []SwarmNode
}

type SwarmNode struct {
	Hostname string
	IP       string
}

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
	configPath := flag.String("config", "", "Path to the configuration file")

	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	collectionPeriod, err := time.ParseDuration(config.CollectionPeriod)
	if err != nil {
		fmt.Printf("Failed to parse collection period: %v\n", err)
		os.Exit(1)
	}

	swarmNodeInfo, err := createswarmNodeInfo(config)
	if err != nil {
		fmt.Printf("Failed to get Swarm node info: %v\n", err)
		os.Exit(1)
	}

	var resource Resource
	memoryLimitSpecified := config.LowerMB >= 0 || config.UpperMB >= 0 || config.LowerGB >= 0 || config.UpperGB >= 0
	cpuMonitoringEnabled := config.LowerCPU >= 0 || config.UpperCPU >= 0

	if cpuMonitoringEnabled && memoryLimitSpecified {
		fmt.Println("Please specify thresholds for either CPU or memory, not both.")
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

	scale := scale.GetScaler()
	portListener, err := bpf_port_listen.GetBPFListener(config.Iface)
	if err != nil {
		fmt.Printf("Failed to setup BPF listener: %v\n", err)
		os.Exit(1)
	}

	scale.SetPortListener(portListener)
	portListener.SetScaler(scale)

	eventNotifier := scale.NewEventNotifier()
	go eventNotifier.ListenForEvents(ctx)

	runningContainers, err := scale.GetRunningContainers(ctx)
	if err != nil {
		fmt.Printf("Failed to get running containers: %v\n", err)
		os.Exit(1)
	}

	
	for _, containerID := range runningContainers {
		fmt.Printf("Monitoring container: %s\n", containerID)
		startMonitoring(ctx, containerID, resource, collectionPeriod)
	}

	// Handle container start and stop events
	go func() {
		for {
			select {
			case containerID := <-eventNotifier.StartChan:
				// Start monitoring for the new container
				fmt.Printf("Monitoring container: %s\n", containerID)
				startMonitoring(ctx, containerID, resource, collectionPeriod)
			case containerID := <-eventNotifier.StopChan:
				// Stop monitoring for the stopped container
				fmt.Printf("Stopping monitoring for container: %s\n", containerID)
				if cancelFunc, exists := monitoringCtxMap.Load(containerID); exists {
					cancelFunc.(context.CancelFunc)()
					monitoringCtxMap.Delete(containerID)
				}
			}
		}
	}()

	go func() {
		http.HandleFunc("/", scaleHandler)
		fmt.Println("Starting HTTP server on port 4567")
		if err := http.ListenAndServe(":4567", nil); err != nil {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	fmt.Println("Swarm node info:", swarmNodeInfo)

	// Wait for a signal to terminate
	<-ctx.Done()

}

func startMonitoring(parentCtx context.Context, containerID string, resource Resource, collectionPeriod time.Duration) {
	if _, exists := monitoringCtxMap.Load(containerID); !exists {
		monitorCtx, monitorCancel := context.WithCancel(parentCtx)
		monitoringCtxMap.Store(containerID, monitorCancel)

		go resource.Monitor(monitorCtx, containerID, collectionPeriod)
	}
}

func (cpu *CPUResource) Monitor(ctx context.Context, containerID string, collectionPeriod time.Duration) {
	lastUsageUsec, err := readCPUUsage(containerID) // Initial read before loop
	if err != nil {
		fmt.Printf("Initial CPU usage read error for container %s: %v\n", containerID, err)
		return
	}

	fmt.Printf("Started monitoring CPU for container %s\n", containerID)

	ticker := time.NewTicker(collectionPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
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

func scaleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		ServiceID string `json:"serviceId"`
		Direction string `json:"direction"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := scale.ChangeServiceReplicas(data.ServiceID, data.Direction); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Scaling successful."))
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
		CollectionPeriod: "10s",
		Iface:            "eth0",
		Managers:         make(map[string]string),
		Workers:          make(map[string]string),
	}

	if err := yaml.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func createswarmNodeInfo(config *Config) (*SwarmNodeInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	swarmNodeInfo := SwarmNodeInfo{
		AutoscalerManager: false,
		OtherNodes:        make([]SwarmNode, 0),
	}

	// Check if this node is a manager
	if _, ok := config.Managers[hostname]; ok {
		swarmNodeInfo.AutoscalerManager = true
	}

	// Add all nodes to OtherNodes list
	for name, ip := range config.Managers {
		if name != hostname {
			swarmNodeInfo.OtherNodes = append(swarmNodeInfo.OtherNodes, SwarmNode{name, ip})
		}
	}
	for name, ip := range config.Workers {
		swarmNodeInfo.OtherNodes = append(swarmNodeInfo.OtherNodes, SwarmNode{name, ip})
	}

	return &swarmNodeInfo, nil
}
