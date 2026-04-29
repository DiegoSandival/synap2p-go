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

// Estructuras de Request para operaciones base
type AddrReq struct {
	Addr []byte
}

type TopicReq struct {
	Topic []byte
}

type DataReq struct {
	Data []byte
}

type CIDReq struct {
	CID []byte
}

type PeerReq struct {
	PeerID []byte
}

type DirectMsgReq struct {
	PeerID []byte
	Data   []byte
}

// --- Parsers Genéricos ---

// ParseU16LenPayload lee un payload con formato [Opcode: 4] + [Len: 2] + [Bytes: Len]
func (p *ProtocolParser) ParseU16LenPayload(msg []byte) ([]byte, error) {
	if len(msg) < 6 {
		return nil, fmt.Errorf("mensaje muy corto")
	}
	length := int(binary.BigEndian.Uint16(msg[4:6]))
	if len(msg) < 6+length {
		return nil, fmt.Errorf("mensaje truncado, esperaba %d bytes", length)
	}
	payload := make([]byte, length)
	copy(payload, msg[6:6+length])
	return payload, nil
}

// ParseU32LenPayload lee un payload con formato [Opcode: 4] + [Len: 4] + [Bytes: Len]
func (p *ProtocolParser) ParseU32LenPayload(msg []byte) ([]byte, error) {
	if len(msg) < 8 {
		return nil, fmt.Errorf("mensaje muy corto")
	}
	length := int(binary.BigEndian.Uint32(msg[4:8]))
	if len(msg) < 8+length {
		return nil, fmt.Errorf("mensaje truncado, esperaba %d bytes", length)
	}
	payload := make([]byte, length)
	copy(payload, msg[8:8+length])
	return payload, nil
}

// ParseDirectMsg lee [Opcode: 4] + [PeerLen: 2] + [PeerID: PeerLen] + [DataLen: 4] + [Data: DataLen]
func (p *ProtocolParser) ParseDirectMsg(msg []byte) (DirectMsgReq, error) {
	var req DirectMsgReq
	if len(msg) < 6 {
		return req, fmt.Errorf("mensaje corto para DirectMsg")
	}
	peerLen := int(binary.BigEndian.Uint16(msg[4:6]))
	offset := 6

	if len(msg) < offset+peerLen {
		return req, fmt.Errorf("id de peer truncado")
	}
	req.PeerID = make([]byte, peerLen)
	copy(req.PeerID, msg[offset:offset+peerLen])
	offset += peerLen

	if len(msg) < offset+4 {
		return req, fmt.Errorf("faltan bytes para longitud de data")
	}
	dataLen := int(binary.BigEndian.Uint32(msg[offset : offset+4]))
	offset += 4

	if len(msg) < offset+dataLen {
		return req, fmt.Errorf("data truncada")
	}
	req.Data = make([]byte, dataLen)
	copy(req.Data, msg[offset:offset+dataLen])

	return req, nil
}

// Utilidades para armar respuestas genéricas o de éxito (puedes personalizarlas luego)
func (p *ProtocolParser) SuccessResponse(opcode byte) []byte {
	return []byte{0x00, 0x00, 0x00, opcode, 0x01} // 1: Success
}

func (p *ProtocolParser) ErrorResponse(opcode byte) []byte {
	return []byte{0x00, 0x00, 0x00, opcode, 0x00} // 0: Error
}
