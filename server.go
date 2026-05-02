package quicnet

import (
	"context"
	"fmt"
	"sync"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	routingdiscovery "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	discoveryutil "github.com/libp2p/go-libp2p/p2p/discovery/util"

	"github.com/DiegoSandival/synap2p-go/internal/identity"
	"github.com/DiegoSandival/synap2p-go/internal/node"
)

type ServerNode struct {
	host             host.Host
	dht              *dht.IpfsDHT
	routingDiscovery *routingdiscovery.RoutingDiscovery
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	closed           bool
}

func NewServer(opts ...Option) (*ServerNode, error) {
	cfg := defaultServerConfig()
	for _, opt := range opts {
		if err := opt.applyServer(&cfg); err != nil {
			return nil, err
		}
	}
	if err := validateServerConfig(cfg); err != nil {
		return nil, err
	}

	privKey, err := identity.LoadOrGenerateKey(cfg.keyPath)
	if err != nil {
		return nil, fmt.Errorf("load server identity: %w", err)
	}

	h, err := node.NewHost(node.HostConfig{
		PrivKey:                 privKey,
		ListenAddrs:             cfg.listenAddrs,
		ConnLowWater:            cfg.connLowWater,
		ConnHighWater:           cfg.connHighWater,
		ConnGracePeriod:         cfg.connGracePeriod,
		DialTimeout:             cfg.dialTimeout,
		UserAgent:               cfg.userAgent,
		EnableRelay:             true,
		EnableRelayService:      true,
		EnableNATService:        true,
		ForceReachabilityPublic: cfg.forceReachabilityPublic,
		RelayResources:          cfg.relayResources,
	})
	if err != nil {
		return nil, fmt.Errorf("build QUIC-only server host: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	dhtOptions := []dht.Option{
		dht.Mode(dht.ModeServer),
		dht.ProtocolPrefix(cfg.protocolPrefix),
	}
	if len(cfg.bootstrapPeers) > 0 {
		dhtOptions = append(dhtOptions, dht.BootstrapPeers(cfg.bootstrapPeers...))
	}

	routing, err := dht.New(ctx, h, dhtOptions...)
	if err != nil {
		cancel()
		_ = h.Close()
		return nil, fmt.Errorf("create server DHT: %w", err)
	}
	if err := routing.Bootstrap(ctx); err != nil {
		cancel()
		_ = routing.Close()
		_ = h.Close()
		return nil, fmt.Errorf("bootstrap server DHT: %w", err)
	}
	routingDiscovery := routingdiscovery.NewRoutingDiscovery(routing)

	server := &ServerNode{
		host:             h,
		dht:              routing,
		routingDiscovery: routingDiscovery,
		ctx:              ctx,
		cancel:           cancel,
	}
	server.bootstrapKnownPeers(cfg.bootstrapPeers, cfg.dialTimeout)
	server.startRelayAdvertisement(cfg.relayDiscoveryNamespace, cfg.relayAdvertiseInterval)
	return server, nil
}

func (s *ServerNode) ID() peer.ID {
	return s.host.ID()
}

func (s *ServerNode) Addrs() []string {
	addrs := s.host.Addrs()
	result := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		result = append(result, addr.String())
	}
	return result
}

func (s *ServerNode) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	s.cancel()
	s.wg.Wait()
	if s.dht != nil {
		_ = s.dht.Close()
	}
	if s.host != nil {
		_ = s.host.Close()
	}
	return nil
}

func (s *ServerNode) startRelayAdvertisement(namespace string, interval time.Duration) {
	if s.routingDiscovery == nil || namespace == "" {
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		advertise := func() {
			discoveryutil.Advertise(s.ctx, s.routingDiscovery, namespace)
		}

		advertise()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				advertise()
			}
		}
	}()
}

func (s *ServerNode) bootstrapKnownPeers(peers []peer.AddrInfo, timeout time.Duration) {
	for _, info := range peers {
		if info.ID == "" || info.ID == s.host.ID() {
			continue
		}
		ctx, cancel := context.WithTimeout(s.ctx, timeout)
		_ = s.host.Connect(ctx, info)
		cancel()
	}
}
