#!/bin/bash

# Calculate total lines of code for all .c files
c_files=$(find . -type f -name "*.c")
total_c_lines=0
for file in $c_files; do
    lines=$(wc -l < "$file")
    total_c_lines=$((total_c_lines + lines))
done

# Calculate total lines of code for all .go files excluding those starting with bpf_
go_files=$(find . -type f -name "*.go" ! -name "bpf_*.go")
total_go_lines=0
for file in $go_files; do
    lines=$(wc -l < "$file")
    total_go_lines=$((total_go_lines + lines))
done

# Output the results
echo "Total lines of code in .c files: $total_c_lines"
echo "Total lines of code in .go files (excluding bpf_*.go): $total_go_lines"

