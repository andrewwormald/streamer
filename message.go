package streamer

type ReceiveMessage struct {
	ID      string
	Message string
}

type SendMessage struct {
	Topic   string
	Message string
}
