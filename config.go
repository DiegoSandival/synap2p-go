package quicnet

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/peer"
	yaml "go.yaml.in/yaml/v2"
)

const (
	ModeClient = "client"
	ModeServer = "server"
	ModeHybrid = "hybrid"

	defaultHybridKeyPath = "hybrid_identity.key"
	defaultEventBuffer   = 128
	opcodeDirectMessage  = 0x0B
)

type Config struct {
	Mode                     string   `json:"mode" yaml:"mode"`
	ListenPort               int      `json:"listen_port" yaml:"listen_port"`
	ListenAddrs              []string `json:"listen_addrs,omitempty" yaml:"listen_addrs,omitempty"`
	BootstrapList            []string `json:"bootstrap_list,omitempty" yaml:"bootstrap_list,omitempty"`
	Namespace                string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	EnableRelay              bool     `json:"enable_relay,omitempty" yaml:"enable_relay,omitempty"`
	DataDir                  string   `json:"data_dir,omitempty" yaml:"data_dir,omitempty"`
	KeyPath                  string   `json:"key_path,omitempty" yaml:"key_path,omitempty"`
	AppID                    string   `json:"app_id,omitempty" yaml:"app_id,omitempty"`
	ProtocolPrefix           string   `json:"protocol_prefix,omitempty" yaml:"protocol_prefix,omitempty"`
	MDNSServiceName          string   `json:"mdns_service_name,omitempty" yaml:"mdns_service_name,omitempty"`
	UserAgent                string   `json:"user_agent,omitempty" yaml:"user_agent,omitempty"`
	StaticRelays             []string `json:"static_relays,omitempty" yaml:"static_relays,omitempty"`
	RelayAdvertiseInterval   string   `json:"relay_advertise_interval,omitempty" yaml:"relay_advertise_interval,omitempty"`
	DiscoveryInterval        string   `json:"discovery_interval,omitempty" yaml:"discovery_interval,omitempty"`
	ProviderRefreshInterval  string   `json:"provider_refresh_interval,omitempty" yaml:"provider_refresh_interval,omitempty"`
	EventBuffer              int      `json:"event_buffer,omitempty" yaml:"event_buffer,omitempty"`
	ForceReachabilityPrivate bool     `json:"force_reachability_private,omitempty" yaml:"force_reachability_private,omitempty"`
	ForceReachabilityPublic  bool     `json:"force_reachability_public,omitempty" yaml:"force_reachability_public,omitempty"`
}

type Node struct {
	mode      string
	engine    *Engine
	server    *ServerNode
	events    chan []byte
	startOnce sync.Once
	closeOnce sync.Once
}

type resolvedConfig struct {
	mode                     string
	listenAddrs              []string
	bootstrapList            []string
	namespace                string
	keyPath                  string
	protocolPrefix           string
	mdnsServiceName          string
	userAgent                string
	staticRelays             []string
	relayAdvertiseInterval   time.Duration
	discoveryInterval        time.Duration
	providerRefreshInterval  time.Duration
	eventBuffer              int
	forceReachabilityPrivate bool
	forceReachabilityPublic  bool
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		err = yaml.Unmarshal(data, &cfg)
	case ".json":
		err = json.Unmarshal(data, &cfg)
	default:
		if err = json.Unmarshal(data, &cfg); err != nil {
			err = yaml.Unmarshal(data, &cfg)
		}
	}
	if err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}
	return cfg, nil
}

func NewNode(cfg Config) (*Node, error) {
	resolved, err := cfg.resolve()
	if err != nil {
		return nil, err
	}

	node := &Node{
		mode:   resolved.mode,
		events: make(chan []byte, resolved.eventBuffer),
	}

	eventHandler := func(event []byte) {
		node.events <- append([]byte(nil), event...)
	}

	switch resolved.mode {
	case ModeServer:
		opts := resolved.serverOptions()
		server, err := NewServer(opts...)
		if err != nil {
			close(node.events)
			return nil, err
		}
		node.server = server
	case ModeClient, ModeHybrid:
		opts := resolved.clientOptions(eventHandler)
		engine, err := NewEngine(opts...)
		if err != nil {
			close(node.events)
			return nil, err
		}
		node.engine = engine
	default:
		close(node.events)
		return nil, fmt.Errorf("unsupported node mode %q", resolved.mode)
	}

	return node, nil
}

