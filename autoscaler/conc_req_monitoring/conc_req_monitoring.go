package conc_req_monitoring

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang BPF bpf/conc_req_monitoring.c -- -I/usr/include/

import (
	"context"
    "bytes"
    "encoding/binary"
    "fmt"
    "log"
	"logging"
    "os"
	"time"
	"scale"
	"server"
	"strings"
    "sync"
    "github.com/cilium/ebpf"
    "github.com/cilium/ebpf/link"
    "github.com/cilium/ebpf/perf"
    "github.com/cilium/ebpf/rlimit"
)

// Define the Data structure
type Data struct {
    Port    uint16
    Message [6]byte
}

type BPFListener struct {
    PerfReader           *perf.Reader
    ConstantsMap         *ebpf.Map
	BufferMap            *ebpf.Map
    ActiveConnectionsMap *ebpf.Map
	ScalingMap           *ebpf.Map
    Events               *ebpf.Map
    Tracepoint           link.Link
    closing              chan struct{}
    closed               bool
    mu                   sync.Mutex
}

type ConcReqResource struct {
	LowerLimit   int64
	UpperLimit   int64
	BufferLength int64
}

type PortContext struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	Signal chan string
}

type PortContextMap struct {
    internalMap sync.Map
}

func (pcm *PortContextMap) Store(port uint16, portCtx PortContext) {
    pcm.internalMap.Store(port, portCtx)
}

func (pcm *PortContextMap) Load(port uint16) (PortContext, bool) {
    value, ok := pcm.internalMap.Load(port)
    if !ok {
        return PortContext{}, false
    }
    return value.(PortContext), true
}

func (pcm *PortContextMap) Delete(port uint16) {
    pcm.internalMap.Delete(port)
}


var (
	portToContext      PortContextMap
	listenerInstance   *BPFListener
	once               sync.Once
)



func InitBPFListener(resource ConcReqResource) error {
    // Allow the current process to lock memory for eBPF resources.
    if err := rlimit.RemoveMemlock(); err != nil {
        return fmt.Errorf("failed to remove memlock limit: %v", err)
    }

    objs := BPFObjects{}
    if err := LoadBPFObjects(&objs, nil); err != nil {
        return fmt.Errorf("loading objects: %v", err)
    }

    // Attach the eBPF program to the tracepoint
    opts := link.TracepointOptions{}
    tp, err := link.Tracepoint("sock", "inet_sock_set_state", objs.TraceInetSockSetState, &opts)
    if err != nil {
        return fmt.Errorf("attaching tracepoint: %v", err)
    }

    perfReader, err := perf.NewReader(objs.Events, os.Getpagesize())
    if err != nil {
        return fmt.Errorf("creating perf reader: %v", err)
    }

    listener := &BPFListener{
        PerfReader:           perfReader,
        ConstantsMap:         objs.ConstantsMap,
		BufferMap:            objs.BufferMap,
        ActiveConnectionsMap: objs.ActiveConnectionsMap,
		ScalingMap:           objs.ScalingMap,
        Events:               objs.Events,
        Tracepoint:           tp,
        closing:              make(chan struct{}),
        closed:               false,
    }

    go listener.listenForEvents()

	lowerLimitKey := uint32(0)
    upperLimitKey := uint32(1)
    bufferLengthKey := uint32(2)

    if err := listener.ConstantsMap.Put(lowerLimitKey, uint32(resource.LowerLimit)); err != nil {
        log.Fatalf("updating constants_map: %v", err)
    }
    if err := listener.ConstantsMap.Put(upperLimitKey, uint32(resource.UpperLimit)); err != nil {
        log.Fatalf("updating constants_map: %v", err)
    }
    if err := listener.ConstantsMap.Put(bufferLengthKey, uint32(resource.BufferLength)); err != nil {
        log.Fatalf("updating constants_map: %v", err)
    }

	listenerInstance = listener
	return nil

}

func (s *BPFListener) Close() {
    s.mu.Lock()
    defer s.mu.Unlock()

    if !s.closed {
        close(s.closing)
        s.Tracepoint.Close()
        s.PerfReader.Close()
        s.closed = true
    }
}

