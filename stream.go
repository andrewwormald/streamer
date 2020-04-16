package streamer

import (
	"context"
	"github.com/luno/jettison/errors"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Stream struct {
	mu   sync.RWMutex
	pool map[string]*client
	u    websocket.Upgrader
}

// New returns a new implementation of the Stream struct and kicks off the housekeeping loop to ensure all closed
// connections are removed from the Stream
func New() *Stream {
	s := &Stream{
		pool: make(map[string]*client),
		u: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}

	go s.cleanClientPoolForever()

	return s
}

// Accept takes ownership of upgrading the HTTP server connection to the WebSocket protocol and adding the new connection
// to the Stream's client pool.
func (s *Stream) Accept(ctx context.Context, w http.ResponseWriter, r *http.Request, clientKey string) error {
	s.u.CheckOrigin = func(r *http.Request) bool { return true }

	conn, err := s.u.Upgrade(w, r, nil)
	if err != nil {
		return errors.Wrap(err, "failed to upgrade connection")
	}

	cl := newClient(conn, clientKey)
	s.store(cl)

	return nil
}

// Publish sends a message to all of the open clients in the Stream's client pool with a context.WithTimeout set to one
// second to ensure the loop does not hang due to a client struggling to consume it's write buffer.
func (s *Stream) Publish(msg string) {
	for _, c := range s.clients() {
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		c.Message(ctx, msg)
	}
}

// Exists uses the provided clientID to determine if the client is alive and part of the Stream's client pool.
func (s *Stream) Exists(clientID string) bool {
	for _, v := range s.clients() {
		if v.ID() == clientID {
			return true
		}
	}

	return false
}

// OpenConnections returns the amount of clients that are in the Stream.
func (s *Stream) OpenConnections() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.pool)
}

// store safely adds the client to the Stream's client pool without causing any data races.
func (s *Stream) store(c *client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pool[c.ID()] = c
}

// remove safely removes the client from the Stream's client pool without causing any data races.
func (s *Stream) remove(c *client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pool, c.ID())
}

// clients create a clean copy of the client pool without causing any data races.
func (s *Stream) clients() []client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sc []client
	for _, value := range s.pool {
		sc = append(sc, *value)
	}

	return sc
}

// cleanClientPoolForever runs every seconds to ensure all closed connections are remove from the Stream's client pool
func (s *Stream) cleanClientPoolForever() []client {
	for {
		// update every second
		for _, c := range s.clients() {
			if c.Alive() {
				continue
			}

			s.remove(&c)
		}
		time.Sleep(time.Second)
	}
}
