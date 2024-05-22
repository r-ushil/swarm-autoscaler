#include <linux/ptrace.h>
#include <linux/tcp.h>
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/version.h>
#include <stdint.h>  // For standard integer types

// Define bpf_map_def if not defined
struct bpf_map_def {
    enum bpf_map_type type;
    uint32_t key_size;
    uint32_t value_size;
    uint32_t max_entries;
    uint32_t map_flags;
};

// Define structure based on tracepoint structure
struct trace_event_raw_inet_sock_set_state {
    uint16_t dport;
    uint32_t newstate;
};

struct data_t {
    uint16_t port;
    uint32_t bufferLength;
    char message[6];
};

struct bpf_map_def SEC("maps") active_connections_map = {
    .type = BPF_MAP_TYPE_HASH,
    .key_size = sizeof(uint16_t),
    .value_size = sizeof(uint32_t),
    .max_entries = 1024,
};

struct bpf_map_def SEC("maps") constants_map = {
    .type = BPF_MAP_TYPE_HASH,
    .key_size = sizeof(uint32_t),
    .value_size = sizeof(uint32_t),
    .max_entries = 3,
};

struct bpf_map_def SEC("maps") buffer_map = {
    .type = BPF_MAP_TYPE_HASH,
    .key_size = sizeof(uint16_t),
    .value_size = sizeof(uint32_t),
    .max_entries = 1024,
};

struct bpf_map_def SEC("maps") events = {
    .type = BPF_MAP_TYPE_PERF_EVENT_ARRAY,
    .key_size = sizeof(int),
    .value_size = sizeof(int),
    .max_entries = 1024,
};

SEC("tracepoint/sock/inet_sock_set_state")
int trace_inet_sock_set_state(struct trace_event_raw_inet_sock_set_state *args) {
    uint16_t dport = args->dport;  // destination port
    uint32_t *active_connections;
    uint32_t new_value;
    uint32_t key;
    uint32_t *lowerLimit, *upperLimit, *bufferLength, *buffer;
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

    if (args->newstate == 1) {  // TCP_ESTABLISHED
        new_value = *active_connections + 1;
        bpf_map_update_elem(&active_connections_map, &dport, &new_value, BPF_ANY);
    } else if (args->newstate == 7) {  // TCP_CLOSE
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
        uint32_t initial = 0;
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
        data.bufferLength = *bufferLength;
        if (new_value <= *lowerLimit) {
            __builtin_memcpy(data.message, "Lower", 5);
        } else {
            __builtin_memcpy(data.message, "Upper", 5);
        }
        data.message[5] = '\0';  // Ensure null-termination
        bpf_perf_event_output(args, &events, BPF_F_CURRENT_CPU, &data, sizeof(data));

        uint32_t zero = 0;
        bpf_map_update_elem(&buffer_map, &dport, &zero, BPF_ANY);
    }

    return 0;
}

char _license[] SEC("license") = "GPL";

