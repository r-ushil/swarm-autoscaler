package cgroup_net_listen

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go bpf ./bpf/cgroup-ingress-perf.c -- -I/usr/include/bpf -O2

import (
    "fmt"
    "log"
	"os"

    "github.com/cilium/ebpf"
    "github.com/cilium/ebpf/link"
    "github.com/cilium/ebpf/perf"
    "github.com/cilium/ebpf/rlimit"
    
)

const cgroupPath = "/sys/fs/cgroup/system.slice"

func SetupBPFListener(containerID string) {

    if err := rlimit.RemoveMemlock(); err != nil {
        log.Fatal(err)
    }

    objs := bpfObjects{}
    if err := loadBpfObjects(&objs, nil); err != nil {
        log.Fatalf("loading objects: %v", err)
    }
    defer objs.Close()

    cgroupFd, err := os.Open(fmt.Sprintf("%s/docker-%s.scope", cgroupPath, containerID))
    if err != nil {
        log.Fatalf("Failed to open cgroup: %v", err)
    }
    defer cgroupFd.Close()

    l, err := link.AttachCgroup(link.CgroupOptions{
        Path:    cgroupFd.Name(),
        Attach:  ebpf.AttachCGroupInetIngress,
        Program: objs.DetectFirstPacket,
    })
    if err != nil {
        log.Fatalf("Failed to attach eBPF program to cgroup: %v", err)
    }
    defer l.Close()

    listenForPacketPerfEvent(&objs)

}

func listenForPacketPerfEvent(objs *bpfObjects) {
    rd, err := perf.NewReader(objs.PerfEventMap, os.Getpagesize())
    if err != nil {
        log.Fatalf("Failed to create perf event reader: %v", err)
    }
    defer rd.Close()

    fmt.Println("Listening for the first packet event...")
    for {
        record, err := rd.Read()
        if err != nil {
            if err == perf.ErrClosed {
                fmt.Println("Perf event reader closed, exiting...")
                return
            }
            log.Printf("Error reading perf event: %v", err)
            continue
        }

        if record.LostSamples != 0 {
            log.Printf("Lost %d samples", record.LostSamples)
        }

        break
    }
}
