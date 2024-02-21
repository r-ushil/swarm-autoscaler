module userspace-collection

go 1.21.7

require (
	cgroup_net_listen v0.0.0
	github.com/docker/docker v25.0.3+incompatible
)

replace cgroup_net_listen => ../../cgroup_net_listen

require (
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/cilium/ebpf v0.13.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.48.0 // indirect
	go.opentelemetry.io/otel v1.23.1 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.23.1 // indirect
	go.opentelemetry.io/otel/metric v1.23.1 // indirect
	go.opentelemetry.io/otel/sdk v1.23.1 // indirect
	go.opentelemetry.io/otel/trace v1.23.1 // indirect
	golang.org/x/exp v0.0.0-20230224173230-c95f2b4c22f2 // indirect
	golang.org/x/sys v0.16.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	gotest.tools/v3 v3.5.1 // indirect
)
