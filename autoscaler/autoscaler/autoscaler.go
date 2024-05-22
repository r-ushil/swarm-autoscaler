package main

import (
	"bpf_port_listen"
	"cgroup_monitoring"
	"context"
	"flag"
	"fmt"
	"logging"
	"os"
	"scale"
	"server"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

type Resource interface {
	Monitor(ctx context.Context, containerID string, collectionPeriod time.Duration, swarmNodeInfo *server.SwarmNodeInfo)
}

var (
	monitoringCtxMap sync.Map // map[containerID]context.CancelFunc for dynamic monitoring
)

type Config struct {
	LowerCPU               float64           `yaml:"lower-cpu"`
	UpperCPU               float64           `yaml:"upper-cpu"`
	LowerMB                int64             `yaml:"lower-mm"`
	UpperMB                int64             `yaml:"upper-mm"`
	LowerGB                int64             `yaml:"lower-mg"`
	UpperGB                int64             `yaml:"upper-mg"`
	LowerConcReq           int64             `yaml:"lower-conc-req"`
	UpperConcReq           int64             `yaml:"upper-conc-req"`
	ReqThresholdTolerance  int64             `yaml:"req-threshold-tolerance"` 
	CollectionPeriod       string            `yaml:"collection-period"`
	KeepAlive              string            `yaml:"keep-alive"`
	Iface                  string            `yaml:"iface"`
	Managers               map[string]string `yaml:"managers"`
	Workers                map[string]string `yaml:"workers"`
	Logging 		       map[string]bool   `yaml:"logging"`
}

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
	memoryMonitoringEnabled := config.LowerMB >= 0 || config.UpperMB >= 0 || config.LowerGB >= 0 || config.UpperGB >= 0
	cpuMonitoringEnabled := config.LowerCPU >= 0 || config.UpperCPU >= 0
	concReqMonitoringEnabled := config.LowerConcReq >= 0 || config.UpperConcReq >= 0

	if (cpuMonitoringEnabled && memoryMonitoringEnabled) || (memoryMonitoringEnabled && concReqMonitoringEnabled) || (cpuMonitoringEnabled && concReqMonitoringEnabled) {
		logging.AddEventLog("More than one of CPU, memory or concurrent request monitoring are enabled. Please specific only one.")
		os.Exit(1)
	}


	if cpuMonitoringEnabled {
		resource = &cgroup_monitoring.CPUResource{LowerUtil: config.LowerCPU, UpperUtil: config.UpperCPU}
	} else if memoryMonitoringEnabled {
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
		resource = &cgroup_monitoring.MemoryResource{LowerLimit: lowerLimit, UpperLimit: upperLimit}
	} else if concReqMonitoringEnabled {
		// make ConcReqResource here
		
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

func loadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// set default values to avoid nil pointers
	config := Config{
		LowerCPU:              -1,
		UpperCPU:              -1,
		LowerMB:               -1,
		UpperMB:               -1,
		LowerGB:               -1,
		UpperGB:               -1,
		LowerConcReq:          -1,
		UpperConcReq:          -1,
		ReqThresholdTolerance: 5,
		KeepAlive:             "5s",
		CollectionPeriod:      "10s",
		Iface:                 "eth0",
		Managers:              make(map[string]string),
		Workers:               make(map[string]string),
		Logging:               make(map[string]bool),
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
