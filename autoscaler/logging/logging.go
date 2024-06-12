package logging

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/olekukonko/tablewriter"
)



type EventLog struct {
	Event string
}

type ScalingLog struct {
	Time string
	Direction string
}

var containerLogs = make(map[string]float64)
var serviceLogs = make(map[string]uint32)
var bpfListenerLogs = make(map[uint32]string)
var eventLogs = []EventLog{}


func AddScalingLog(direction string) {
	logEntry := ScalingLog{time.Now().String(), direction}

	logFilePath := "logging/scaling.log"

	// Open the file in append mode, create it if it doesn't exist
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		return
	}
	defer file.Close()

	if _, err := file.WriteString(fmt.Sprintf("%s %s\n", logEntry.Time, logEntry.Direction)); err != nil {
		fmt.Printf("Error writing to log file: %v\n", err)
	}
}

func AddContainerLog(containerId string, util float64) {
	containerLogs[containerId] = util
}

func RemoveContainerLog(containerId string) {
	if _, ok := containerLogs[containerId]; !ok {
		return
	}

	delete(containerLogs, containerId)
}

func AddServiceLog(serviceId string, replicas uint32) {
	serviceLogs[serviceId] = replicas
}

func RemoveServiceLog(serviceId string) {
	if _, ok := serviceLogs[serviceId]; !ok {
		return
	}

	delete(serviceLogs, serviceId)
}

func AddBPFListenerLog(serviceId string, port uint32) {
	bpfListenerLogs[port] = serviceId
}

func RemoveBPFListenerLog(port uint32) {
	if _, ok := bpfListenerLogs[port]; !ok {
		return
	}

	delete(bpfListenerLogs, port)	
}

func AddEventLog(event string) {
	fmt.Println(event)
	eventLogs = append(eventLogs, EventLog{Event: event})
}

func WriteScaleLogs() {

	logFile, err := os.OpenFile("logging/autoscaler.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}

	defer logFile.Close()



}

func WriteLogs(events bool) {

	logFile, err := os.OpenFile("logging/autoscaler.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}

	defer logFile.Close()

	table := tablewriter.NewWriter(logFile)
	table.SetHeader([]string{"Container ID", "Utilization"})
	for containerId, util := range containerLogs {
		table.Append([]string{containerId, strconv.FormatFloat(util, 'f', 2, 64)})
	}
	table.Render() 

	table = tablewriter.NewWriter(logFile)
	table.SetHeader([]string{"Service ID", "Replicas"})
	for serviceId, replicas := range serviceLogs {
		table.Append([]string{serviceId, strconv.Itoa(int(replicas))})
	}
	table.Render()

	table = tablewriter.NewWriter(logFile)
	table.SetHeader([]string{"Service ID", "Port"})
	for port, serviceId := range bpfListenerLogs {
		table.Append([]string{serviceId, strconv.Itoa(int(port))})
	}
	table.Render()

	if events {
		table = tablewriter.NewWriter(logFile)
		table.SetHeader([]string{"Event"})
		for _, log := range eventLogs {
			table.Append([]string{log.Event})
		}
		table.Render()
	}
}