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