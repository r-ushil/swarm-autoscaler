package main

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang BPF bpf/tc-port-monitor.c -- -I/usr/include/bpf

import (
    "encoding/binary"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
	"sync"
    "github.com/cilium/ebpf"
    "github.com/cilium/ebpf/link"
    "github.com/cilium/ebpf/perf"
    "github.com/cilium/ebpf/rlimit"
    "github.com/vishvananda/netlink"
)

var (
    portToServiceID sync.Map
)

type ScaleToZeroService struct {
    PerfReader *perf.Reader
    PortsMap   *ebpf.Map
    EventsMap  *ebpf.Map
    Link       link.Link
    closing    chan struct{}
}

func NewScaleToZeroService(ifaceName string) (*ScaleToZeroService, error) {
    // Allow the current process to lock memory for eBPF resources.
    if err := rlimit.RemoveMemlock(); err != nil {
        return nil, fmt.Errorf("failed to remove memlock limit: %v", err)
    }

    objs := BPFObjects{}
    if err := LoadBPFObjects(&objs, nil); err != nil {
        return nil, fmt.Errorf("loading objects: %s", err)
    }

    iface, err := netlink.LinkByName(ifaceName)
    if err != nil {
        return nil, fmt.Errorf("failed to get interface by name %s: %v", ifaceName, err)
    }

    qlen, err := link.AttachTCX(link.TCXOptions{
        Program:   objs.PortClassifier,
        Interface: iface.Attrs().Index,
        Attach:    ebpf.AttachTCXIngress,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to attach TC program: %v", err)
    }

    pr, err := perf.NewReader(objs.Events, os.Getpagesize())
    if err != nil {
        return nil, fmt.Errorf("failed to create perf event reader: %v", err)
    }

    s := &ScaleToZeroService{
        PerfReader: pr,
        PortsMap:   objs.PortsMap,
        EventsMap:  objs.Events,
        Link:       qlen,
        closing:    make(chan struct{}),
    }

    go s.listenForEvents()

    return s, nil
}

func (s *ScaleToZeroService) Close() {
    close(s.closing)
    s.Link.Close()
    s.PerfReader.Close()
}

func (s *ScaleToZeroService) listenForEvents() {
    log.Println("Listening for perf events...")
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
                log.Printf("Error reading perf event: %v", err)
                continue
            }

            if len(record.RawSample) >= 4 { // Ensure there's enough data for a uint32
                port := binary.LittleEndian.Uint32(record.RawSample[:4])
                serviceID, ok := portToServiceID.Load(port)
                if !ok {
                    log.Printf("Service ID for port %d not found", port)
                    continue
                }
                log.Printf("Packet detected on port %d, triggering scale action for service %s", port, serviceID)
                // Here, call the scaling function, for example:
                // scale.scaleTo(port, serviceID.(string), 1)
            } else {
                log.Println("Received malformed perf event")
            }
        }
    }
}


func (s *ScaleToZeroService) AddPort(port uint32, serviceID string) error {
    // Storing the service ID in the local Go map.
    portToServiceID.Store(port, serviceID)

    // Preparing the value to add to the eBPF map. You can adjust the value logic as needed.
    var value uint32 = 1 // Example value, adjust as necessary for your use case.
    
    // Adding the port to the eBPF map.
    if err := s.PortsMap.Update(port, value, ebpf.UpdateAny); err != nil {
        return fmt.Errorf("failed to add port to BPF map: %v", err)
    }
    return nil
}

func (s *ScaleToZeroService) RemovePort(port uint32) error {
    // Removing the port from the local Go map.
    _, ok := portToServiceID.Load(port)
    if !ok {
        return fmt.Errorf("service ID for port %d not found", port)
    }
    portToServiceID.Delete(port)

    // Removing the port from the eBPF map.
    if err := s.PortsMap.Delete(port); err != nil {
        return fmt.Errorf("failed to remove port from BPF map: %v", err)
    }

    // Call to other service logic that needs to happen upon port removal.
    // scale.scaleTo(port, serviceID.(string), 1)

    return nil
}


func main() {
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

    service, err := NewScaleToZeroService("wlp60s0") // Use "eth0" for example

    if err != nil {
        log.Fatalf("Failed to initialize scale to zero service: %v", err)
    }

     // Add port 8080 with service ID "my-service"
     if err := service.AddPort(8080, "my-service"); err != nil {
        log.Fatalf("Failed to add port to scale to zero service: %v", err)
    }

    <-sigs // Wait for interrupt signal
    log.Println("Shutting down...")
    service.Close()
}
