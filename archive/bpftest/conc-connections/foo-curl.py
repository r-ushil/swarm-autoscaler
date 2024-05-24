from bcc import BPF
import docker
import time
import ctypes as ct
import os

'''
STATE OF AFFAIRS: Works will cURL well. Not with ab2 at all.
'''

# Define the container IDs
container_ids = [
    "10d48902bbbac3c1397325803f51af11c5b9df8583d21df56e71a13bb4b8d442",
    # Add more container IDs as needed
]

# Initialize Docker client
client = docker.from_env()

# Get container PIDs and network namespaces
container_info = []
for container_id in container_ids:
    container = client.containers.get(container_id)
    pid = container.attrs['State']['Pid']
    netns_path = f"/proc/{pid}/ns/net"
    netns = os.stat(netns_path).st_ino
    container_info.append({"pid": pid, "netns": netns})

for info in container_info:
    print(f"Container PID: {info['pid']}, Network Namespace: {info['netns']}")

# BPF program
bpf_text = """
#include <uapi/linux/ptrace.h>
#include <net/sock.h>
#include <bcc/proto.h>
#include <linux/tracepoint.h>
#include <linux/tcp.h>
#include <linux/sched.h>

BPF_HASH(conn_count_map, u32, u32);
BPF_HASH(constants_map, u32, u32);
BPF_HASH(buffer_map, u32, u32);
BPF_HASH(scaling_map, u32, u32);
BPF_HASH(valid_netns_map, u32, u32);
BPF_PERF_OUTPUT(events);

struct data_t {
    u32 netns;
    char message[6];
};

static inline u32 get_netns_from_pid(void) {
    struct task_struct *task;
    struct nsproxy *ns;
    struct net *net;
    u32 netns = 0;

    task = (struct task_struct *)bpf_get_current_task();
    bpf_probe_read_kernel(&ns, sizeof(ns), &task->nsproxy);
    if (ns) {
        bpf_probe_read_kernel(&net, sizeof(net), &ns->net_ns);
        if (net) {
            bpf_probe_read_kernel(&netns, sizeof(netns), &net->ns.inum);
        }
    }
    return netns;
}

int kprobe__tcp_rcv_state_process(struct pt_regs *ctx, struct sock *sk) {
    u32 netns = get_netns_from_pid();
    netns = (u32)(netns);  // Ensure netns is treated as unsigned

    // Check if the namespace is valid for tracked containers
    u32 *valid_ns = valid_netns_map.lookup(&netns);
    if (!valid_ns) {
        // bpf_trace_printk("Namespace %u not tracked, skipping\\n", netns);
        return 0;
    }

    bpf_trace_printk("Valid Namespace ID: %u\\n", *valid_ns);

    // Determine the TCP state
    u8 state = sk->sk_state;
    // if (state == TCP_ESTABLISHED) {
    //     bpf_trace_printk("TCP state: ESTABLISHED\\n");
    // } else if (state == TCP_CLOSE) {
    //     bpf_trace_printk("TCP state: CLOSE\\n");
    // } else {
    //     bpf_trace_printk("TCP state: %u\\n", state);
    // }

    u32 new_value = 0;
    u32 key = 0;
    u32 *lowerLimit = constants_map.lookup(&key);
    key = 1;
    u32 *upperLimit = constants_map.lookup(&key);
    key = 2;
    u32 *bufferLength = constants_map.lookup(&key);

    if (!lowerLimit || !upperLimit || !bufferLength) {
        bpf_trace_printk("Constants not found in maps\\n");
        return 0;
    }

    u32 zero32 = 0;
    u32 *count = conn_count_map.lookup_or_try_init(&netns, &zero32);
    if (count) {
        if (state == 4) { // TCP_FIN_WAIT1
            (*count)++;
            new_value = *count;
            bpf_trace_printk("Increment count: %u\\n", *count);
        } else if (state == 9) { // TCP_LAST_ACK
            if (*count > 0) {
                (*count)--;
                new_value = *count;
                bpf_trace_printk("Decrement count: %u\\n", *count);
            }
        }
    }

    u32 *scaling = scaling_map.lookup(&netns);
    if (scaling && *scaling == 1) {
        bpf_trace_printk("Scaling is active, skipping buffer update\\n");
        return 0;
    }

    u32 initial = 0;
    u32 *buffer = buffer_map.lookup_or_try_init(&netns, &initial);
    if (!buffer) {
        bpf_trace_printk("Buffer map initialization failed\\n");
        return 0;
    }

    if (new_value <= *lowerLimit || new_value >= *upperLimit) {
        (*buffer)++;
        // bpf_trace_printk("Buffer incremented, netns: %u, buffer: %u\\n", netns, *buffer);
    } else {
        *buffer = 0;
        // bpf_trace_printk("Buffer reset, netns: %u\\n", netns);
    }

    buffer_map.update(&netns, buffer);

    if (*buffer == *bufferLength) {
        u32 scaling_value = 1;
        scaling_map.update(&netns, &scaling_value);
        bpf_trace_printk("Buffer limit reached, netns: %u\\n", netns);

        struct data_t data = {};
        data.netns = netns;
        if (new_value <= *lowerLimit) {
            __builtin_memcpy(data.message, "Lower", 5);
        } else {
            __builtin_memcpy(data.message, "Upper", 5);
        }
        data.message[5] = '\\0';

        bpf_trace_printk("Perf event: netns=%u, message=%s\\n", data.netns, data.message);

        events.perf_submit(ctx, &data, sizeof(data));

        u32 zero = 0;
        buffer_map.update(&netns, &zero);
    }

    // bpf_trace_printk("Receive state process: netns=%u, state=%u\\n", netns, state);
    return 0;
}
"""

# Initialize BPF
b = BPF(text=bpf_text)

constants_map = b["constants_map"]
lower_limit_key = ct.c_uint32(0)
upper_limit_key = ct.c_uint32(1)
buffer_length_key = ct.c_uint32(2)

# Set constants
constants_map[lower_limit_key] = ct.c_uint32(3)  # lowerLimit
constants_map[upper_limit_key] = ct.c_uint32(10)  # upperLimit
constants_map[buffer_length_key] = ct.c_uint32(5)  # bufferLength

# Populate the valid_netns_map with valid namespaces
valid_netns_map = b["valid_netns_map"]
conn_count_map = b["conn_count_map"]
for info in container_info:
    netns = ct.c_uint32(info['netns'])
    valid_netns_map[netns] = ct.c_uint32(1)  # Mark as valid
    conn_count_map[netns] = ct.c_uint32(0)   # Initialize to 0

print("Monitoring active connections... Hit Ctrl-C to end.")

# Poll the BPF map and print the connection counts
try:
    while True:
        time.sleep(1)
        print("\n%-10s %-10s" % ("NETNS", "Active Connections"))
        for k, v in b["conn_count_map"].items():
            print("%-10d %-10d" % (k.value, v.value))

except KeyboardInterrupt:
    print("Detaching...")
