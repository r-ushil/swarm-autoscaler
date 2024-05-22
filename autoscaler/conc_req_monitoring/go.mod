module conc_req_monitoring

require (
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/olekukonko/tablewriter v0.0.5 // indirect
	golang.org/x/exp v0.0.0-20230224173230-c95f2b4c22f2 // indirect
	golang.org/x/sys v0.18.0 // indirect
)

require server v0.0.0

replace server => ../server

require logging v0.0.0 // indirect

replace logging => ../logging

require github.com/cilium/ebpf v0.15.0

require scale v0.0.0

replace scale => ../scale

go 1.21.9
