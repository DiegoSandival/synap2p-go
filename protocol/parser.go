package protocol

import (
	"encoding/binary"
	"fmt"
)

type ProtocolParser struct{}

// Opcode extrae el comando asumiendo que está en el byte 3 (como "0x00 0x00 0x00 0x0X")
func (p *ProtocolParser) Opcode(msg []byte) byte {
	if len(msg) < 4 {
		return 0xFF
	}
	return msg[3]
}

// ParseSinglePayload asume el formato: [Opcode: 4] + [ID: 16] + [Resto como Payload]
func (p *ProtocolParser) ParseSinglePayload(msg []byte) (id []byte, payload []byte, err error) {
	if len(msg) < 20 {
		return nil, nil, fmt.Errorf("mensaje muy corto, faltan opcode o ID")
	}
	id = make([]byte, 16)
	copy(id, msg[4:20])

	payloadLen := len(msg) - 20
	if payloadLen > 0 {
		payload = make([]byte, payloadLen)
		copy(payload, msg[20:])
	} else {
		payload = []byte{}
	}

	return id, payload, nil
}

// ParseDirectMsg lee: [Opcode: 4] + [ID: 16] + [PeerID Len: 2] + [PeerID: N] + [Data: M]
func (p *ProtocolParser) ParseDirectMsg(msg []byte) (id []byte, peerId []byte, data []byte, err error) {
	if len(msg) < 22 {
		return nil, nil, nil, fmt.Errorf("mensaje corto para DirectMsg")
	}
	id = make([]byte, 16)
	copy(id, msg[4:20])

	peerLen := int(binary.BigEndian.Uint16(msg[20:22]))
	offset := 22

	if len(msg) < offset+peerLen {
		return nil, nil, nil, fmt.Errorf("id de peer truncado")
	}
	peerId = make([]byte, peerLen)
	copy(peerId, msg[offset:offset+peerLen])
	offset += peerLen

	dataLen := len(msg) - offset
	data = make([]byte, dataLen)
	if dataLen > 0 {
		copy(data, msg[offset:])
	}

	return id, peerId, data, nil
}

func (p *ProtocolParser) SuccessResponse(opcode byte, id []byte) []byte {
	res := make([]byte, 21) // 4 + 16 + 1
	res[3] = opcode
	if len(id) == 16 {
		copy(res[4:20], id)
	}
	res[20] = 0x01
	return res
}

func (p *ProtocolParser) SuccessDataResponse(opcode byte, id []byte, data []byte) []byte {
	res := make([]byte, 21+len(data)) // 4 + 16 + 1 (status) + data
	res[3] = opcode
	if len(id) == 16 {
		copy(res[4:20], id)
	}
	res[20] = 0x01
	copy(res[21:], data)
	return res
}

func (p *ProtocolParser) ErrorResponse(opcode byte, id []byte) []byte {
	res := make([]byte, 21) // 4 + 16 + 1
	res[3] = opcode
	if len(id) == 16 {
		copy(res[4:20], id)
	}
	res[20] = 0x00
	return res
}

func (p *ProtocolParser) FormatEventPubSub(id []byte, data []byte) []byte {
	res := make([]byte, 20+len(data))
	res[3] = 0x0E // EVENT_PUBSUB_MSG
	if len(id) == 16 {
		copy(res[4:20], id)
	}
	copy(res[20:], data)
	return res
}

func (p *ProtocolParser) FormatEventDirectMsg(id []byte, peerID string, data []byte) []byte {
	peerIDBytes := []byte(peerID)
	peerLen := len(peerIDBytes)
	res := make([]byte, 20+2+peerLen+len(data))
	res[3] = 0x0F // EVENT_DIRECT_MSG
	if len(id) == 16 {
		copy(res[4:20], id)
	}
	binary.BigEndian.PutUint16(res[20:22], uint16(peerLen))
	copy(res[22:22+peerLen], peerIDBytes)
	copy(res[22+peerLen:], data)
	return res
}

func (p *ProtocolParser) FormatEventPeerDiscovered(peerID string) []byte {
	peerIDBytes := []byte(peerID)
	peerLen := len(peerIDBytes)
	res := make([]byte, 20+2+peerLen)
	res[3] = 0x10 // EVENT_PEER_DISCOVERED
	// ID de 16 bytes inicializado en 0 (basta con crear el arreglo)
	binary.BigEndian.PutUint16(res[20:22], uint16(peerLen))
	copy(res[22:22+peerLen], peerIDBytes)
	return res
}
