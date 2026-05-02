package node

import (
	"fmt"
	"time"

	libp2p "github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	autorelay "github.com/libp2p/go-libp2p/p2p/host/autorelay"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	connmgr "github.com/libp2p/go-libp2p/p2p/net/connmgr"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
)

type HostConfig struct {
	PrivKey                  crypto.PrivKey
	ListenAddrs              []string
	ConnLowWater             int
	ConnHighWater            int
	ConnGracePeriod          time.Duration
	DialTimeout              time.Duration
	UserAgent                string
	EnableRelay              bool
	EnableRelayService       bool
	EnableNATService         bool
	EnableHolePunching       bool
	AutoRelayPeerSource      autorelay.PeerSource
	StaticRelays             []peer.AddrInfo
	ForceReachabilityPrivate bool
	ForceReachabilityPublic  bool
	RelayResources           relayv2.Resources
}

func NewHost(cfg HostConfig) (host.Host, error) {
	connManager, err := connmgr.NewConnManager(
		cfg.ConnLowWater,
		cfg.ConnHighWater,
		connmgr.WithGracePeriod(cfg.ConnGracePeriod),
	)
	if err != nil {
		return nil, fmt.Errorf("create connection manager: %w", err)
	}

	limits := rcmgr.DefaultLimits
	libp2p.SetDefaultServiceLimits(&limits)
	resourceManager, err := rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(limits.AutoScale()))
	if err != nil {
		return nil, fmt.Errorf("create resource manager: %w", err)
	}

	opts := []libp2p.Option{
		libp2p.Identity(cfg.PrivKey),
		libp2p.NoTransports,
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.ConnectionManager(connManager),
		libp2p.ResourceManager(resourceManager),
		libp2p.WithDialTimeout(cfg.DialTimeout),
		libp2p.UserAgent(cfg.UserAgent),
	}

	if len(cfg.ListenAddrs) > 0 {
		opts = append(opts, libp2p.ListenAddrStrings(cfg.ListenAddrs...))
	} else {
		opts = append(opts, libp2p.NoListenAddrs)
	}
	if cfg.EnableRelay {
		opts = append(opts, libp2p.EnableRelay())
	}
	if cfg.EnableRelayService {
		opts = append(opts, libp2p.EnableRelayService(relayv2.WithResources(cfg.RelayResources)))
	}
	if cfg.EnableNATService {
		opts = append(opts, libp2p.EnableNATService())
	}
	if cfg.EnableHolePunching {
		opts = append(opts, libp2p.EnableHolePunching())
	}
	if cfg.AutoRelayPeerSource != nil {
		opts = append(opts, libp2p.EnableAutoRelayWithPeerSource(cfg.AutoRelayPeerSource))
	} else if len(cfg.StaticRelays) > 0 {
		opts = append(opts, libp2p.EnableAutoRelayWithStaticRelays(cfg.StaticRelays))
	}
	if cfg.ForceReachabilityPrivate {
		opts = append(opts, libp2p.ForceReachabilityPrivate())
	}
	if cfg.ForceReachabilityPublic {
		opts = append(opts, libp2p.ForceReachabilityPublic())
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("create libp2p host: %w", err)
	}
	return h, nil
}
