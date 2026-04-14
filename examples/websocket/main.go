package main

import (
	"context"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	quicnet "github.com/DiegoSandival/synap2p-go"
	"github.com/gorilla/websocket"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	commandTimeout = 15 * time.Second

	cmdRelay = 0x01
	cmdDial  = 0x02
	cmdSub   = 0x03
	cmdUse   = 0x04
	cmdPub   = 0x05
	cmdPeers = 0x06

	respOK    = 0x00
	respError = 0x01
	evtGossip = 0x80
	evtDirect = 0x81
)

type wsSession struct {
	client           *quicnet.ClientNode
	conn             *websocket.Conn
	writeMu          sync.Mutex
	currentTopic     string
	subscribedTopics map[string]struct{}
	stateMu          sync.RWMutex
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	keyPath := flag.String("key", "./ws_identity.key", "path to the client identity key")
	listenAddr := flag.String("listen", "127.0.0.1:8080", "HTTP listen address for the websocket server")
	path := flag.String("path", "/ws", "websocket path")
	bootstrap := flag.String("bootstrap", "", "comma-separated bootstrap peer multiaddrs")
	protocolPrefix := flag.String("protocol", "/synap2p", "protocol prefix for the DHT")
	flag.Parse()

	opts := []quicnet.Option{
		quicnet.WithKeyPath(*keyPath),
		quicnet.WithProtocolPrefix(*protocolPrefix),
	}

	if bootstrapPeers := splitCSV(*bootstrap); len(bootstrapPeers) > 0 {
		opts = append(opts, quicnet.WithBootstrapPeers(bootstrapPeers...))
	}

	handlerState := &serverState{opts: opts}
	mux := http.NewServeMux()
	mux.HandleFunc(*path, handlerState.handleWS)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("synap2p websocket example\nuse binary websocket frames on " + *path + "\n"))
	})

	server := &http.Server{
		Addr:              *listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("websocket example listening on ws://%s%s", *listenAddr, *path)
	log.Printf("supported commands: relay(0x01), dial(0x02), sub(0x03), use(0x04), pub(0x05), peers(0x06)")

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case <-signals:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	case err := <-errCh:
		log.Fatalf("websocket server failed: %v", err)
	}
}

type serverState struct {
	opts []quicnet.Option
}

func (s *serverState) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade websocket: %v", err)
		return
	}

	client, err := quicnet.NewClient(append([]quicnet.Option(nil), s.opts...)...)
	if err != nil {
		_ = conn.Close()
		log.Printf("create client for websocket session: %v", err)
		return
	}

	session := &wsSession{
		client:           client,
		conn:             conn,
		subscribedTopics: make(map[string]struct{}),
	}

	session.installDirectHandler()
	log.Printf("websocket session opened: peer=%s remote=%s", client.ID(), r.RemoteAddr)

	defer func() {
		_ = session.client.Close()
		_ = session.conn.Close()
		log.Printf("websocket session closed: peer=%s remote=%s", client.ID(), r.RemoteAddr)
	}()

	if err := session.sendSessionHello(); err != nil {
		log.Printf("send hello frame: %v", err)
		return
	}

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			log.Printf("read websocket frame: %v", err)
			return
		}

		if messageType != websocket.BinaryMessage {
			if err := session.sendError(0x00, "only binary websocket frames are supported"); err != nil {
				log.Printf("send frame type error: %v", err)
				return
			}
			continue
		}

		if err := session.handleFrame(payload); err != nil {
			log.Printf("handle frame: %v", err)
			return
		}
	}
}

func (s *wsSession) installDirectHandler() {
	// Direct messages are already configured on the client at construction time, so this example
	// forwards only gossip and direct events generated from subscription callbacks and stream handlers.
	// The public API does not let us replace the direct handler after NewClient, so keep this hook for symmetry.
}

func (s *wsSession) sendSessionHello() error {
	peerID := []byte(s.client.ID().String())
	addrs := s.client.Addrs()
	payloadLen := 2 + len(peerID) + 2
	for _, addr := range addrs {
		payloadLen += 2 + len(addr)
	}

	payload := make([]byte, 2, 2+payloadLen)
	payload[0] = respOK
	payload[1] = 0x7F
	payload = appendU16Bytes(payload, peerID)
	payload = appendU16(payload, uint16(len(addrs)))
	for _, addr := range addrs {
		payload = appendU16Bytes(payload, []byte(addr))
	}
	return s.writeBinary(payload)
}

