#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/pkt_cls.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/if_ether.h>

#ifndef IPPROTO_TCP
#define IPPROTO_TCP 6
#endif

// Define u32 for convenience
typedef unsigned int u32;

// Define the ports_map for monitoring specific TCP ports
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, u32);
    __type(value, u32);
    __uint(max_entries, 256);
} ports_map SEC(".maps");

// Define the events map for signaling packet detection on monitored ports
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __type(key, u32);
    __type(value, u32);
    __uint(max_entries, 8); // 8 CPUs, find the number of CPUs with `nproc`
} events SEC(".maps");

SEC("classifier")
int port_classifier(struct __sk_buff *skb) {

    void *data_end = (void *)(long)skb->data_end;
    void *data = (void *)(long)skb->data;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) {
        return TC_ACT_OK;
    }
    if (eth->h_proto != __constant_htons(ETH_P_IP)) {
        return TC_ACT_OK;
    }

    struct iphdr *ip = data + sizeof(*eth);
    if ((void *)(ip + 1) > data_end) {
        return TC_ACT_OK;
    }
    if (ip->protocol != IPPROTO_TCP) {
        return TC_ACT_OK;
    }

    struct tcphdr *tcp = (void *)ip + sizeof(*ip);
    if ((void *)(tcp + 1) > data_end) {
        return TC_ACT_OK;
    }

    u32 tcp_dest_port = __builtin_bswap16(tcp->dest); // Convert network byte order to host byte order
    u32 *found = bpf_map_lookup_elem(&ports_map, &tcp_dest_port);
    if (found) {
        long value = tcp_dest_port; // Pass the detected port as the value
        bpf_perf_event_output(skb, &events, BPF_F_CURRENT_CPU, &value, sizeof(value));
        //bpf_printk("Sent perf to scale to 1");
    }

    return TC_ACT_OK;
}

char _license[] SEC("license") = "GPL";