func (n *Node) Mode() string {
	if n == nil {
		return ""
	}
	return n.mode
}

func (n *Node) Start(ctx context.Context) error {
	if n == nil {
		return fmt.Errorf("node is nil")
	}
	if ctx == nil {
		return nil
	}

	n.startOnce.Do(func() {
		go func() {
			<-ctx.Done()
			_ = n.Close()
		}()
	})
	return nil
}

func (n *Node) Close() error {
	if n == nil {
		return nil
	}

	var closeErr error
	n.closeOnce.Do(func() {
		if n.engine != nil {
			closeErr = n.engine.Close()
		}
		if closeErr == nil && n.server != nil {
			closeErr = n.server.Close()
		}
		close(n.events)
	})
	return closeErr
}

func (n *Node) Events() <-chan []byte {
	if n == nil {
		return nil
	}
	return n.events
}

func (n *Node) Process(msg []byte) error {
	if n == nil {
		return fmt.Errorf("node is nil")
	}
	if n.engine == nil {
		return fmt.Errorf("node mode %q does not expose opcode processing", n.mode)
	}
	n.engine.Process(msg)
	return nil
}

func (n *Node) SendOpcode(opcode byte, requestID []byte, payload []byte) error {
	frame, err := BuildOpcodeFrame(opcode, requestID, payload)
	if err != nil {
		return err
	}
	return n.Process(frame)
}

func (n *Node) SendDirectOpcode(requestID []byte, peerID string, payload []byte) error {
	frame, err := BuildDirectOpcodeFrame(requestID, peerID, payload)
	if err != nil {
		return err
	}
	return n.Process(frame)
}

func (n *Node) SendDirectMessage(ctx context.Context, peerID string, payload []byte) error {
	if n == nil || n.engine == nil || n.engine.Client == nil {
		return fmt.Errorf("node mode %q does not expose direct messaging", n.Mode())
	}
	decodedPeerID, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("decode peer id %q: %w", peerID, err)
	}
	return n.engine.Client.SendDirectMessage(ctx, decodedPeerID, payload)
}

func (n *Node) ID() peer.ID {
	if n == nil {
		return ""
	}
	if n.engine != nil && n.engine.Client != nil {
		return n.engine.Client.ID()
	}
	if n.server != nil {
		return n.server.ID()
	}
	return ""
}

func (n *Node) Addrs() []string {
	if n == nil {
		return nil
	}
	if n.engine != nil && n.engine.Client != nil {
		return n.engine.Client.Addrs()
	}
	if n.server != nil {
		return n.server.Addrs()
	}
	return nil
}

func BuildOpcodeFrame(opcode byte, requestID []byte, payload []byte) ([]byte, error) {
	if len(requestID) != 16 {
		return nil, fmt.Errorf("request ID must be exactly 16 bytes")
	}

	frame := make([]byte, 20+len(payload))
	frame[3] = opcode
	copy(frame[4:20], requestID)
	copy(frame[20:], payload)
	return frame, nil
}

func BuildDirectOpcodeFrame(requestID []byte, peerID string, payload []byte) ([]byte, error) {
	if len(requestID) != 16 {
		return nil, fmt.Errorf("request ID must be exactly 16 bytes")
	}

	peerIDBytes := []byte(peerID)
	if len(peerIDBytes) > 0xFFFF {
		return nil, fmt.Errorf("peer ID is too long")
	}

	frame := make([]byte, 22+len(peerIDBytes)+len(payload))
	frame[3] = opcodeDirectMessage
	copy(frame[4:20], requestID)
	binary.BigEndian.PutUint16(frame[20:22], uint16(len(peerIDBytes)))
	copy(frame[22:22+len(peerIDBytes)], peerIDBytes)
	copy(frame[22+len(peerIDBytes):], payload)
	return frame, nil
}

