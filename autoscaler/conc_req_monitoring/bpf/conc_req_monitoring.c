#include <uapi/linux/ptrace.h>
#include <net/sock.h>
#include <bcc/proto.h>
#include <linux/tcp.h>

struct data_t {
    u16 port;
    u32 bufferLength;
    char message[6];
};

BPF_HASH(active_connections_map, u16, u32);
BPF_HASH(constants_map, u32, u32);
BPF_HASH(buffer_map, u16, u32);
BPF_PERF_OUTPUT(events);

TRACEPOINT_PROBE(sock, inet_sock_set_state) {
    u16 dport = args->dport;  // destination port
    u32 *active_connections;
    u32 new_value;
    u32 key;
    u32 *lowerLimit, *upperLimit, *bufferLength, *buffer;
    struct data_t data = {};

    // Retrieve constants from the map
    key = 0;
    lowerLimit = constants_map.lookup(&key);
    key = 1;
    upperLimit = constants_map.lookup(&key);
    key = 2;
    bufferLength = constants_map.lookup(&key);

    // Check if the port is in the active_connections_map
    active_connections = active_connections_map.lookup(&dport);
    if (!active_connections || !lowerLimit || !upperLimit || !bufferLength) {
        return 0;
    }

    if (args->newstate == 1) {  // TCP_ESTABLISHED
        new_value = *active_connections + 1;
        active_connections_map.update(&dport, &new_value);
    } else if (args->newstate == 7) {  // TCP_CLOSE
        if (*active_connections > 0) {
            new_value = *active_connections - 1;
            active_connections_map.update(&dport, &new_value);
        }
    } else {
        return 0;
    }

    // Update buffer map based on active connections
    buffer = buffer_map.lookup(&dport);
    if (!buffer) {
        u32 initial = 0;
        buffer_map.update(&dport, &initial);
        buffer = buffer_map.lookup(&dport);
        if (!buffer) {
            return 0;  // Safety check
        }
    }

    if (new_value <= *lowerLimit || new_value >= *upperLimit) {
        (*buffer)++;
    } else {
        *buffer = 0;
    }

    buffer_map.update(&dport, buffer);

    if (*buffer == *bufferLength) {
        data.port = dport;
        data.bufferLength = *bufferLength;
        if (new_value <= *lowerLimit) {
            __builtin_memcpy(data.message, "Lower", 5);
        } else {
            __builtin_memcpy(data.message, "Upper", 5);
        }
        data.message[5] = '\\0';  // Ensure null-termination
        events.perf_submit(args, &data, sizeof(data));
        
        u32 zero = 0;
        buffer_map.update(&dport, &zero);
    }

    return 0;
}
