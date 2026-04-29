package quicnet

import (
	"github.com/DiegoSandival/synap2p-go/protocol"
)

// Engine es la fachada principal de la biblioteca. Envuelve todas las dependencias
// necesarias (Cliente, Handler, Parser) y expone una API extremadamente simple
// para quienes consumen quicnet desde afuera (ej. un daemon de WebSocket).
type Engine struct {
	Client  *ClientNode
	Handler *CentralHandler
	Parser  *protocol.ProtocolParser
}

// NewEngine inicializa y orquesta limpiamente el Cliente P2P y sus manejadores.
func NewEngine(opts ...Option) (*Engine, error) {
	client, err := NewClient(opts...)
	if err != nil {
		return nil, err
	}

	handler := NewCentralHandler(client)
	parser := &protocol.ProtocolParser{}

	return &Engine{
		Client:  client,
		Handler: handler,
		Parser:  parser,
	}, nil
}

// Process invoca el enrutador asíncrono para despachar solicitudes P2P.
// Esta función es segura, asíncrona y no bloqueante.
func (e *Engine) Process(msg []byte) {
	ProcessRequest(msg, e.Parser, e.Handler)
}

// Close apaga el nodo P2P y libera todos los recursos subyacentes.
func (e *Engine) Close() error {
	if e.Client != nil {
		return e.Client.Close()
	}
	return nil
}
