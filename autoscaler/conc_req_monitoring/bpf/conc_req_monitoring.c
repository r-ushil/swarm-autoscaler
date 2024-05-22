#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/ptrace.h>
#include <linux/tcp.h>
#include <linux/types.h> // Include for __u16, __u32

// Define maps
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u16);
    __type(value, __u32);
    __uint(max_entries, 1024);
} active_connections_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u32);
    __type(value, __u32);
    __uint(max_entries, 10);
} constants_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, __u16);
    __type(value, __u32);
    __uint(max_entries, 1024);
} buffer_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
} events SEC(".maps");

struct data_t {
    __u16 port;
    char message[6];
};

// Manually define the tracepoint structure
struct trace_event_raw_inet_sock_set_state {
    __u64 unused;     // Padding to align the structure
    __u64 skaddr;     // Socket address
    __u32 oldstate;   // Previous state
    __u32 newstate;   // New state
    __u16 sport;      // Source port
    __u16 dport;      // Destination port
    __u16 family;     // Protocol family
};

SEC("tracepoint/sock/inet_sock_set_state")
int trace_inet_sock_set_state(struct trace_event_raw_inet_sock_set_state *ctx) {
    __u16 dport = ctx->dport;
    __u32 *active_connections;
    __u32 new_value;
    __u32 key;
    __u32 *lowerLimit, *upperLimit, *bufferLength, *buffer;
    struct data_t data = {};

    // Retrieve constants from the map
    key = 0;
    lowerLimit = bpf_map_lookup_elem(&constants_map, &key);
    key = 1;
    upperLimit = bpf_map_lookup_elem(&constants_map, &key);
    key = 2;
    bufferLength = bpf_map_lookup_elem(&constants_map, &key);

    

    // Check if the port is in the active_connections_map
    active_connections = bpf_map_lookup_elem(&active_connections_map, &dport);
    if (!active_connections || !lowerLimit || !upperLimit || !bufferLength) {
        return 0;
    }

    bpf_printk("Tracepoint hit");

    if (ctx->newstate == 1) {  // TCP_ESTABLISHED
        new_value = *active_connections + 1;
        bpf_map_update_elem(&active_connections_map, &dport, &new_value, BPF_ANY);
    } else if (ctx->newstate == 7) {  // TCP_CLOSE
        if (*active_connections > 0) {
            new_value = *active_connections - 1;
            bpf_map_update_elem(&active_connections_map, &dport, &new_value, BPF_ANY);
        }
    } else {
        return 0;
    }

    // Update buffer map based on active connections
    buffer = bpf_map_lookup_elem(&buffer_map, &dport);
    if (!buffer) {
        __u32 initial = 0;
        bpf_map_update_elem(&buffer_map, &dport, &initial, BPF_ANY);
        buffer = bpf_map_lookup_elem(&buffer_map, &dport);
        if (!buffer) {
            return 0;  // Safety check
        }
    }

    if (new_value <= *lowerLimit || new_value >= *upperLimit) {
        (*buffer)++;
    } else {
        *buffer = 0;
    }

    bpf_map_update_elem(&buffer_map, &dport, buffer, BPF_ANY);

    if (*buffer == *bufferLength) {
        data.port = dport;
        if (new_value <= *lowerLimit) {
            __builtin_memcpy(data.message, "Lower", 5);
        } else {
            __builtin_memcpy(data.message, "Upper", 5);
        }
        data.message[5] = '\0';  // Ensure null-termination

        bpf_printk("Perf event: port=%d, message=%s\n", data.port, data.message);

        bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &data, sizeof(data));
        
        __u32 zero = 0;
        bpf_map_update_elem(&buffer_map, &dport, &zero, BPF_ANY);
    }

    return 0;
}

char _license[] SEC("license") = "GPL";
