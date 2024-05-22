module autoscaler

go 1.21.9

require logging v0.0.0

require (
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/cilium/ebpf v0.15.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker v26.0.1+incompatible // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.19.1 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	github.com/vishvananda/netlink v1.1.0 // indirect
	github.com/vishvananda/netns v0.0.0-20191106174202-0a2b9b5464df // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.50.0 // indirect
	go.opentelemetry.io/otel v1.27.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.27.0 // indirect
	go.opentelemetry.io/otel/metric v1.27.0 // indirect
	go.opentelemetry.io/otel/sdk v1.27.0 // indirect
	go.opentelemetry.io/otel/trace v1.27.0 // indirect
	golang.org/x/exp v0.0.0-20230224173230-c95f2b4c22f2 // indirect
	golang.org/x/sys v0.20.0 // indirect
	google.golang.org/grpc v1.64.0 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

replace logging => ../logging

require scale v0.0.0

require bpf_port_listen v0.0.0

require (
	gopkg.in/yaml.v2 v2.4.0
	server v0.0.0
)

replace scale => ../scale

replace bpf_port_listen => ../bpf_port_listen

replace server => ../server

require cgroup_monitoring v0.0.0

replace cgroup_monitoring => ../cgroup_monitoring

require conc_req_monitoring v0.0.0

replace conc_req_monitoring => ../conc_req_monitoring
