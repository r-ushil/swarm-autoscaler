package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// SwarmNodeInfo represents the node-specific information
type SwarmNodeInfo struct {
	AutoscalerManager bool
	OtherNodes        []SwarmNode
}

type SwarmNode struct {
	Hostname string
	IP       string
	Manager  bool
}

func GetManagerNode(otherNodes []SwarmNode) (SwarmNode, error) {
	for _, node := range otherNodes {
		if node.Manager {
			return node, nil
		}
	}

	return SwarmNode{}, fmt.Errorf("no manager node found")
}

func ScaleServer(scaleFunc func(serviceID string, direction string) error) {
	scaleHandler := createScalerHandler(scaleFunc)
	http.HandleFunc("/", scaleHandler)
	fmt.Println("Starting HTTP server on port 4567")
	if err := http.ListenAndServe(":4567", nil); err != nil {
		fmt.Printf("HTTP server error: %v\n", err)
	}
}

func createScalerHandler(scaleFunc func(serviceID string, direction string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		if err := scaleFunc(data.ServiceID, data.Direction); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Scaling successful."))
	}
}

// send scale request to manager node from worker node
func SendScaleRequest(serviceId string, direction string, managerIP string) error {

	data := map[string]string{"serviceId": serviceId, "direction": direction}
	jsonData, err := json.Marshal(data)

	if err != nil {
		return fmt.Errorf("error marshalling JSON data: %w", err)
	}

	resp, err := http.Post("http://"+managerIP+":4567/", "application/json", bytes.NewBuffer(jsonData))

	if err != nil {
		return fmt.Errorf("error sending scale request to manager node: %w", err)
	}

	defer resp.Body.Close()

	return nil

}

// BPF Port Listener Server

func PortServer(listenOnPortFunc func(port uint32, serviceID string) error, removePortFunc func(port uint32) error) {
	listenHandler := ListenPortHandler(listenOnPortFunc)
	removeHandler := RemovePortHandler(removePortFunc)

	http.HandleFunc("/listen", listenHandler)
	http.HandleFunc("/remove", removeHandler)
	fmt.Println("Starting HTTP server on port 4568")
	if err := http.ListenAndServe(":4568", nil); err != nil {
		fmt.Printf("HTTP server error: %v\n", err)
	}
}

func ListenPortHandler(listenOnPortFunc func(port uint32, serviceID string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
			return
		}

		var data struct {
			Port      uint32 `json:"port"`
			ServiceID string `json:"serviceId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// call the listenOnPortFunc
		if err := listenOnPortFunc(data.Port, data.ServiceID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Listening on port."))
	}
}

func RemovePortHandler(removePortFunc func(port uint32) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
			return
		}

		var data struct {
			Port uint32 `json:"port"`
		}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// call the removePortFunc
		if err := removePortFunc(data.Port); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Removed port."))
	}
}

func SendListenRequest(port uint32, serviceID string, ip string) error {

	data := map[string]interface{}{"port": port, "serviceId": serviceID}
	jsonData, err := json.Marshal(data)

	if err != nil {
		return fmt.Errorf("error marshalling JSON data: %w", err)
	}

	resp, err := http.Post("http://"+ip+":4568/listen", "application/json", bytes.NewBuffer(jsonData))

	if err != nil {
		return fmt.Errorf("error sending listen request to manager node: %w", err)
	}

	defer resp.Body.Close()

	return nil

}

func SendRemoveRequest(port uint32, ip string) error {

	data := map[string]uint32{"port": port}
	jsonData, err := json.Marshal(data)

	if err != nil {
		return fmt.Errorf("error marshalling JSON data: %w", err)
	}

	resp, err := http.Post("http://"+ip+":4568/remove", "application/json", bytes.NewBuffer(jsonData))

	if err != nil {
		return fmt.Errorf("error sending remove request to manager node: %w", err)
	}

	defer resp.Body.Close()

	return nil
}

func SendListenRequestToAllNodes(swarmNodeInfo SwarmNodeInfo, port uint32, serviceID string) error {
	for _, node := range swarmNodeInfo.OtherNodes {

		err := SendListenRequest(port, serviceID, node.IP)
		if err != nil {
			fmt.Println("Error sending listen request to node:", err)
			return fmt.Errorf("error sending listen request to node %s: %w", node.IP, err)
		}
	}

	return nil
}

func SendRemoveRequestToAllNodes(swarmNodeInfo SwarmNodeInfo, port uint32) error {
	for _, node := range swarmNodeInfo.OtherNodes {

		err := SendRemoveRequest(port, node.IP)
		if err != nil {
			return fmt.Errorf("error sending remove request to node %s: %w", node.IP, err)
		}
	}

	return nil
}
