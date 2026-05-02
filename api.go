package quicnet

import (
	"crypto/rand"
	"fmt"
	"strings"
)

const (
	OpcodeRelay         byte = 0x01
	OpcodeDial          byte = 0x02
	OpcodeSubscribe     byte = 0x03
	OpcodeUseTopic      byte = 0x04
	OpcodePublish       byte = 0x05
	OpcodePeers         byte = 0x06
	OpcodeUnsubscribe   byte = 0x07
	OpcodeAnnounceCID   byte = 0x08
	OpcodeUnannounceCID byte = 0x09
	OpcodeFindProviders byte = 0x0A
	OpcodeDirectMessage byte = 0x0B
	OpcodeDisconnect    byte = 0x0C
	OpcodeGenerateCID   byte = 0x0D
)

func NewRequestID() ([]byte, error) {
	requestID := make([]byte, 16)
	if _, err := rand.Read(requestID); err != nil {
		return nil, fmt.Errorf("generate request ID: %w", err)
	}
	return requestID, nil
}

func (n *Node) Relay(relayAddr string) ([]byte, error) {
	return n.sendSimpleOpcode(OpcodeRelay, relayAddr)
}

func (n *Node) Dial(peerAddr string) ([]byte, error) {
	return n.sendSimpleOpcode(OpcodeDial, peerAddr)
}

func (n *Node) Subscribe(topic string) ([]byte, error) {
	return n.sendSimpleOpcode(OpcodeSubscribe, topic)
}

func (n *Node) UseTopic(topic string) ([]byte, error) {
	return n.sendSimpleOpcode(OpcodeUseTopic, topic)
}

func (n *Node) Publish(topic string, data []byte) ([]byte, error) {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil, fmt.Errorf("topic is required")
	}
	if strings.Contains(topic, "|") {
		return nil, fmt.Errorf("topic cannot contain '|'")
	}

	payload := make([]byte, len(topic)+1+len(data))
	copy(payload, topic)
	payload[len(topic)] = '|'
	copy(payload[len(topic)+1:], data)
	return n.sendOpcodeWithGeneratedID(OpcodePublish, payload)
}

func (n *Node) Peers() ([]byte, error) {
	return n.sendOpcodeWithGeneratedID(OpcodePeers, nil)
}

func (n *Node) Unsubscribe(topic string) ([]byte, error) {
	return n.sendSimpleOpcode(OpcodeUnsubscribe, topic)
}

func (n *Node) AnnounceCID(cid string) ([]byte, error) {
	return n.sendSimpleOpcode(OpcodeAnnounceCID, cid)
}

func (n *Node) UnannounceCID(cid string) ([]byte, error) {
	return n.sendSimpleOpcode(OpcodeUnannounceCID, cid)
}

func (n *Node) FindProviders(cid string) ([]byte, error) {
	return n.sendSimpleOpcode(OpcodeFindProviders, cid)
}

func (n *Node) Disconnect(peerID string) ([]byte, error) {
	return n.sendSimpleOpcode(OpcodeDisconnect, peerID)
}

func (n *Node) GenerateCID(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is required")
	}
	return n.sendOpcodeWithGeneratedID(OpcodeGenerateCID, data)
}

func (n *Node) SendMessage(peerID string, msg []byte) ([]byte, error) {
	if strings.TrimSpace(peerID) == "" {
		return nil, fmt.Errorf("peer ID is required")
	}
	requestID, err := NewRequestID()
	if err != nil {
		return nil, err
	}
	if err := n.SendDirectOpcode(requestID, peerID, msg); err != nil {
		return nil, err
	}
	return requestID, nil
}

func (n *Node) sendSimpleOpcode(opcode byte, value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("payload is required")
	}
	return n.sendOpcodeWithGeneratedID(opcode, []byte(trimmed))
}

func (n *Node) sendOpcodeWithGeneratedID(opcode byte, payload []byte) ([]byte, error) {
	requestID, err := NewRequestID()
	if err != nil {
		return nil, err
	}
	if err := n.SendOpcode(opcode, requestID, payload); err != nil {
		return nil, err
	}
	return requestID, nil
}