func (s *BPFListener) listenForEvents() {
    for {
        select {
        case <-s.closing:
            return
        default:
            record, err := s.PerfReader.Read()
            if err != nil {
                if err == perf.ErrClosed {
                    return // Exiting
                }
                logging.AddEventLog(fmt.Sprintf("Error reading perf event: %v", err))
                continue
            }

            if record.LostSamples > 0 {
                logging.AddEventLog(fmt.Sprintf("lost %d samples", record.LostSamples))
                continue
            }

            var data Data
            reader := bytes.NewReader(record.RawSample)
            if err := binary.Read(reader, binary.LittleEndian, &data); err != nil {
                log.Printf("parsing event data: %v", err)
                continue
            }

			direction := string(data.Message[:])
            if portCtx, ok := portToContext.Load(data.Port); ok {

				select {
				case <-portCtx.Ctx.Done():
					// context already cancelled
				default:
					portCtx.Signal <- direction
				}

			}
        }
    }
}

func addPort(port uint16, portCtx PortContext) error {
	if err := listenerInstance.ActiveConnectionsMap.Put(uint16(port), uint32(0)); err != nil {
        return err
    }

	addPortToScalingMap(uint16(port))

	portToContext.Store(port, portCtx)

	logging.AddEventLog(fmt.Sprintf("Monitoring on port %d", port))

	return nil
}

func removePort(port uint16) error {
	if err := listenerInstance.ActiveConnectionsMap.Delete(port); err != nil {
		log.Printf("Failed to delete port %d from ActiveConnectionsMap: %v", port, err)
		return err
	}

	if err := listenerInstance.BufferMap.Delete(port); err != nil {
		log.Printf("Failed to delete port %d from BufferMap: %v", port, err)
		return err
	}

	portToContext.Delete(port)
	return nil
}

func addPortToScalingMap(port uint16) error {
	if err := listenerInstance.ScalingMap.Put(port, uint32(0)); err != nil {
		log.Printf("Failed to update port %d from ScalingMap: %v", port, err)
	}

	return nil
}

func (resource *ConcReqResource) Monitor(ctx context.Context, containerID string, collectionPeriod time.Duration, swarmNodeInfo *server.SwarmNodeInfo) {

	serviceID, err := scale.FindServiceIDFromContainer(containerID)
	if err != nil {
		log.Fatalf("Couldn't get service ID in ConcReqResource Monitor")
	}

	port, err := scale.GetPublishedPort(serviceID)

	if err != nil {
		log.Fatalf("Couldn't get published port for service %s", serviceID)
	}

	ctx, cancel := context.WithCancel(ctx)
	signal := make(chan string)

	portCtx := PortContext{
		Ctx: ctx,
		Cancel: cancel,
		Signal: signal,
	}

	addPort(uint16(port), portCtx)

	cleanup := func() {
		if err := removePort(uint16(port)); err != nil {
			fmt.Println("Couldn't clean up BPF monitor for port %v", port)
		}
	}

	for {
		select {
		case <-ctx.Done():
			cleanup()
			logging.AddEventLog(fmt.Sprintf("Stopped monitoring for container %s", containerID))
			return
		case threshold := <-signal:
			var direction string
			

			// Trim null character from eBPF program
			threshold = strings.Trim(threshold, "\x00")
			
			if threshold == "Lower" {
				direction = "under"
			} else if threshold == "Upper" {
				direction = "over"
			} else {
				logging.AddEventLog(fmt.Sprintf("Invalid scaling direction."))
			}
			
			logging.AddEventLog(fmt.Sprintf("Scale triggered for port %d in direction %s\n", port, direction))
			
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

				if err := server.SendScaleRequest(serviceID, direction, managerNode.IP); err != nil {
					logging.AddEventLog(fmt.Sprintf("Error sending scale request to manager node: %v", err))
				}
			}

			// sleep to wait for scaling then add port back to BPF program
			time.Sleep(time.Second * 10)
			addPortToScalingMap(uint16(port))
		}
	}
}