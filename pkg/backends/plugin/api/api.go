package api

import (
	"net/rpc"

	"github.com/hashicorp/go-plugin"
)

// Handshake is a common handshake that is shared by plugin and host.
var Handshake = plugin.HandshakeConfig{
	// This isn't required when using VersionedPlugins
	ProtocolVersion:  1,
	MagicCookieKey:   "CONFD_PLUGIN",
	MagicCookieValue: "hello_confd",
}

// -------------------------------------------------------------------------
// 1. DTOs (Data Transfer Objects for net/rpc)
// -------------------------------------------------------------------------

type GetValuesArgs struct {
	Keys []string
}
type GetValuesReply struct {
	Values map[string]string
}

type WatchPrefixArgs struct {
	Prefix    string
	Keys      []string
	WaitIndex uint64
}
type WatchPrefixReply struct {
	WaitIndex uint64
}

// -------------------------------------------------------------------------
// 2. The Plugin Interface
// -------------------------------------------------------------------------

// BackendProvider is the interface that we're exposing as a plugin.
type BackendProvider interface {
	GetValues(keys []string) (map[string]string, error)
	WatchPrefix(prefix string, keys []string, waitIndex uint64) (uint64, error)
	HealthCheck() error
	Close() error
}

// -------------------------------------------------------------------------
// 3. RPC Client (Used by Confd to talk to the Plugin)
// -------------------------------------------------------------------------

type BackendRPCClient struct {
	client *rpc.Client
}

func (g *BackendRPCClient) GetValues(keys []string) (map[string]string, error) {
	var resp GetValuesReply
	err := g.client.Call("Plugin.GetValues", GetValuesArgs{Keys: keys}, &resp)
	return resp.Values, err
}

func (g *BackendRPCClient) WatchPrefix(prefix string, keys []string, waitIndex uint64) (uint64, error) {
	var resp WatchPrefixReply
	err := g.client.Call("Plugin.WatchPrefix", WatchPrefixArgs{Prefix: prefix, Keys: keys, WaitIndex: waitIndex}, &resp)
	return resp.WaitIndex, err
}

func (g *BackendRPCClient) HealthCheck() error {
	var resp struct{}
	err := g.client.Call("Plugin.HealthCheck", new(interface{}), &resp)
	return err
}

func (g *BackendRPCClient) Close() error {
	var resp struct{}
	err := g.client.Call("Plugin.Close", new(interface{}), &resp)
	return err
}

// -------------------------------------------------------------------------
// 4. RPC Server (Used by the Plugin to receive calls from Confd)
// -------------------------------------------------------------------------

type BackendRPCServer struct {
	Impl BackendProvider
}

func (s *BackendRPCServer) GetValues(args GetValuesArgs, resp *GetValuesReply) error {
	values, err := s.Impl.GetValues(args.Keys)
	resp.Values = values
	return err
}

func (s *BackendRPCServer) WatchPrefix(args WatchPrefixArgs, resp *WatchPrefixReply) error {
	index, err := s.Impl.WatchPrefix(args.Prefix, args.Keys, args.WaitIndex)
	resp.WaitIndex = index
	return err
}

func (s *BackendRPCServer) HealthCheck(args interface{}, resp *struct{}) error {
	return s.Impl.HealthCheck()
}

func (s *BackendRPCServer) Close(args interface{}, resp *struct{}) error {
	return s.Impl.Close()
}

// -------------------------------------------------------------------------
// 5. The Plugin Boilerplate (Ties Client & Server together)
// -------------------------------------------------------------------------

// ConfdBackendPlugin is the implementation of plugin.Plugin so we can serve/consume this
type ConfdBackendPlugin struct {
	// Impl is only set when creating the Server plugin
	Impl BackendProvider
}

func (p *ConfdBackendPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &BackendRPCServer{Impl: p.Impl}, nil
}

func (p *ConfdBackendPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &BackendRPCClient{client: c}, nil
}
