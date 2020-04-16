package streamer

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/luno/jettison/log"
)

type client struct {
	mu sync.Mutex
	id string

	writeBuf  chan string
	readBuf   chan string
	interrupt chan os.Signal

	closed bool

	ctx    context.Context
	cancel context.CancelFunc
	conn   *websocket.Conn
}

// New provides a new stream client that satisfies the Client interface. This method kicks off housekeeping loops such
// as listening to messaged from the client and closing the connection when the client disconnects or is too slow.
func newClient(c *websocket.Conn, id string) *client {
	ctx, cancel := context.WithCancel(context.Background())

	s := &client{
		id:        id,
		writeBuf:  make(chan string, 10),
		readBuf:   make(chan string, 10),
		interrupt: make(chan os.Signal, 1),
		ctx:       ctx,
		cancel:    cancel,
		conn:      c,
	}

	signal.Notify(s.interrupt, os.Interrupt)

	go s.run()

	go s.listen()

	return s
}

// NewMock returns a mock client without running any housekeeping loops.
func newMockClient(w *websocket.Conn, ID string) *client {
	ctx, cancel := context.WithCancel(context.Background())

	return &client{
		id:        ID,
		writeBuf:  make(chan string, 20),
		readBuf:   make(chan string, 20),
		interrupt: make(chan os.Signal, 1),
		conn:      w,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// ID returns the id of the client that the stream uses as its key in the client pool.
func (s *client) ID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}

// Alive returns true if the connection is still open and receiving messages.
func (s *client) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.closed
}

// Close sends a close connection message to the client and returns nil if the operation is successful. If nil is
// returned then Alive will begin to return false as the client has now been closed.
func (s *client) Close() error {
	err := s.conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		if errors.Is(err, websocket.ErrCloseSent) {
			s.cancel()
			return nil
		}

		return err
	}

	s.cancel()
	return nil
}

// Message pushes a string to the client's write buffer. If the buffer is full this method will hang until the
// context is cancelled. It is recommended to pass a context with timeout.
func (s *client) Message(ctx context.Context, msg string) {
	select {
	case <-ctx.Done():
		log.Info(ctx, "context of stream client cancelled")
		return

	case s.writeBuf <- msg:
	}
}

// Listen returns a channel than streams the clients messages to the server.
func (s *client) Listen() chan string {
	reply := make(chan string)
	go func() {
		for {
			msg := <-s.readBuf
			reply <- msg
		}
	}()
	return reply
}

// run manages the clients channels and specifically the client's context cancellation.
func (s *client) run() {
	for {
		select {
		case <-s.ctx.Done():
			s.closed = true
			s.Close()
			return
		case <-s.interrupt:
			s.cancel()
			return
		case m := <-s.writeBuf:
			s.write([]byte(m))
		}
	}
}

// write entails a writing deadline and writes a text message to the client. If the operation fails the client's
// context is cancelled and an error is logged.
func (s *client) write(msg []byte) {
	timeout := time.Second * 5
	err := s.conn.SetWriteDeadline(time.Now().Add(timeout))
	if err != nil {
		log.Error(s.ctx, err)
		s.cancel()
	}

	if err := s.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		if !errors.Is(err, websocket.ErrCloseSent) {
			log.Error(s.ctx, err)
			s.cancel()
		}
	}
}

// listen continuously listens for a message from the client and adds the messages to the read buffer as it receives them
func (s *client) listen() {
	for {
		if s.closed {
			return
		}

		msgType, b, err := s.conn.ReadMessage()
		if err != nil {
			s.cancel()
			return
		}

		ctx, _ := context.WithTimeout(context.Background(), time.Second)

		switch msgType {
		case websocket.TextMessage:
			select {
			case <-ctx.Done():
			case s.readBuf <- string(b):
			}
		case websocket.BinaryMessage:
			continue
		case websocket.PingMessage:
			continue
		case websocket.PongMessage:
			continue
		default:
			s.cancel()
		}
	}
}
