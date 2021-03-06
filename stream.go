package streamer

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
)

// Stream is a abstracted ws client connection pool that has an API to interact with the entire pool of client
// connections. Each pool requires that the connection is accepted into it before it can include it or listen to it.
type Stream struct {
	mu       sync.RWMutex
	readBuff chan ReceiveMessage
	pool     map[string]*Channel
	u        websocket.Upgrader
}

// New returns a new implementation of the Stream struct and kicks off the housekeeping loop to ensure all closed
// connections are removed from the Stream
func New(opts ...StreamOption) *Stream {
	s := &Stream{
		pool:     make(map[string]*Channel),
		readBuff: make(chan ReceiveMessage, 10),
		u: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}

	for _, o := range opts {
		o(s)
	}

	go s.cleanPoolForever()

	return s
}

// StreamOption is a type that allows configuration of the Stream type.
type StreamOption func(*Stream)

// WithReadBufferSize takes an int which is used to set the go channel size and therefore passing 1 would entail a
// limit of 1 message to be queued at a time.
func WithReadBufferSize(size int) StreamOption {
	return func(s *Stream) {
		s.readBuff = make(chan ReceiveMessage, size)
	}
}

// WithUpgrader allows the stream to be configured with a custom gorilla websocket upgrader.
func WithUpgrader(u websocket.Upgrader) StreamOption {
	return func(s *Stream) {
		s.u = u
	}
}

// Accept takes ownership of upgrading the HTTP server connection to the WebSocket protocol and adding the new connection
// to the Stream's channel pool.
func (s *Stream) Accept(w http.ResponseWriter, r *http.Request, channelKey string) error {
	s.u.CheckOrigin = func(r *http.Request) bool { return true }

	conn, err := s.u.Upgrade(w, r, nil)
	if err != nil {
		return errors.Wrap(err, "failed to upgrade connection")
	}

	cl := NewChannel(conn, channelKey)
	s.store(cl)

	return nil
}

// Publish sends a message to all of the open channels in the Stream's channel pool and takes channelTimeout which it uses to
// set a deadline per channel.
func (s *Stream) Publish(m SendMessage) {
	for _, c := range s.channels() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
		err := c.Send(ctx, m)
		if err != nil {
			// NoReturnErr: Allow other channels to be unaffected and close this connection
			c.Close(websocket.CloseTryAgainLater)
		}
		cancel()
	}
}

// Responder is blocking method that should be run in a goroutine for responding and handling received messages
func (s *Stream) Responder(handler func(m ReceiveMessage)) {
	for {
		handler(<-s.readBuff)
	}
}

// Read returns the streams read buffer that it consumes from for handling messages from the stream's channels
func (s *Stream) Read() chan ReceiveMessage {
	return s.readBuff
}

// ErrChannelDoesNotExist is returned when the channel is not found in the stream
var ErrChannelDoesNotExist = errors.New("channel does not exist", j.C("ERR_bcd404068d4f7f1b"))

// Collect uses the provided channelID to fetch the channel
func (s *Stream) Collect(channelID string) (*Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.pool[channelID]; !ok {
		return nil, ErrChannelDoesNotExist
	}

	return s.pool[channelID], nil
}

// Connections returns the amount of valid channels that are in the Stream.
func (s *Stream) Connections() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.pool)
}

// store safely adds the channel to the Stream's channel pool without causing any data races.
func (s *Stream) store(c *Channel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pool[c.ID()] = c
}

// remove safely removes the channel from the Stream's channel pool without causing any data races.
func (s *Stream) remove(c *Channel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pool, c.ID())
}

// channels returns a slice version of the pool in a async safe manner.
func (s *Stream) channels() []Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sc []Channel
	for _, value := range s.pool {
		sc = append(sc, *value)
	}

	return sc
}

// cleanPoolForever is a blocking method of Stream that runs every second to ensure all closed connections are remove
// from the Stream's pool of channels
func (s *Stream) cleanPoolForever() {
	for {
		for _, c := range s.channels() {
			if !c.Closed() {
				continue
			}

			s.remove(&c)
		}
		time.Sleep(time.Second)
	}
}
