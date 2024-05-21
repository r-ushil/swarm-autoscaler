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

TRACEPOINT_PROBE(sock, inet_sock_set_state) {
    u16 dport = args->dport;  // destination port
    u32 *active_connections;
    u32 new_value;

    // Check if the port is in the active_connections_map
    active_connections = active_connections_map.lookup(&dport);
    if (!active_connections) {
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
    }

    return 0;
}
"""

# Load the eBPF program
b = BPF(text=bpf_program)

# port 8080 (0x1F90 in hex) and count=0
active_connections_map = b["active_connections_map"]
port_8080 = ctypes.c_uint16(0x1F90)
active_connections_map[port_8080] = ctypes.c_uint32(0)

print("Monitoring active TCP connections on specified ports... Press Ctrl+C to exit.")


# TODO: need to periodically clean up map in userspace because of lost TCP_CLOSE packets
try:
    while True:
        print("Active connections per port:")
        for key, leaf in b["active_connections_map"].items():
            print(f"Port {key.value}: {leaf.value} active connections")
        time.sleep(2)
except KeyboardInterrupt:
    print("Exiting...")
