package quicnet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-cid"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	mdns "github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	routingdiscovery "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	discoveryutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	relayclient "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/DiegoSandival/synap2p-go/internal/identity"
	"github.com/DiegoSandival/synap2p-go/internal/node"
	"github.com/DiegoSandival/synap2p-go/protocol"
)

type ClientNode struct {
	host             host.Host
	dht              *dht.IpfsDHT
	pubsub           *pubsub.PubSub
	routingDiscovery *routingdiscovery.RoutingDiscovery
	mdnsService      mdns.Service
	cfg              clientConfig

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu                sync.RWMutex
	topics            map[string]*clientTopic
	announcements     map[string]context.CancelFunc
	relayReservations map[peer.ID]relayReservation
	closed            bool
}

type relayReservation struct {
	reservation *relayclient.Reservation
	addrs       []ma.Multiaddr
}

type clientTopic struct {
	topic           *pubsub.Topic
	subscription    *pubsub.Subscription
	subCancel       context.CancelFunc
	discoveryCancel context.CancelFunc
}

type mdnsNotifee struct {
	handler func(peer.AddrInfo)
}

func (n mdnsNotifee) HandlePeerFound(info peer.AddrInfo) {
	if n.handler != nil {
		n.handler(info)
	}
}

func NewClient(opts ...Option) (*ClientNode, error) {
	cfg := defaultClientConfig()
	for _, opt := range opts {
		if err := opt.applyClient(&cfg); err != nil {
			return nil, err
		}
	}
	if err := validateClientConfig(cfg); err != nil {
		return nil, err
	}

	privKey, err := identity.LoadOrGenerateKey(cfg.keyPath)
	if err != nil {
		return nil, fmt.Errorf("load client identity: %w", err)
	}

	h, err := node.NewHost(node.HostConfig{
		PrivKey:                  privKey,
		ListenAddrs:              cfg.listenAddrs,
		ConnLowWater:             cfg.connLowWater,
		ConnHighWater:            cfg.connHighWater,
		ConnGracePeriod:          cfg.connGracePeriod,
		DialTimeout:              cfg.dialTimeout,
		UserAgent:                cfg.userAgent,
		EnableRelay:              true,
		EnableHolePunching:       true,
		StaticRelays:             cfg.staticRelays,
		ForceReachabilityPrivate: cfg.forceReachabilityPrivate,
		ForceReachabilityPublic:  cfg.forceReachabilityPublic,
	})
	if err != nil {
		return nil, fmt.Errorf("build QUIC-only client host: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	dhtOptions := []dht.Option{
		dht.Mode(dht.ModeClient),
		dht.ProtocolPrefix(cfg.protocolPrefix),
	}
	if len(cfg.bootstrapPeers) > 0 {
		dhtOptions = append(dhtOptions, dht.BootstrapPeers(cfg.bootstrapPeers...))
	}

	routing, err := dht.New(ctx, h, dhtOptions...)
	if err != nil {
		cancel()
		_ = h.Close()
		return nil, fmt.Errorf("create client DHT: %w", err)
	}

	if err := routing.Bootstrap(ctx); err != nil {
		cancel()
		_ = routing.Close()
		_ = h.Close()
		return nil, fmt.Errorf("bootstrap client DHT: %w", err)
	}

	routingDiscovery := routingdiscovery.NewRoutingDiscovery(routing)
	ps, err := pubsub.NewGossipSub(
		ctx,
		h,
		pubsub.WithDiscovery(routingDiscovery),
		pubsub.WithMessageSignaturePolicy(pubsub.StrictSign),
		pubsub.WithFloodPublish(true),
		pubsub.WithMaxMessageSize(cfg.maxPubSubMessageSize),
	)
	if err != nil {
		cancel()
		_ = routing.Close()
		_ = h.Close()
		return nil, fmt.Errorf("create GossipSub: %w", err)
	}

	client := &ClientNode{
		host:              h,
		dht:               routing,
		pubsub:            ps,
		routingDiscovery:  routingDiscovery,
		cfg:               cfg,
		ctx:               ctx,
		cancel:            cancel,
		topics:            make(map[string]*clientTopic),
		announcements:     make(map[string]context.CancelFunc),
		relayReservations: make(map[peer.ID]relayReservation),
	}

	h.SetStreamHandler(cfg.directProtocol, client.handleDirectStream)
	client.bootstrapKnownPeers(append(append([]peer.AddrInfo(nil), cfg.bootstrapPeers...), cfg.staticRelays...))

	service := mdns.NewMdnsService(h, cfg.mdnsServiceName, mdnsNotifee{handler: client.handleMDNSPeer})
	if err := service.Start(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("start mDNS service: %w", err)
	}
	client.mdnsService = service

	return client, nil
}

func (c *ClientNode) ID() peer.ID {
	return c.host.ID()
}

func (c *ClientNode) Addrs() []string {
	addrs := c.host.Addrs()

	c.mu.RLock()
	relayAddrs := make([]ma.Multiaddr, 0, len(c.relayReservations))
	for _, reservation := range c.relayReservations {
		relayAddrs = append(relayAddrs, reservation.addrs...)
	}
	c.mu.RUnlock()

	result := make([]string, 0, len(addrs)+len(relayAddrs))
	seen := make(map[string]struct{}, len(addrs)+len(relayAddrs))
	for _, addr := range addrs {
		key := addr.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	for _, addr := range relayAddrs {
		key := addr.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

func (c *ClientNode) Subscribe(topicName string, handler func(msg []byte)) error {
	if handler == nil {
		return fmt.Errorf("subscribe handler is required")
	}

	state, err := c.ensureTopic(topicName)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if state.subscription != nil {
		return fmt.Errorf("topic %q is already subscribed", topicName)
	}

	sub, err := state.topic.Subscribe(pubsub.WithBufferSize(128))
	if err != nil {
		return fmt.Errorf("subscribe to topic %q: %w", topicName, err)
	}

	ctx, cancel := context.WithCancel(c.ctx)
	state.subscription = sub
	state.subCancel = cancel

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				if ctx.Err() != nil || errors.Is(err, context.Canceled) {
					return
				}
				continue
			}

			data := append([]byte(nil), msg.Data...)

			if c.cfg.eventHandler != nil {
				// Parseamos para obtener el ID original (16 bytes desde offset 4 a 20)
				// Si no tiene esta estructura, mandamos un ID de ceros
				id := make([]byte, 16)
				if len(data) >= 20 {
					copy(id, data[4:20])
				}

				parser := &protocol.ProtocolParser{}
				evtMsg := parser.FormatEventPubSub(id, data)
				c.wg.Add(1)
				go func(payload []byte) {
					defer c.wg.Done()
					c.cfg.eventHandler(payload)
				}(evtMsg)
			}

			c.wg.Add(1)
			go func(payload []byte) {
				defer c.wg.Done()
				handler(payload)
			}(data)
		}
	}()

	return nil
}

func (c *ClientNode) Unsubscribe(topicName string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	state, ok := c.topics[topicName]
	if !ok {
		return fmt.Errorf("topic %q is not tracked or subscribed", topicName)
	}

	if state.subCancel != nil {
		state.subCancel()
		state.subCancel = nil
	}
	if state.subscription != nil {
		state.subscription.Cancel()
		state.subscription = nil
	}
	if state.discoveryCancel != nil {
		state.discoveryCancel()
		state.discoveryCancel = nil
	}
	if state.topic != nil {
		state.topic.Close()
	}

	delete(c.topics, topicName)
	return nil
}

func (c *ClientNode) Publish(ctx context.Context, topicName string, data []byte) error {
	state, err := c.ensureTopic(topicName)
	if err != nil {
		return err
	}

	return state.topic.Publish(ctx, data, pubsub.WithReadiness(pubsub.MinTopicSize(1)))
}

func (c *ClientNode) AnnounceData(ctx context.Context, cidString string) error {
	contentID, err := cid.Parse(cidString)
	if err != nil {
		return fmt.Errorf("parse CID %q: %w", cidString, err)
	}

	if err := c.dht.Provide(ctx, contentID, true); err != nil {
		return fmt.Errorf("announce provider for %q: %w", cidString, err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.announcements[cidString]; exists {
		return nil
	}

	announceCtx, cancel := context.WithCancel(c.ctx)
	c.announcements[cidString] = cancel
	c.wg.Add(1)
	go c.refreshProvideLoop(announceCtx, cidString, contentID)
	return nil
}

func (c *ClientNode) UnannounceData(cidString string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cancel, ok := c.announcements[cidString]
	if !ok {
		return fmt.Errorf("CID %q is not currently announced", cidString)
	}
	delete(c.announcements, cidString)
	cancel()
	return nil
}

func (c *ClientNode) FindDataProviders(ctx context.Context, cidString string) ([]peer.AddrInfo, error) {
	contentID, err := cid.Parse(cidString)
	if err != nil {
		return nil, fmt.Errorf("parse CID %q: %w", cidString, err)
	}

	providers, err := c.dht.FindProviders(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("find providers for %q: %w", cidString, err)
	}
	return providers, nil
}

func (c *ClientNode) ConnectToRelay(ctx context.Context, relayMultiaddr string) error {
	info, err := node.ParseAddrInfo(relayMultiaddr)
	if err != nil {
		return fmt.Errorf("parse relay address: %w", err)
	}
	debugf("connect relay requested: relay=%s addr=%s", info.ID, relayMultiaddr)
	if err := c.connectAddrInfo(ctx, info); err != nil {
		debugf("connect relay failed: relay=%s err=%v", info.ID, err)
		return err
	}

	reservation, err := relayclient.Reserve(ctx, c.host, info)
	if err != nil {
		debugf("relay reservation failed: relay=%s err=%v", info.ID, err)
		return fmt.Errorf("reserve relay slot with %s: %w", info.ID, err)
	}

	c.mu.Lock()
	c.relayReservations[info.ID] = relayReservation{
		reservation: reservation,
		addrs:       relayCircuitAddrs(info),
	}
	c.mu.Unlock()
	debugf("relay reservation ok: relay=%s", info.ID)
	return nil
}

func relayCircuitAddrs(info peer.AddrInfo) []ma.Multiaddr {
	if info.ID == "" || len(info.Addrs) == 0 {
		return nil
	}

	relaySuffix, err := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s/p2p-circuit", info.ID))
	if err != nil {
		return nil
	}

	result := make([]ma.Multiaddr, 0, len(info.Addrs))
	seen := make(map[string]struct{}, len(info.Addrs))
	for _, addr := range info.Addrs {
		if addr == nil {
			continue
		}
		circuitAddr := addr.Encapsulate(relaySuffix)
		key := circuitAddr.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, circuitAddr)
	}
	return result
}

func (c *ClientNode) ConnectToPeer(ctx context.Context, peerAddr string) error {
	info, err := node.ParseAddrInfo(peerAddr)
	if err != nil {
		return fmt.Errorf("parse peer address: %w", err)
	}
	debugf("connect peer requested: peer=%s addr=%s", info.ID, peerAddr)
	return c.connectAddrInfo(ctx, info)
}

func (c *ClientNode) SendDirectMessage(ctx context.Context, peerID peer.ID, data []byte) error {
	stream, err := c.host.NewStream(ctx, peerID, c.cfg.directProtocol)
	if err != nil {
		return fmt.Errorf("open direct stream to %s: %w", peerID, err)
	}
	defer stream.Close()

	if _, err := stream.Write(data); err != nil {
		_ = stream.Reset()
		return fmt.Errorf("write direct message to %s: %w", peerID, err)
	}

	if err := stream.CloseWrite(); err != nil {
		return fmt.Errorf("close write side for %s: %w", peerID, err)
	}
	return nil
}

func (c *ClientNode) ConnectedPeers() []peer.ID {
	peers := c.host.Network().Peers()
	result := make([]peer.ID, len(peers))
	copy(result, peers)
	return result
}

func (c *ClientNode) DisconnectPeer(peerID peer.ID) error {
	if err := c.host.Network().ClosePeer(peerID); err != nil {
		return fmt.Errorf("disconnect peer %s: %w", peerID, err)
	}
	return nil
}

// EmitEvent encapsula la lógica asíncrona para enviar respuestas o eventos al EventHandler exterior
func (c *ClientNode) EmitEvent(data []byte) {
	if c.cfg.eventHandler != nil && !c.closed {
		c.wg.Add(1)
		go func(payload []byte) {
			defer c.wg.Done()
			c.cfg.eventHandler(payload)
		}(data)
	}
}

func (c *ClientNode) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	for cidKey, cancel := range c.announcements {
		cancel()
		delete(c.announcements, cidKey)
	}
	for _, topic := range c.topics {
		if topic.subCancel != nil {
			topic.subCancel()
		}
		if topic.subscription != nil {
			topic.subscription.Cancel()
		}
		if topic.discoveryCancel != nil {
			topic.discoveryCancel()
		}
	}
	c.mu.Unlock()

	c.cancel()
	if c.mdnsService != nil {
		_ = c.mdnsService.Close()
	}
	if c.dht != nil {
		_ = c.dht.Close()
	}
	if c.host != nil {
		_ = c.host.Close()
	}
	c.wg.Wait()
	return nil
}

func (c *ClientNode) ensureTopic(topicName string) (*clientTopic, error) {
	c.mu.RLock()
	state, ok := c.topics[topicName]
	c.mu.RUnlock()
	if ok {
		return state, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if state, ok = c.topics[topicName]; ok {
		return state, nil
	}

	topic, err := c.pubsub.Join(topicName)
	if err != nil {
		return nil, fmt.Errorf("join topic %q: %w", topicName, err)
	}

	discoveryCtx, cancel := context.WithCancel(c.ctx)
	state = &clientTopic{
		topic:           topic,
		discoveryCancel: cancel,
	}
	c.topics[topicName] = state

	c.wg.Add(1)
	go c.topicDiscoveryLoop(discoveryCtx, topicName)
	return state, nil
}

func (c *ClientNode) topicDiscoveryLoop(ctx context.Context, topicName string) {
	defer c.wg.Done()
	namespace := c.cfg.topicDiscoveryPrefix + topicName

	advertise := func() {
		discoveryutil.Advertise(ctx, c.routingDiscovery, namespace)
	}
	advertise()

	ticker := time.NewTicker(c.cfg.discoveryInterval)
	defer ticker.Stop()

	for {
		peerCh, err := c.routingDiscovery.FindPeers(ctx, namespace)
		if err == nil {
			for info := range peerCh {
				if info.ID == "" || info.ID == c.host.ID() {
					continue
				}
				c.handleDiscoveredPeer(info)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			advertise()
		}
	}
}

func (c *ClientNode) refreshProvideLoop(ctx context.Context, cidString string, contentID cid.Cid) {
	defer c.wg.Done()
	ticker := time.NewTicker(c.cfg.providerRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			announceCtx, cancel := context.WithTimeout(c.ctx, c.cfg.dialTimeout)
			_ = c.dht.Provide(announceCtx, contentID, true)
			cancel()
		}
	}
}

func (c *ClientNode) handleDirectStream(stream network.Stream) {
	defer stream.Close()
	payload, err := io.ReadAll(stream)
	if err != nil {
		_ = stream.Reset()
		return
	}

	if c.cfg.eventHandler != nil {
		id := make([]byte, 16)
		if len(payload) >= 20 {
			copy(id, payload[4:20])
		}

		parser := &protocol.ProtocolParser{}
		evtMsg := parser.FormatEventDirectMsg(id, stream.Conn().RemotePeer().String(), payload)

		c.wg.Add(1)
		go func(data []byte) {
			defer c.wg.Done()
			c.cfg.eventHandler(data)
		}(evtMsg)
	}

	if c.cfg.directHandler != nil {
		c.wg.Add(1)
		go func(from peer.ID, data []byte) {
			defer c.wg.Done()
			c.cfg.directHandler(c.ctx, from, data)
		}(stream.Conn().RemotePeer(), append([]byte(nil), payload...))
	}
}

func (c *ClientNode) handleMDNSPeer(info peer.AddrInfo) {
	c.handleDiscoveredPeer(info)
}

func (c *ClientNode) handleDiscoveredPeer(info peer.AddrInfo) {
	if info.ID == c.host.ID() {
		return
	}
	debugf("peer discovered: peer=%s addrs=%v", info.ID, info.Addrs)

	if c.cfg.eventHandler != nil {
		parser := &protocol.ProtocolParser{}
		evtMsg := parser.FormatEventPeerDiscovered(info.ID.String())
		c.wg.Add(1)
		go func(data []byte) {
			defer c.wg.Done()
			c.cfg.eventHandler(data)
		}(evtMsg)
	}

	c.host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ctx, cancel := context.WithTimeout(c.ctx, c.cfg.dialTimeout)
		defer cancel()
		_ = c.connectAddrInfo(ctx, info)
	}()
}

func (c *ClientNode) connectAddrInfo(ctx context.Context, info peer.AddrInfo) error {
	debugf("dial start: peer=%s addrs=%v", info.ID, info.Addrs)
	c.host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
	if err := c.host.Connect(ctx, info); err != nil {
		debugf("dial failed: peer=%s err=%v", info.ID, err)
		return fmt.Errorf("connect to %s: %w", info.ID, err)
	}
	debugf("dial ok: peer=%s", info.ID)
	return nil
}

// LogHandler is the external logging injection point
type LogHandler func(level, msg string)

var globalLogHandler LogHandler

func debugf(format string, args ...any) {
	if globalLogHandler != nil {
		globalLogHandler("DEBUG", fmt.Sprintf(format, args...))
	} else if isDebugEnabled() {
		log.Printf("[synap2p-debug] "+format, args...)
	}
}

func isDebugEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SYNAP2P_DEBUG")))
	switch v {
	case "1", "true", "yes", "on", "debug":
		return true
	default:
		return false
	}
}

func (c *ClientNode) bootstrapKnownPeers(peers []peer.AddrInfo) {
	for _, peerInfo := range peers {
		info := peerInfo
		if info.ID == "" || info.ID == c.host.ID() {
			continue
		}
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			ctx, cancel := context.WithTimeout(c.ctx, c.cfg.dialTimeout)
			defer cancel()
			_ = c.connectAddrInfo(ctx, info)
		}()
	}
}