func (s *wsSession) handleFrame(frame []byte) error {
	if len(frame) == 0 {
		return s.sendError(0x00, "empty frame")
	}

	opcode := frame[0]
	payload := frame[1:]

	switch opcode {
	case cmdRelay:
		addr, rest, err := readU16Bytes(payload)
		if err != nil || len(rest) != 0 {
			return s.sendError(opcode, "invalid relay frame")
		}
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		err = s.client.ConnectToRelay(ctx, string(addr))
		cancel()
		if err != nil {
			return s.sendError(opcode, err.Error())
		}
		return s.sendOK(opcode)

	case cmdDial:
		addr, rest, err := readU16Bytes(payload)
		if err != nil || len(rest) != 0 {
			return s.sendError(opcode, "invalid dial frame")
		}
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		err = s.client.ConnectToPeer(ctx, string(addr))
		cancel()
		if err != nil {
			return s.sendError(opcode, err.Error())
		}
		return s.sendOK(opcode)

	case cmdSub:
		topic, rest, err := readU16Bytes(payload)
		if err != nil || len(rest) != 0 {
			return s.sendError(opcode, "invalid sub frame")
		}
		topicName := string(topic)
		s.stateMu.Lock()
		_, exists := s.subscribedTopics[topicName]
		s.stateMu.Unlock()
		if exists {
			return s.sendOK(opcode)
		}
		if err := s.client.Subscribe(topicName, func(msg []byte) {
			if err := s.sendGossipEvent(topicName, msg); err != nil {
				log.Printf("send gossip event: %v", err)
			}
		}); err != nil {
			return s.sendError(opcode, err.Error())
		}
		s.stateMu.Lock()
		s.subscribedTopics[topicName] = struct{}{}
		s.stateMu.Unlock()
		return s.sendOK(opcode)

	case cmdUse:
		topic, rest, err := readU16Bytes(payload)
		if err != nil || len(rest) != 0 {
			return s.sendError(opcode, "invalid use frame")
		}
		topicName := string(topic)
		s.stateMu.RLock()
		_, exists := s.subscribedTopics[topicName]
		s.stateMu.RUnlock()
		if !exists {
			return s.sendError(opcode, "topic is not subscribed")
		}
		s.stateMu.Lock()
		s.currentTopic = topicName
		s.stateMu.Unlock()
		return s.sendOK(opcode)

	case cmdPub:
		message, rest, err := readU32Bytes(payload)
		if err != nil || len(rest) != 0 {
			return s.sendError(opcode, "invalid pub frame")
		}
		s.stateMu.RLock()
		topicName := s.currentTopic
		s.stateMu.RUnlock()
		if topicName == "" {
			return s.sendError(opcode, "no current topic selected")
		}
		ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
		err = s.client.Publish(ctx, topicName, message)
		cancel()
		if err != nil {
			return s.sendError(opcode, err.Error())
		}
		return s.sendOK(opcode)

	case cmdPeers:
		if len(payload) != 0 {
			return s.sendError(opcode, "peers frame must not contain payload")
		}
		return s.sendPeers()

	default:
		return s.sendError(opcode, fmt.Sprintf("unknown opcode 0x%02x", opcode))
	}
}

func (s *wsSession) sendGossipEvent(topic string, data []byte) error {
	payload := make([]byte, 2, 2+2+len(topic)+4+len(data))
	payload[0] = evtGossip
	payload[1] = 0x00
	payload = appendU16Bytes(payload, []byte(topic))
	payload = appendU32Bytes(payload, data)
	return s.writeBinary(payload)
}

func (s *wsSession) sendDirectEvent(from peer.ID, data []byte) error {
	payload := make([]byte, 2, 2+2+len(from.String())+4+len(data))
	payload[0] = evtDirect
	payload[1] = 0x00
	payload = appendU16Bytes(payload, []byte(from.String()))
	payload = appendU32Bytes(payload, data)
	return s.writeBinary(payload)
}

func (s *wsSession) sendOK(opcode byte) error {
	return s.writeBinary([]byte{respOK, opcode})
}

func (s *wsSession) sendError(opcode byte, message string) error {
	payload := []byte{respError, opcode}
	payload = appendU16Bytes(payload, []byte(message))
	return s.writeBinary(payload)
}

func (s *wsSession) sendPeers() error {
	peers := s.client.ConnectedPeers()
	peerStrings := make([]string, 0, len(peers))
	for _, peerID := range peers {
		peerStrings = append(peerStrings, peerID.String())
	}
	sort.Strings(peerStrings)

	payload := []byte{respOK, cmdPeers}
	payload = appendU16(payload, uint16(len(peerStrings)))
	for _, peerID := range peerStrings {
		payload = appendU16Bytes(payload, []byte(peerID))
	}
	return s.writeBinary(payload)
}

func (s *wsSession) writeBinary(payload []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.WriteMessage(websocket.BinaryMessage, payload)
}

func readU16Bytes(payload []byte) ([]byte, []byte, error) {
	if len(payload) < 2 {
		return nil, nil, fmt.Errorf("missing u16 length")
	}
	length := int(binary.BigEndian.Uint16(payload[:2]))
	payload = payload[2:]
	if len(payload) < length {
		return nil, nil, fmt.Errorf("payload shorter than u16 length")
	}
	return payload[:length], payload[length:], nil
}

func readU32Bytes(payload []byte) ([]byte, []byte, error) {
	if len(payload) < 4 {
		return nil, nil, fmt.Errorf("missing u32 length")
	}
	length := int(binary.BigEndian.Uint32(payload[:4]))
	payload = payload[4:]
	if len(payload) < length {
		return nil, nil, fmt.Errorf("payload shorter than u32 length")
	}
	return payload[:length], payload[length:], nil
}

func appendU16(dst []byte, value uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, value)
	return append(dst, buf...)
}

func appendU16Bytes(dst []byte, payload []byte) []byte {
	if len(payload) > 0xFFFF {
		payload = payload[:0xFFFF]
	}
	dst = appendU16(dst, uint16(len(payload)))
	return append(dst, payload...)
}

func appendU32Bytes(dst []byte, payload []byte) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(len(payload)))
	dst = append(dst, buf...)
	return append(dst, payload...)
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		items = append(items, trimmed)
	}
	return items
}