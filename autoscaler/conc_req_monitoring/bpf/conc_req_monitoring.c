#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_core_read.h>

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, u32);
    __type(value, u32);
    __uint(max_entries, 1024);
} conn_count_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, u32);
    __type(value, u32);
    __uint(max_entries, 1024);
} constants_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, u32);
    __type(value, u32);
    __uint(max_entries, 1024);
} buffer_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, u32);
    __type(value, u32);
    __uint(max_entries, 1024);
} scaling_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, u32);
    __type(value, u32);
    __uint(max_entries, 1024);
} valid_netns_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(max_entries, 8); // Set this to the number of CPUs
} events SEC(".maps");

struct data_t {
    u32 netns;
    char message[6];
};

static __always_inline u32 get_netns_from_task(struct task_struct *task) {
    struct nsproxy *ns = NULL;
    struct net *net = NULL;
    u32 netns = 0;

    bpf_core_read(&ns, sizeof(ns), &task->nsproxy);
    if (ns) {
        bpf_core_read(&net, sizeof(net), &ns->net_ns);
        if (net) {
            bpf_core_read(&netns, sizeof(netns), &net->ns.inum);
        }
    }
    return netns;
}

SEC("kprobe/tcp_recvmsg")
int kprobe_tcp_recvmsg(struct pt_regs *ctx) {
    struct sock *sk = NULL;
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();
    u32 netns = get_netns_from_task(task);

    //bpf_printk("netns: %u\n", netns);


    // Read the sk pointer from the first argument of tcp_recvmsg
    bpf_probe_read_kernel(&sk, sizeof(sk), (void *)&ctx->di);

    if (!sk) {
        //bpf_printk("Failed to read sk pointer\n");
        return 0;
    }

    // Check if the namespace is valid for tracked containers
    u32 *valid_ns = bpf_map_lookup_elem(&valid_netns_map, &netns);
    if (!valid_ns) {
        //bpf_printk("Namespace %u is not valid.\n", netns);
        return 0;
    }

    // Determine the TCP state using bpf_core_read
    u8 state = 0;
    bpf_core_read(&state, sizeof(state), &sk->__sk_common.skc_state);

    //bpf_printk("TCP state: %u\n", state);

    u32 new_value = 0;
    u32 key = 0;
    u32 *lowerLimit = bpf_map_lookup_elem(&constants_map, &key);
    key = 1;
    u32 *upperLimit = bpf_map_lookup_elem(&constants_map, &key);
    key = 2;
    u32 *bufferLength = bpf_map_lookup_elem(&constants_map, &key);

    if (!lowerLimit || !upperLimit || !bufferLength) {
        //bpf_printk("Missing constants.\n");
        return 0;
    }

    u32 zero32 = 0;
    u32 *count = bpf_map_lookup_elem(&conn_count_map, &netns);
    if (!count) {
        bpf_map_update_elem(&conn_count_map, &netns, &zero32, BPF_NOEXIST);
        count = &zero32;
    }

    if (state == TCP_ESTABLISHED) {
        (*count)++;
        new_value = *count;
    } else if (state == TCP_CLOSE || state == TCP_CLOSE_WAIT) {
        if (*count > 0) {
            (*count)--;
            new_value = *count;
        }
    }

    //bpf_printk("Connection count for netns %u: %u\n", netns, new_value);

    u32 *scaling = bpf_map_lookup_elem(&scaling_map, &netns);
    if (scaling && *scaling == 1) {
        return 0;
    }

    u32 initial = 0;
    u32 *buffer = bpf_map_lookup_elem(&buffer_map, &netns);
    if (!buffer) {
        bpf_map_update_elem(&buffer_map, &netns, &initial, BPF_NOEXIST);
        buffer = &initial;
    }

    if (new_value <= *lowerLimit || new_value >= *upperLimit) {
        (*buffer)++;
    } else {
        *buffer = 0;
    }

    bpf_map_update_elem(&buffer_map, &netns, buffer, BPF_ANY);

    if (*buffer == *bufferLength) {
        u32 scaling_value = 1;
        bpf_map_update_elem(&scaling_map, &netns, &scaling_value, BPF_ANY);

        struct data_t data = {};
        data.netns = netns;
        if (new_value <= *lowerLimit) {
            __builtin_memcpy(data.message, "Lower", 5);
        } else {
            __builtin_memcpy(data.message, "Upper", 5);
        }
        data.message[5] = '\0';

        bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &data, sizeof(data));

        u32 zero = 0;
        bpf_map_update_elem(&buffer_map, &netns, &zero, BPF_ANY);

        //bpf_printk("Scaling triggered for netns %u\n", netns);
    }

    return 0;
}

char LICENSE[] SEC("license") = "GPL";
