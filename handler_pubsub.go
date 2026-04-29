package quicnet

import (
	"context"
	"strings"

	"github.com/DiegoSandival/synap2p-go/protocol"
)

func (h *CentralHandler) Sub(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, topicBytes, err := parser.ParseSinglePayload(payload) // Topic
	if err != nil {
		return parser.ErrorResponse(0x03, id)
	}

	topic := string(topicBytes)
	err = h.Client.Subscribe(topic, func(msg []byte) {})
	if err != nil {
		return parser.ErrorResponse(0x03, id)
	}

	return parser.SuccessResponse(0x03, id)
}

func (h *CentralHandler) Use(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, topicBytes, err := parser.ParseSinglePayload(payload) // Topic
	if err != nil {
		return parser.ErrorResponse(0x04, id)
	}

	topic := string(topicBytes)
	_, err = h.Client.ensureTopic(topic)
	if err != nil {
		return parser.ErrorResponse(0x04, id)
	}

	return parser.SuccessResponse(0x04, id)
}

func (h *CentralHandler) Pub(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, dataBytes, err := parser.ParseSinglePayload(payload) // Topic:Data
	// Asumimos formato Topic|Data o algo así, ya que pubsub necesita el topic name.
	// Si "Data: N" en el protocolo no especifica topic, habría que separarlo.
	// Para uso sencillo, supongamos que los primeros N bytes hasta un separador son Topic,
	// o usaremos un separador como "|".
	if err != nil {
		return parser.ErrorResponse(0x05, id)
	}

	strData := string(dataBytes)
	idx := strings.Index(strData, "|")
	if idx == -1 {
		return parser.ErrorResponse(0x05, id) // requiere Topic|Data
	}

	topic := strData[:idx]
	data := []byte(strData[idx+1:])

	err = h.Client.Publish(context.Background(), topic, data)
	if err != nil {
		return parser.ErrorResponse(0x05, id)
	}

	return parser.SuccessResponse(0x05, id)
}

func (h *CentralHandler) Unsub(parser *protocol.ProtocolParser, payload []byte) []byte {
	id, topicBytes, err := parser.ParseSinglePayload(payload) // Topic
	if err != nil {
		return parser.ErrorResponse(0x07, id)
	}

	topic := string(topicBytes)
	err = h.Client.Unsubscribe(topic)
	if err != nil {
		return parser.ErrorResponse(0x07, id)
	}

	return parser.SuccessResponse(0x07, id)
}
