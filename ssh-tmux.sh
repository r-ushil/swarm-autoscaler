#!/bin/bash

# Start tmux session
tmux new-session -d

# Number of horizontal windows
HORIZONTAL_WINDOWS=4

# Split horizontally
for ((i=0; i<($HORIZONTAL_WINDOWS-1); i++)); do
    tmux split-window -h
done

# Arrange panes into 4 rectangles at the top, then 4 rectangles at the bottom
tmux select-layout even-horizontal

# Select panes and split vertically
for ((i=0; i<$HORIZONTAL_WINDOWS; i++)); do
    tmux select-pane -t $((i*2))
    tmux split-window -v
done

# Define SSH commands for accessing swarm-vm machines
swarm_commands=(
    "ssh -tt shell ssh -tt octopus2 ssh -tt swarm-vm1"
    "ssh -tt shell ssh -tt octopus2 ssh -tt swarm-vm2"
    "ssh -tt shell ssh -tt octopus2 ssh -tt swarm-vm3"
)

for i in {0..2}; do
    tmux send-keys -t $((2*i)) "${swarm_commands[$i]}" C-m
    tmux send-keys -t $((2*i+1)) "${swarm_commands[$i]}" C-m
done

tmux send-keys -t 6 'ssh -tt shell ssh -tt octopus2' C-m
tmux send-keys -t 7 'ssh -tt shell ssh -tt octopus2 pct enter 100' C-m

# Clear the terminal in each pane
for i in {0..7}; do
    tmux send-keys -t $i 'clear' C-m
done


# Attach to tmux session
tmux select-pane -t 0
tmux attach-session

