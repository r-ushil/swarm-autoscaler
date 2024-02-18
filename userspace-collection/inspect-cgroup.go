package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	cgroupDir        = "/sys/fs/cgroup/system.slice"
	collectionPeriod = 5 * time.Second // Adjust the collection period as needed
)

func main() {
	files, err := os.ReadDir(cgroupDir)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "docker-") && strings.HasSuffix(file.Name(), ".scope") {
			containerID := strings.TrimPrefix(strings.TrimSuffix(file.Name(), ".scope"), "docker-")
			go monitorMemoryUsage(containerID)
		}
	}

	// Prevent the main goroutine from exiting immediately.
	select {}
}

func monitorMemoryUsage(containerID string) {
	for range time.Tick(collectionPeriod) {
		memUsage, err := readMemoryUsage(containerID)
		if err != nil {
			fmt.Printf("Error reading memory usage for container %s: %v\n", containerID, err)
			continue
		}

		fmt.Printf("ContainerID: %s, Memory Usage: %d bytes\n", containerID, memUsage)
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
