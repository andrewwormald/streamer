package streamer

// ReceiveMessage is the data structure expected from the client
type ReceiveMessage struct {
	ChannelID string `json:"id"`
	Message   string `json:"message"`
}

// SendMessage is the outgoing data structure being sent from the stream to the client
type SendMessage struct {
	Topic   string `json:"topic"`
	Message string `json:"message"`
}
