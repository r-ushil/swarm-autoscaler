from bcc import BPF
import ctypes
import time

# eBPF program to monitor TCP connections on specified ports using tracepoints
bpf_program = """
#include <uapi/linux/ptrace.h>
#include <net/sock.h>
#include <bcc/proto.h>
#include <linux/tcp.h>

BPF_HASH(active_connections_map, u16, u32);
BPF_HASH(constants_map, u32, u32);
BPF_HASH(buffer_map, u16, u32);
BPF_PERF_OUTPUT(events);

struct data_t {
    u16 port;
    char message[6];
};

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
"""

# Load the eBPF program
b = BPF(text=bpf_program)

# Map to hold constants
constants_map = b["constants_map"]
lower_limit_key = ctypes.c_uint32(0)
upper_limit_key = ctypes.c_uint32(1)
buffer_length_key = ctypes.c_uint32(2)

# Set constants
constants_map[lower_limit_key] = ctypes.c_uint32(3)  # lowerLimit
constants_map[upper_limit_key] = ctypes.c_uint32(10)  # upperLimit
constants_map[buffer_length_key] = ctypes.c_uint32(5)  # bufferLength

# Port 8080 (0x1F90 in hex) and count=0
active_connections_map = b["active_connections_map"]
port_8080 = ctypes.c_uint16(0x1F90)
active_connections_map[port_8080] = ctypes.c_uint32(0)

# Map to hold port buffer values
buffer_map = b["buffer_map"]

# Define the data structure in Python
class Data(ctypes.Structure):
    _fields_ = [("port", ctypes.c_uint16),
                ("message", ctypes.c_char * 6)]

# Callback function to handle events
def print_event(cpu, data, size):
    event = ctypes.cast(data, ctypes.POINTER(Data)).contents
    print(f"Port: {event.port}, Exceeded: {event.message.decode('utf-8')}")

# Open perf buffer to receive events
b["events"].open_perf_buffer(print_event)

print("Monitoring active TCP connections on specified ports... Press Ctrl+C to exit.")

try:
    while True:
        b.perf_buffer_poll()
        time.sleep(2)
except KeyboardInterrupt:
    print("Exiting...")
