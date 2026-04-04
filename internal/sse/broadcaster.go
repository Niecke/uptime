package sse

type Broadcaster struct {
	clients    map[chan string]struct{} // connected SSE clients
	Register   chan chan string         // new client connecting
	Unregister chan chan string         // client disconnecting
	Broadcast  chan string              // new event to send to all clients
}

func NewBroadcaster() Broadcaster {
	var b Broadcaster
	b.clients = make(map[chan string]struct{})
	b.Register = make(chan chan string, 1)
	b.Unregister = make(chan chan string, 1)
	b.Broadcast = make(chan string, 10)
	return b
}

func (b *Broadcaster) Run() {
	for {

		select {
		// register new clients
		case client := <-b.Register:
			b.clients[client] = struct{}{}
		// unregister old clients
		case client := <-b.Unregister:
			delete(b.clients, client)
		// process broadcast messages
		case event := <-b.Broadcast:
			for client := range b.clients {
				client <- event
			}
		}
	}
}
