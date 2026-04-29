package quicnet

import (
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
)

type CentralHandler struct {
	Host   host.Host
	DHT    *dht.IpfsDHT
	PubSub *pubsub.PubSub
}

// Inyectamos las dependencias necesarias de libp2p
func NewCentralHandler(h host.Host, dhtInstance *dht.IpfsDHT, pubsubInstance *pubsub.PubSub) *CentralHandler {
	return &CentralHandler{
		Host:   h,
		DHT:    dhtInstance,
		PubSub: pubsubInstance,
	}
}
