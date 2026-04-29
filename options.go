package quicnet

import (
	"context"
	"fmt"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"

	"github.com/DiegoSandival/synap2p-go/internal/node"
)

const (
	defaultClientKeyPath        = "client_identity.key"
	defaultServerKeyPath        = "server_identity.key"
	defaultMDNSServiceName      = "_synap2p._udp"
	defaultProtocolPrefix       = "/synap2p"
	defaultDirectProtocol       = protocol.ID("/p2plib/direct/1.0")
	defaultUserAgent            = "synap2p-go/0.1"
	defaultDiscoveryInterval    = 30 * time.Second
	defaultProviderRefresh      = 15 * time.Minute
	defaultMaxPubSubMessageSize = 1 << 20
	defaultConnLowWater         = 64
	defaultConnHighWater        = 96
	defaultConnGracePeriod      = time.Minute
	defaultDialTimeout          = 15 * time.Second
	defaultClientListenAddrIPv4 = "/ip4/0.0.0.0/udp/0/quic-v1"
	defaultClientListenAddrIPv6 = "/ip6/::/udp/0/quic-v1"
	defaultServerListenAddrIPv4 = "/ip4/0.0.0.0/udp/4001/quic-v1"
	defaultServerListenAddrIPv6 = "/ip6/::/udp/4001/quic-v1"
	defaultTopicDiscoveryPrefix = "pubsub:"
)

type DirectMessageHandler func(ctx context.Context, from peer.ID, data []byte)

type Option interface {
	applyClient(*clientConfig) error
	applyServer(*serverConfig) error
}

type option struct {
	client func(*clientConfig) error
	server func(*serverConfig) error
}

func (o option) applyClient(cfg *clientConfig) error {
	if o.client == nil {
		return fmt.Errorf("option is not supported for ClientNode")
	}
	return o.client(cfg)
}

func (o option) applyServer(cfg *serverConfig) error {
	if o.server == nil {
		return fmt.Errorf("option is not supported for ServerNode")
	}
	return o.server(cfg)
}

type clientConfig struct {
	keyPath                  string
	listenAddrs              []string
	bootstrapPeers           []peer.AddrInfo
	staticRelays             []peer.AddrInfo
	mdnsServiceName          string
	protocolPrefix           protocol.ID
	directProtocol           protocol.ID
	directHandler            DirectMessageHandler
	eventHandler             EventHandler
	connLowWater             int
	connHighWater            int
	connGracePeriod          time.Duration
	dialTimeout              time.Duration
	userAgent                string
	discoveryInterval        time.Duration
	providerRefreshInterval  time.Duration
	maxPubSubMessageSize     int
	forceReachabilityPrivate bool
	forceReachabilityPublic  bool
	topicDiscoveryPrefix     string
}

type serverConfig struct {
	keyPath                 string
	listenAddrs             []string
	bootstrapPeers          []peer.AddrInfo
	protocolPrefix          protocol.ID
	connLowWater            int
	connHighWater           int
	connGracePeriod         time.Duration
	dialTimeout             time.Duration
	userAgent               string
	forceReachabilityPublic bool
	relayResources          relayv2.Resources
}

func defaultClientConfig() clientConfig {
	return clientConfig{
		keyPath:                  defaultClientKeyPath,
		listenAddrs:              []string{defaultClientListenAddrIPv4, defaultClientListenAddrIPv6},
		mdnsServiceName:          defaultMDNSServiceName,
		protocolPrefix:           protocol.ID(defaultProtocolPrefix),
		directProtocol:           defaultDirectProtocol,
		connLowWater:             defaultConnLowWater,
		connHighWater:            defaultConnHighWater,
		connGracePeriod:          defaultConnGracePeriod,
		dialTimeout:              defaultDialTimeout,
		userAgent:                defaultUserAgent,
		discoveryInterval:        defaultDiscoveryInterval,
		providerRefreshInterval:  defaultProviderRefresh,
		maxPubSubMessageSize:     defaultMaxPubSubMessageSize,
		topicDiscoveryPrefix:     defaultTopicDiscoveryPrefix,
		forceReachabilityPrivate: false,
	}
}

func defaultServerConfig() serverConfig {
	return serverConfig{
		keyPath:                 defaultServerKeyPath,
		listenAddrs:             []string{defaultServerListenAddrIPv4, defaultServerListenAddrIPv6},
		protocolPrefix:          protocol.ID(defaultProtocolPrefix),
		connLowWater:            defaultConnLowWater,
		connHighWater:           defaultConnHighWater,
		connGracePeriod:         defaultConnGracePeriod,
		dialTimeout:             defaultDialTimeout,
		userAgent:               defaultUserAgent,
		forceReachabilityPublic: true,
		relayResources:          relayv2.DefaultResources(),
	}
}

func WithKeyPath(path string) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.keyPath = path
			return nil
		},
		server: func(cfg *serverConfig) error {
			cfg.keyPath = path
			return nil
		},
	}
}

func WithListenAddrs(addrs ...string) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.listenAddrs = append([]string(nil), addrs...)
			return nil
		},
		server: func(cfg *serverConfig) error {
			cfg.listenAddrs = append([]string(nil), addrs...)
			return nil
		},
	}
}

func WithBootstrapPeers(peerAddrs ...string) Option {
	infos, err := node.ParseAddrInfos(peerAddrs)
	if err != nil {
		return option{
			client: func(*clientConfig) error { return err },
			server: func(*serverConfig) error { return err },
		}
	}

	return option{
		client: func(cfg *clientConfig) error {
			cfg.bootstrapPeers = append([]peer.AddrInfo(nil), infos...)
			return nil
		},
		server: func(cfg *serverConfig) error {
			cfg.bootstrapPeers = append([]peer.AddrInfo(nil), infos...)
			return nil
		},
	}
}

