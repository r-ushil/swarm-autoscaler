package conc_req_monitoring

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang BPF bpf/conc_req_monitoring.c -- -I/usr/include -g
import (
    "context"
    "bytes"
    "encoding/binary"
    "fmt"
    "log"
    "os"
    "strings"
    "sync"
    "time"

    "github.com/cilium/ebpf"
    "github.com/cilium/ebpf/link"
    "github.com/cilium/ebpf/perf"
    "github.com/cilium/ebpf/rlimit"
    "scale"
    "server"
)

type Data struct {
    Netns   uint32
    Message [6]byte
}

type BPFListener struct {
    PerfReader       *perf.Reader
    ConstantsMap     *ebpf.Map
    BufferMap        *ebpf.Map
    ConnCountMap     *ebpf.Map
    ScalingMap       *ebpf.Map
    ValidNetnsMap    *ebpf.Map
    Events           *ebpf.Map
    TcpRecvMsgLink   link.Link
    closing          chan struct{}
    closed           bool
    mu               sync.Mutex
}

type ConcReqResource struct {
    LowerLimit   int64
    UpperLimit   int64
    BufferLength int64
}

type NamespaceContext struct {
    Ctx    context.Context
    Cancel context.CancelFunc
    Signal chan string
}

type NamespaceContextMap struct {
    internalMap sync.Map
}

func (ncm *NamespaceContextMap) Store(netns uint32, nsCtx NamespaceContext) {
    ncm.internalMap.Store(netns, nsCtx)
}

func (ncm *NamespaceContextMap) Load(netns uint32) (NamespaceContext, bool) {
    value, ok := ncm.internalMap.Load(netns)
    if !ok {
        return NamespaceContext{}, false
    }
    return value.(NamespaceContext), true
}

func (ncm *NamespaceContextMap) Delete(netns uint32) {
    ncm.internalMap.Delete(netns)
}

var (
    namespaceToContext NamespaceContextMap
    listenerInstance   *BPFListener
    once               sync.Once
)

func InitBPFListener(resource ConcReqResource) error {
    // Allow the current process to lock memory for eBPF resources.
    if err := rlimit.RemoveMemlock(); err != nil {
        return fmt.Errorf("failed to remove memlock limit: %v", err)
    }

    
    objs := BPFObjects{}
    opts := ebpf.CollectionOptions{
        Programs: ebpf.ProgramOptions{
            LogLevel: ebpf.LogLevelInstruction,
            LogSize:  64 * 1024, // Adjust log size if needed
        },
    }

    if err := LoadBPFObjects(&objs, &opts); err != nil {
        return fmt.Errorf("loading objects: %v", err)
    }

    fmt.Println("Loading program...")

    // Attach the eBPF program to the kprobes
    tcpRecvMsgLink, err := link.Kprobe("tcp_recvmsg", objs.KprobeTcpRecvmsg, nil)
    if err != nil {
        return fmt.Errorf("attaching tcp_recvmsg kprobe: %v", err)
    }

    perfReader, err := perf.NewReader(objs.Events, os.Getpagesize())
    if err != nil {
        return fmt.Errorf("creating perf reader: %v", err)
    }

    listener := &BPFListener{
        PerfReader:       perfReader,
        ConstantsMap:     objs.ConstantsMap,
        BufferMap:        objs.BufferMap,
        ConnCountMap:     objs.ConnCountMap,
        ScalingMap:       objs.ScalingMap,
        ValidNetnsMap:    objs.ValidNetnsMap,
        Events:           objs.Events,
        TcpRecvMsgLink:   tcpRecvMsgLink,
        closing:          make(chan struct{}),
        closed:           false,
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
        s.TcpRecvMsgLink.Close()
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
                fmt.Printf("Error reading perf event: %v\n", err)
                continue
            }

            if record.LostSamples > 0 {
                fmt.Printf("lost %d samples\n", record.LostSamples)
                continue
            }

            var data Data
            reader := bytes.NewReader(record.RawSample)
            if err := binary.Read(reader, binary.LittleEndian, &data); err != nil {
                fmt.Printf("parsing event data: %v\n", err)
                continue
            }

            direction := string(data.Message[:])
            if nsCtx, ok := namespaceToContext.Load(data.Netns); ok {

                select {
                case <-nsCtx.Ctx.Done():
                    // context already cancelled
                default:
                    nsCtx.Signal <- direction
                }

            }
        }
    }
}

