#!/bin/sh

# Install clang, llvm, libbpf-devel and libelf-devel

go generate ./...
go build .
