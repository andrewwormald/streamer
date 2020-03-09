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
	Accept(ctx context.Context, c *websocket.Conn, clientKey string)
	ClientExists(clientKey string) bool
	Publish(message string)
	Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error)
}

type stream struct {
	mu   sync.RWMutex
	pool map[string]*client.Client
	u websocket.Upgrader
}

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

func (s *stream) Upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	s.u.CheckOrigin = func(r *http.Request) bool { return true }

	conn, err := s.u.Upgrade(w, r, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to upgrade connection")
	}

	return conn, nil
}

func (s *stream) Accept(ctx context.Context, c *websocket.Conn, clientKey string) {
	cl := client.New(c, clientKey)
	s.store(cl)
}

func (s *stream) Publish(msg string) {
	for _, c := range s.clients() {
		ctx, _ := context.WithTimeout(context.Background(), time.Second * 10)
		c.Stream(ctx, msg)
	}
}

func (s *stream) ClientExists(clientKey string) bool {
	for _, v := range s.clients() {
		if v.ID() == clientKey {
			return true
		}
	}

	return false
}

func (s *stream) store(c client.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pool[c.ID()] = &c
}

func (s *stream) remove(c client.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pool, c.ID())
}

func (s *stream) clients() []client.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sc []client.Client
	for _, value := range s.pool {
		sc = append(sc, *value)
	}

	return sc
}

func (s *stream) cleanClientPoolForever() []client.Client {
	for {
		for _, c := range s.clients() {
			if c.Alive() {
				continue
			}

			s.remove(c)
		}

		// reduce overhead on the mutex locking
		time.Sleep(time.Second * 5)
	}
}

var _ Streamer = (*stream)(nil)