func addNamespace(netns uint32, nsCtx NamespaceContext) error {
    if err := listenerInstance.ConnCountMap.Put(netns, uint32(0)); err != nil {
        log.Fatalf("Failed to add namespace %d to ConnCountMap: %v", netns, err)
        return err
    }

    if err := listenerInstance.ValidNetnsMap.Put(netns, uint32(1)); err != nil {
        log.Fatalf("Failed to add namespace %d to ValidNetnsMap: %v", netns, err)
        return err
    }

    addNamespaceToScalingMap(netns)

    namespaceToContext.Store(netns, nsCtx)

    fmt.Printf("Monitoring on namespace %d\n", netns)

    return nil
}

func removeNamespace(netns uint32) error {
    if err := listenerInstance.ConnCountMap.Delete(netns); err != nil {
        fmt.Printf("Failed to delete namespace %d from ConnCountMap: %v\n", netns, err)
        return err
    }

    if err := listenerInstance.BufferMap.Delete(netns); err != nil {
        fmt.Printf("Failed to delete namespace %d from BufferMap: %v\n", netns, err)
        return err
    }

    if err := listenerInstance.ValidNetnsMap.Delete(netns); err != nil {
        fmt.Printf("Failed to delete namespace %d from ValidNetnsMap: %v\n", netns, err)
        return err
    }

    namespaceToContext.Delete(netns)
    return nil
}

func addNamespaceToScalingMap(netns uint32) error {
    if err := listenerInstance.ScalingMap.Put(netns, uint32(0)); err != nil {
        fmt.Printf("Failed to update namespace %d in ScalingMap: %v\n", netns, err)
    }

    return nil
}

func (resource *ConcReqResource) Monitor(ctx context.Context, containerID string, collectionPeriod time.Duration, swarmNodeInfo *server.SwarmNodeInfo) {

    serviceID, err := scale.FindServiceIDFromContainer(containerID)
    if err != nil {
        log.Fatalf("Couldn't get service ID in ConcReqResource Monitor")
    }

    netns, err := scale.GetContainerNamespace(containerID)
    if err != nil {
        log.Fatalf("Couldn't get network namespace for container %s: %v", containerID, err)
    }

    ctx, cancel := context.WithCancel(ctx)
    signal := make(chan string)

    nsCtx := NamespaceContext{
        Ctx:    ctx,
        Cancel: cancel,
        Signal: signal,
    }

    addNamespace(netns, nsCtx)

    cleanup := func() {
        if err := removeNamespace(netns); err != nil {
            fmt.Printf("Couldn't clean up BPF monitor for namespace %v\n", netns)
        }
    }

    for {
        select {
        case <-ctx.Done():
            cleanup()
            fmt.Printf("Stopped monitoring for container %s\n", containerID)
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
                fmt.Printf("Invalid scaling direction.\n")
            }

            fmt.Printf("Scale triggered for namespace %d in direction %s\n", netns, direction)

            if swarmNodeInfo.AutoscalerManager {
                if err := scale.ScaleService(containerID, direction); err != nil {
                    fmt.Printf("Error scaling service for container %s: %v\n", containerID, err)
                }
            } else {
                managerNode, err := server.GetManagerNode(swarmNodeInfo.OtherNodes)
                if err != nil {
                    fmt.Printf("Error getting manager node: %v\n", err)
                    return
                }

                if err := server.SendScaleRequest(serviceID, direction, managerNode.IP); err != nil {
                    fmt.Printf("Error sending scale request to manager node: %v\n", err)
                }
            }

            // Sleep to wait for scaling then add namespace back to BPF program
            time.Sleep(time.Second * 5)
            addNamespaceToScalingMap(netns)
        }
    }
}
