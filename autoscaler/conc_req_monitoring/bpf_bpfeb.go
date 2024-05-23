// Code generated by bpf2go; DO NOT EDIT.
//go:build mips || mips64 || ppc64 || s390x

package conc_req_monitoring

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

// LoadBPF returns the embedded CollectionSpec for BPF.
func LoadBPF() (*ebpf.CollectionSpec, error) {
	reader := bytes.NewReader(_BPFBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("can't load BPF: %w", err)
	}

	return spec, err
}

// LoadBPFObjects loads BPF and converts it into a struct.
//
// The following types are suitable as obj argument:
//
//	*BPFObjects
//	*BPFPrograms
//	*BPFMaps
//
// See ebpf.CollectionSpec.LoadAndAssign documentation for details.
func LoadBPFObjects(obj interface{}, opts *ebpf.CollectionOptions) error {
	spec, err := LoadBPF()
	if err != nil {
		return err
	}

	return spec.LoadAndAssign(obj, opts)
}

// BPFSpecs contains maps and programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type BPFSpecs struct {
	BPFProgramSpecs
	BPFMapSpecs
}

// BPFSpecs contains programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type BPFProgramSpecs struct {
	TraceInetSockSetState *ebpf.ProgramSpec `ebpf:"trace_inet_sock_set_state"`
}

// BPFMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type BPFMapSpecs struct {
	ActiveConnectionsMap *ebpf.MapSpec `ebpf:"active_connections_map"`
	BufferMap            *ebpf.MapSpec `ebpf:"buffer_map"`
	ConstantsMap         *ebpf.MapSpec `ebpf:"constants_map"`
	Events               *ebpf.MapSpec `ebpf:"events"`
	ScalingMap           *ebpf.MapSpec `ebpf:"scaling_map"`
}

// BPFObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to LoadBPFObjects or ebpf.CollectionSpec.LoadAndAssign.
type BPFObjects struct {
	BPFPrograms
	BPFMaps
}

func (o *BPFObjects) Close() error {
	return _BPFClose(
		&o.BPFPrograms,
		&o.BPFMaps,
	)
}

// BPFMaps contains all maps after they have been loaded into the kernel.
//
// It can be passed to LoadBPFObjects or ebpf.CollectionSpec.LoadAndAssign.
type BPFMaps struct {
	ActiveConnectionsMap *ebpf.Map `ebpf:"active_connections_map"`
	BufferMap            *ebpf.Map `ebpf:"buffer_map"`
	ConstantsMap         *ebpf.Map `ebpf:"constants_map"`
	Events               *ebpf.Map `ebpf:"events"`
	ScalingMap           *ebpf.Map `ebpf:"scaling_map"`
}

func (m *BPFMaps) Close() error {
	return _BPFClose(
		m.ActiveConnectionsMap,
		m.BufferMap,
		m.ConstantsMap,
		m.Events,
		m.ScalingMap,
	)
}

// BPFPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to LoadBPFObjects or ebpf.CollectionSpec.LoadAndAssign.
type BPFPrograms struct {
	TraceInetSockSetState *ebpf.Program `ebpf:"trace_inet_sock_set_state"`
}

func (p *BPFPrograms) Close() error {
	return _BPFClose(
		p.TraceInetSockSetState,
	)
}

func _BPFClose(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Do not access this directly.
//
//go:embed bpf_bpfeb.o
var _BPFBytes []byte
