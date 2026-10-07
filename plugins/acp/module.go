// Package acp is the restricted-pilot ACP Face Provider (issue #1): one
// local ACP stdio client bridged to the Vivy FaceHost Control RPC. The
// module performs no I/O during construction; all wire work happens inside
// boundFace.Run on the streams the launcher owns.
package acp

import (
	"context"
	"errors"
	"io"
	"net"

	"agent-vivy/sdk/module"
	faceport "agent-vivy/sdk/port/face"
)

const (
	moduleID   = "projectvivy/acp"
	moduleVer  = "0.1.0"
	providerID = "projectvivy.acp"
	faceKind   = "acp"
	sourceRef  = "repo:plugins/acp"
	sourceSHA  = "9c576deacacca8775b8f7fe32526a8bfb587f98df83935ed08f55f6b459b5eb2"
)

type vivyModule struct{}

func New() module.Module { return vivyModule{} }
func (vivyModule) Construct(context.Context, module.Host) (module.Instance, error) {
	return moduleInstance{}, nil
}
func (vivyModule) Descriptor() module.Descriptor {
	return module.Descriptor{
		APIVersion: module.APIVersionV1,
		Module:     module.Identity{ID: moduleID, Version: moduleVer},
		Source:     module.Source{Ref: sourceRef, SHA256: sourceSHA},
		Provides:   []module.PortRef{{Port: "std/face@v1", ID: providerID}},
		Requires: []module.Requirement{{
			PortRef:  module.PortRef{Port: "core/face-host@v1"},
			Provider: "vivy/face-host",
		}},
		RequestedGrants: []module.Grant{module.GrantRPCClient},
		Lifecycle:       module.Lifecycle{Scope: module.ScopeGeneration},
	}
}

type moduleInstance struct{}

func (moduleInstance) Start(context.Context) error { return nil }
func (moduleInstance) Ready(context.Context) error { return nil }
func (moduleInstance) Stop(context.Context) error  { return nil }
func (moduleInstance) Close(context.Context) error { return nil }

type provider struct{}

func NewProvider() faceport.FaceProvider { return provider{} }
func (provider) Definition() faceport.Definition {
	return faceport.Definition{ID: providerID, Kind: faceKind}
}
func (provider) Construct(_ context.Context, h faceport.Host) (faceport.Instance, error) {
	if h == nil {
		return nil, errors.New("acp: face host is required")
	}
	return &boundFace{host: h}, nil
}

// boundFace is the constructed Face instance. Run owns the session for the
// lifetime of one ACP client connection.
type boundFace struct {
	host faceport.Host
}

func (f *boundFace) Run(ctx context.Context, o faceport.Options) (faceport.Result, error) {
	if o.In == nil || o.Out == nil || o.Err == nil {
		return faceport.Result{Status: "failed"},
			errors.New("acp: requires input, output, and error streams")
	}
	a := newAgent(f.host)
	// The event route must exist before any run subscription (spec §5).
	f.host.OnEvent(a.onEvent)
	conn, err := newConnection(a, o.In, o.Out)
	if err != nil {
		return faceport.Result{Status: "failed"}, err
	}
	a.bindConnection(conn)
	if err := conn.Start(ctx); err != nil {
		_ = conn.Close()
		return faceport.Result{Status: "failed"}, err
	}
	select {
	case <-ctx.Done():
	case <-conn.Done():
	}
	_ = conn.Close()
	// A clean client disconnect surfaces as io.EOF (or a closed-pipe peer)
	// on the transport: that is a normal Face exit, not a failure.
	err = conn.Err()
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		err = nil
	}
	return faceport.Result{Status: "completed"}, err
}
