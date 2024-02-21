#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

typedef unsigned int u32;

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __type(key, u32);
    __type(value, u32);
    __uint(max_entries, 1);
} signal_map SEC(".maps");

SEC("cgroup_skb/ingress")
int detect_first_packet(struct __sk_buff *skb) {
    u32 key = 0;
    u32 value = 1;
    bpf_map_update_elem(&signal_map, &key, &value, BPF_ANY);
    return BPF_OK;
}

char _license[] SEC("license") = "GPL";

