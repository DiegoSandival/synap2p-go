package quicnet

type CentralHandler struct {
	Client *ClientNode
}

// Inyectamos las dependencias necesarias de la lógica (ClientNode)
func NewCentralHandler(client *ClientNode) *CentralHandler {
	return &CentralHandler{
		Client: client,
	}
}
