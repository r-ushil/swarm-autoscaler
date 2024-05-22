package conc_req_monitoring
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang BPF bpf/conc_req_monitoring.c -- -I/usr/include/ 


import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/rlimit"
	"log"
	"os"
	"time"
)

type ConcReqResource struct {
	LowerLimit  int64
	UpperLimit  int64
	BufferLimit int64
}

type Event struct {
	Port         uint16
	BufferLength uint32
	Message      [6]byte
}

func main() {
	// Allow the current process to lock memory for eBPF resources.
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Fatalf("failed to remove memlock limit: %v", err)
	}

	// Load pre-compiled programs and maps into the kernel
	objs := BPFObjects{}
	if err := LoadBPFObjects(&objs, nil); err != nil {
		log.Fatalf("loading objects: %s", err)
	}
	defer objs.Close()

	// Set up constants
	constants := map[uint32]uint32{
		0: 3, // lowerLimit
		1: 10, // upperLimit
		2: 5, // bufferLength
	}
	for k, v := range constants {
		if err := objs.ConstantsMap.Put(k, v); err != nil {
			log.Fatalf("failed to set constants: %v", err)
		}
	}

	// Attach the eBPF program to the inet_sock_set_state tracepoint
	tracepoint, err := link.Tracepoint("sock", "inet_sock_set_state", objs.Prog, nil)
	if err != nil {
		log.Fatalf("failed to attach BPF program: %v", err)
	}
	defer tracepoint.Close()

	// Open a perf event reader
	rd, err := perf.NewReader(objs.Events, os.Getpagesize())
	if err != nil {
		log.Fatalf("failed to create perf event reader: %v", err)
	}
	defer rd.Close()

	// Process events from the perf buffer
	go func() {
		var event Event
		for {
			record, err := rd.Read()
			if err != nil {
				if err == perf.ErrClosed {
					return // Exiting
				}
				log.Printf("failed to read from perf buffer: %v", err)
				continue
			}

			if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
				log.Printf("failed to decode received data: %v", err)
				continue
			}

			fmt.Printf("Port: %d, BufferLength: %d, Exceeded: %s\n", event.Port, event.BufferLength, string(event.Message[:]))
		}
	}()

	// Block until a signal is received
	select {
	case <-context.Background().Done():
		fmt.Println("Shutting down...")
	}
}
