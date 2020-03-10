package stream

import (
	"context"
	"github.com/luno/jettison/errors"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"streamer/client"
)

type Streamer interface {
	Accept(ctx context.Context, w http.ResponseWriter, r *http.Request, clientKey string) error
	ClientExists(clientKey string) bool
	Publish(message string)
}

type stream struct {
	mu   sync.RWMutex
	pool map[string]*client.Client
	u websocket.Upgrader
}

// New returns a new implementation of the stream struct and kicks off the housekeeping loop to ensure all closed
// connections are removed from the stream
func New() *stream {
	s := &stream{
		pool: make(map[string]*client.Client),
		u: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}

	go s.cleanClientPoolForever()

	return s
}

// Accept takes ownership of upgrading the HTTP server connection to the WebSocket protocol and adding the new connection
// to the stream's client pool.
func (s *stream) Accept(ctx context.Context, w http.ResponseWriter, r *http.Request, clientKey string) error {
	s.u.CheckOrigin = func(r *http.Request) bool { return true }

	conn, err := s.u.Upgrade(w, r, nil)
	if err != nil {
		return errors.Wrap(err, "failed to upgrade connection")
	}

	cl := client.New(conn, clientKey)
	s.store(cl)

	return nil
}

// Publish sends a message to all of the open clients in the stream's client pool with a context.WithTimeout set to one
// second to ensure the loop does not hang due to a client struggling to consume it's write buffer.
func (s *stream) Publish(msg string) {
	for _, c := range s.clients() {
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		c.Message(ctx, msg)
	}
}

// ClientExists uses the provided clientID to determine if the client is alive and part of the stream's client pool.
func (s *stream) ClientExists(clientID string) bool {
	for _, v := range s.clients() {
		if v.ID() == clientID {
			return true
		}
	}

	return false
}

// store safely adds the client to the stream's client pool without causing any data races.
func (s *stream) store(c client.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pool[c.ID()] = &c
}

// remove safely removes the client from the stream's client pool without causing any data races.
func (s *stream) remove(c client.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pool, c.ID())
}

// clients create a clean copy of the client pool without causing any data races.
func (s *stream) clients() []client.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sc []client.Client
	for _, value := range s.pool {
		sc = append(sc, *value)
	}

	return sc
}

// cleanClientPoolForever runs every seconds to ensure all closed connections are remove from the stream's client pool
func (s *stream) cleanClientPoolForever() []client.Client {
	for {
		// update every 5 seconds
		for _, c := range s.clients() {
			if c.Alive() {
				continue
			}

			s.remove(c)
		}
		time.Sleep(time.Second)
	}
}

var _ Streamer = (*stream)(nil)