func (cfg Config) resolve() (resolvedConfig, error) {
	mode, err := resolveMode(cfg.Mode, cfg.EnableRelay)
	if err != nil {
		return resolvedConfig{}, err
	}

	if cfg.ListenPort < 0 {
		return resolvedConfig{}, fmt.Errorf("listen_port must be zero or greater")
	}

	relayAdvertiseInterval, err := parseOptionalDuration(cfg.RelayAdvertiseInterval, defaultRelayAdvertiseIntvl)
	if err != nil {
		return resolvedConfig{}, fmt.Errorf("parse relay advertise interval: %w", err)
	}
	discoveryInterval, err := parseOptionalDuration(cfg.DiscoveryInterval, defaultDiscoveryInterval)
	if err != nil {
		return resolvedConfig{}, fmt.Errorf("parse discovery interval: %w", err)
	}
	providerRefreshInterval, err := parseOptionalDuration(cfg.ProviderRefreshInterval, defaultProviderRefresh)
	if err != nil {
		return resolvedConfig{}, fmt.Errorf("parse provider refresh interval: %w", err)
	}

	resolved := resolvedConfig{
		mode:                     mode,
		listenAddrs:              resolveListenAddrs(cfg.ListenAddrs, cfg.ListenPort),
		bootstrapList:            append([]string(nil), cfg.BootstrapList...),
		namespace:                resolveNamespace(cfg.Namespace, cfg.AppID),
		keyPath:                  resolveKeyPath(mode, cfg.DataDir, cfg.KeyPath),
		protocolPrefix:           resolveProtocolPrefix(cfg.ProtocolPrefix, cfg.AppID),
		mdnsServiceName:          resolveMDNSServiceName(cfg.MDNSServiceName, cfg.AppID),
		userAgent:                cfg.UserAgent,
		staticRelays:             append([]string(nil), cfg.StaticRelays...),
		relayAdvertiseInterval:   relayAdvertiseInterval,
		discoveryInterval:        discoveryInterval,
		providerRefreshInterval:  providerRefreshInterval,
		eventBuffer:              resolveEventBuffer(cfg.EventBuffer),
		forceReachabilityPrivate: cfg.ForceReachabilityPrivate,
		forceReachabilityPublic:  cfg.ForceReachabilityPublic,
	}

	return resolved, nil
}

func (cfg resolvedConfig) clientOptions(eventHandler EventHandler) []Option {
	opts := []Option{
		WithKeyPath(cfg.keyPath),
		WithListenAddrs(cfg.listenAddrs...),
		WithProtocolPrefix(cfg.protocolPrefix),
		WithRelayDiscoveryNamespace(cfg.namespace),
		WithDiscoveryInterval(cfg.discoveryInterval),
		WithProviderRefreshInterval(cfg.providerRefreshInterval),
	}

	if len(cfg.bootstrapList) > 0 {
		opts = append(opts, WithBootstrapPeers(cfg.bootstrapList...))
	}
	if len(cfg.staticRelays) > 0 {
		opts = append(opts, WithStaticRelays(cfg.staticRelays...))
	}
	if cfg.mdnsServiceName != "" {
		opts = append(opts, WithMDNSServiceName(cfg.mdnsServiceName))
	}
	if cfg.userAgent != "" {
		opts = append(opts, WithUserAgent(cfg.userAgent))
	}
	if eventHandler != nil {
		opts = append(opts, WithEventHandler(eventHandler))
	}
	if cfg.forceReachabilityPrivate {
		opts = append(opts, WithReachabilityPrivate())
	}
	if cfg.forceReachabilityPublic {
		opts = append(opts, WithReachabilityPublic())
	}
	if cfg.mode == ModeHybrid {
		opts = append(opts, option{client: func(clientCfg *clientConfig) error {
			clientCfg.enableRelayService = true
			clientCfg.enableNATService = true
			clientCfg.dhtMode = dht.ModeAuto
			clientCfg.relayAdvertiseInterval = cfg.relayAdvertiseInterval
			return nil
		}})
	}

	return opts
}

