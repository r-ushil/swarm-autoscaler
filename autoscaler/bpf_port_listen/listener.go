package bpf_port_listen

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang BPF bpf/tc-port-monitor.c -- -I/usr/include/bpf

import (
    "encoding/binary"
    "fmt"
    "log"
    "os"
    "sync"
    "github.com/cilium/ebpf"
    "github.com/cilium/ebpf/link"
    "github.com/cilium/ebpf/perf"
    "github.com/cilium/ebpf/rlimit"
    "github.com/vishvananda/netlink"
)


type Scaler interface {
    ScaleTo(serviceID string, replicas uint64) error
}

var (
    portToServiceID sync.Map
    listenerInstance *BPFListener
    once sync.Once
)

type BPFListener struct {
    PerfReader    *perf.Reader
    PortsMap      *ebpf.Map
    EventsMap     *ebpf.Map
    Link          link.Link
    closing       chan struct{}
    Scaler Scaler
}

func GetBPFListener(ifaceName string) (*BPFListener, error) {
    var err error
    once.Do(func() {
        listenerInstance, err = initBPFPortListener(ifaceName)
    })
    return listenerInstance, err
}

func (s *BPFListener) SetScaler(scale Scaler) {
    s.Scaler = scale
}

func initBPFPortListener(ifaceName string) (*BPFListener, error) {
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

    s := &BPFListener{
        PerfReader: pr,
        PortsMap:   objs.PortsMap,
        EventsMap:  objs.Events,
        Link:       qlen,
        closing:    make(chan struct{}),
        Scaler:     nil,
    }

    go s.listenForEvents()

    return s, nil
}

func (s *BPFListener) Close() {
    close(s.closing)
    s.Link.Close()
    s.PerfReader.Close()
}

func (s *BPFListener) listenForEvents() {
    log.Println("BPF program setup, listening for perf events...")
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
                
                // remove the port and scale back up
                if err := s.RemovePort(port); err != nil {
                    log.Printf("Failed to remove port %d: %v", port, err)
                }

            } else {
                log.Println("Received malformed perf event")
            }
        }
    }
}


func (s *BPFListener) ListenOnPort(port uint32, serviceID string) error {
    // Storing the service ID in the local Go map.
    portToServiceID.Store(port, serviceID)

    var value uint32 = 1 // need a fixed value for the eBPF map
    
    if err := s.PortsMap.Update(port, value, ebpf.UpdateAny); err != nil {
        return fmt.Errorf("failed to add port to BPF map: %v", err)
    }
    return nil
}

func (s *BPFListener) RemovePort(port uint32) error {
    // Removing the port from the local Go map.
    serviceID, ok := portToServiceID.Load(port)
    if !ok {
        return fmt.Errorf("service ID for port %d not found", port)
    }
    portToServiceID.Delete(port)

    // Removing the port from the eBPF map.
    if err := s.PortsMap.Delete(port); err != nil {
        return fmt.Errorf("failed to remove port from BPF map: %v", err)
    }

    fmt.Printf("Removed port %d from BPF listener\n", port)
    fmt.Printf("Scaling service %s back up\n", serviceID)

    // Call scaler to scale back up to 1.
    s.Scaler.ScaleTo(serviceID.(string), 1)

    return nil
}
