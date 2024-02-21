#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

typedef unsigned int u32;

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __type(key, u32);
    __type(value, u32);
    __uint(max_entries, 8); // 8 CPUs, find the number of CPUs with `nproc`
} perf_event_map SEC(".maps");

SEC("cgroup_skb/ingress")
int detect_first_packet(struct __sk_buff *skb) {
    int key = 0; // Use a single key for simplicity
    long data = 1; // Data to send, indicating an event

    bpf_perf_event_output(skb, &perf_event_map, BPF_F_CURRENT_CPU, &data, sizeof(data));
    return BPF_OK;
}

char _license[] SEC("license") = "GPL";