func WithStaticRelays(relayAddrs ...string) Option {
	infos, err := node.ParseAddrInfos(relayAddrs)
	if err != nil {
		return option{
			client: func(*clientConfig) error { return err },
		}
	}

	return option{
		client: func(cfg *clientConfig) error {
			cfg.staticRelays = append([]peer.AddrInfo(nil), infos...)
			return nil
		},
	}
}

type EventHandler func(event []byte)

func WithEventHandler(handler EventHandler) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.eventHandler = handler
			return nil
		},
	}
}

func WithLogger(logger LogHandler) Option {
	return option{
		client: func(cfg *clientConfig) error {
			globalLogHandler = logger
			return nil
		},
		server: func(cfg *serverConfig) error {
			globalLogHandler = logger
			return nil
		},
	}
}

func WithDirectMessageHandler(handler DirectMessageHandler) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.directHandler = handler
			return nil
		},
	}
}

func WithMDNSServiceName(serviceName string) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.mdnsServiceName = serviceName
			return nil
		},
	}
}

func WithProtocolPrefix(prefix string) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.protocolPrefix = protocol.ID(prefix)
			return nil
		},
		server: func(cfg *serverConfig) error {
			cfg.protocolPrefix = protocol.ID(prefix)
			return nil
		},
	}
}

func WithDirectProtocol(id string) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.directProtocol = protocol.ID(id)
			return nil
		},
	}
}

func WithConnectionManager(lowWater, highWater int, gracePeriod time.Duration) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.connLowWater = lowWater
			cfg.connHighWater = highWater
			cfg.connGracePeriod = gracePeriod
			return nil
		},
		server: func(cfg *serverConfig) error {
			cfg.connLowWater = lowWater
			cfg.connHighWater = highWater
			cfg.connGracePeriod = gracePeriod
			return nil
		},
	}
}

func WithDialTimeout(timeout time.Duration) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.dialTimeout = timeout
			return nil
		},
		server: func(cfg *serverConfig) error {
			cfg.dialTimeout = timeout
			return nil
		},
	}
}

func WithUserAgent(userAgent string) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.userAgent = userAgent
			return nil
		},
		server: func(cfg *serverConfig) error {
			cfg.userAgent = userAgent
			return nil
		},
	}
}

func WithDiscoveryInterval(interval time.Duration) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.discoveryInterval = interval
			return nil
		},
	}
}

func WithProviderRefreshInterval(interval time.Duration) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.providerRefreshInterval = interval
			return nil
		},
	}
}

func WithMaxPubSubMessageSize(maxSize int) Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.maxPubSubMessageSize = maxSize
			return nil
		},
	}
}

func WithReachabilityPrivate() Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.forceReachabilityPrivate = true
			cfg.forceReachabilityPublic = false
			return nil
		},
	}
}

func WithReachabilityPublic() Option {
	return option{
		client: func(cfg *clientConfig) error {
			cfg.forceReachabilityPublic = true
			cfg.forceReachabilityPrivate = false
			return nil
		},
		server: func(cfg *serverConfig) error {
			cfg.forceReachabilityPublic = true
			return nil
		},
	}
}

func WithRelayResources(resources relayv2.Resources) Option {
	return option{
		server: func(cfg *serverConfig) error {
			cfg.relayResources = resources
			return nil
		},
	}
}

func validateClientConfig(cfg clientConfig) error {
	if cfg.keyPath == "" {
		return fmt.Errorf("key path is required")
	}
	if len(cfg.listenAddrs) == 0 {
		return fmt.Errorf("at least one QUIC listen address is required")
	}
	if cfg.connLowWater < 0 || cfg.connHighWater < cfg.connLowWater {
		return fmt.Errorf("invalid connection manager thresholds")
	}
	if cfg.maxPubSubMessageSize <= 0 {
		return fmt.Errorf("max pubsub message size must be positive")
	}
	if cfg.protocolPrefix == "" {
		return fmt.Errorf("protocol prefix is required")
	}
	if cfg.discoveryInterval <= 0 {
		return fmt.Errorf("discovery interval must be positive")
	}
	if cfg.providerRefreshInterval <= 0 {
		return fmt.Errorf("provider refresh interval must be positive")
	}
	return nil
}

func validateServerConfig(cfg serverConfig) error {
	if cfg.keyPath == "" {
		return fmt.Errorf("key path is required")
	}
	if len(cfg.listenAddrs) == 0 {
		return fmt.Errorf("at least one QUIC listen address is required")
	}
	if cfg.connLowWater < 0 || cfg.connHighWater < cfg.connLowWater {
		return fmt.Errorf("invalid connection manager thresholds")
	}
	if cfg.protocolPrefix == "" {
		return fmt.Errorf("protocol prefix is required")
	}
	return nil
}

func defaultPubSubOptions(cfg clientConfig, discovery pubsub.DiscoverOpt) []pubsub.Option {
	params := pubsub.DefaultGossipSubParams()
	params.D = 8
	params.Dlo = 6
	params.Dhi = 12

	return []pubsub.Option{
		pubsub.WithGossipSubParams(params),
		pubsub.WithMessageSignaturePolicy(pubsub.StrictSign),
		pubsub.WithFloodPublish(true),
		pubsub.WithMaxMessageSize(cfg.maxPubSubMessageSize),
		pubsub.WithDiscovery(nil, discovery),
	}
}