func (cfg resolvedConfig) serverOptions() []Option {
	opts := []Option{
		WithKeyPath(cfg.keyPath),
		WithListenAddrs(cfg.listenAddrs...),
		WithProtocolPrefix(cfg.protocolPrefix),
		WithRelayDiscoveryNamespace(cfg.namespace),
		WithRelayAdvertiseInterval(cfg.relayAdvertiseInterval),
	}

	if len(cfg.bootstrapList) > 0 {
		opts = append(opts, WithBootstrapPeers(cfg.bootstrapList...))
	}
	if cfg.userAgent != "" {
		opts = append(opts, WithUserAgent(cfg.userAgent))
	}
	if cfg.forceReachabilityPublic {
		opts = append(opts, WithReachabilityPublic())
	}

	return opts
}

func resolveMode(mode string, enableRelay bool) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		normalized = ModeClient
	}
	if normalized == ModeClient && enableRelay {
		normalized = ModeHybrid
	}

	switch normalized {
	case ModeClient, ModeServer, ModeHybrid:
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported mode %q", mode)
	}
}

func resolveListenAddrs(addrs []string, port int) []string {
	if len(addrs) > 0 {
		return append([]string(nil), addrs...)
	}
	return []string{
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", port),
		fmt.Sprintf("/ip6/::/udp/%d/quic-v1", port),
	}
}

func resolveNamespace(namespace, appID string) string {
	if trimmed := strings.TrimSpace(namespace); trimmed != "" {
		return trimmed
	}
	if slug := slugify(appID, '_'); slug != "" {
		return fmt.Sprintf("_%s_relays", slug)
	}
	return defaultRelayDiscoveryNS
}

func resolveProtocolPrefix(prefix, appID string) string {
	if trimmed := strings.TrimSpace(prefix); trimmed != "" {
		return trimmed
	}
	if slug := slugify(appID, '-'); slug != "" {
		return "/synap2p/" + slug
	}
	return defaultProtocolPrefix
}

func resolveMDNSServiceName(serviceName, appID string) string {
	if trimmed := strings.TrimSpace(serviceName); trimmed != "" {
		return trimmed
	}
	if slug := slugify(appID, '-'); slug != "" {
		return fmt.Sprintf("_%s._udp", slug)
	}
	return defaultMDNSServiceName
}

func resolveKeyPath(mode, dataDir, keyPath string) string {
	if keyPath == "" {
		keyPath = defaultKeyPathForMode(mode)
	}
	if dataDir == "" {
		return keyPath
	}
	if filepath.IsAbs(keyPath) {
		return keyPath
	}
	return filepath.Join(dataDir, keyPath)
}

func defaultKeyPathForMode(mode string) string {
	switch mode {
	case ModeServer:
		return defaultServerKeyPath
	case ModeHybrid:
		return defaultHybridKeyPath
	default:
		return defaultClientKeyPath
	}
}

func parseOptionalDuration(value string, fallback time.Duration) (time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return time.ParseDuration(value)
}

func resolveEventBuffer(size int) int {
	if size > 0 {
		return size
	}
	return defaultEventBuffer
}

func slugify(value string, separator rune) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	lastWasSeparator := false
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastWasSeparator = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastWasSeparator = false
		default:
			if !lastWasSeparator && builder.Len() > 0 {
				builder.WriteRune(separator)
				lastWasSeparator = true
			}
		}
	}

	result := builder.String()
	result = strings.Trim(result, string(separator))
	return result
}
