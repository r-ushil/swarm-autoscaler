package logging

import (
	"fmt"
	"os"
	"strconv"
	"github.com/olekukonko/tablewriter"
)



type EventLog struct {
	Event string
}

var containerLogs = make(map[string]float64)
var serviceLogs = make(map[string]uint32)
var bpfListenerLogs = make(map[uint32]string)
var eventLogs = []EventLog{}


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