package client

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

type Client interface {
	ID() string
	Alive() bool
	Close() error
	Stream(ctx context.Context, msg string)
}

type client struct {
	mu sync.Mutex
	id    string

	writeBuf    chan string
	readBuf    chan string
	interrupt chan os.Signal

	closed bool

	ctx context.Context
	cancel context.CancelFunc
	conn  *websocket.Conn
}

func New(c *websocket.Conn, id string) *client {
	ctx, cancel := context.WithCancel(context.Background())

	s := &client{
		id:   id,
		writeBuf:   make(chan string, 20),
		readBuf:   make(chan string, 20),
		interrupt: make(chan os.Signal, 1),
		ctx: ctx,
		cancel: cancel,
		conn: c,
	}

	signal.Notify(s.interrupt, os.Interrupt)

	go s.run()

	go s.listen()

	return s
}

func (s *client) ID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}

func (s *client) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *client) Close() error {
	err := s.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		if errors.Is(err, websocket.ErrCloseSent) {
			return nil
		}

		return err
	}

	return nil
}

func (s *client) Stream(ctx context.Context, msg string) {
	select {
	case <-ctx.Done():
		log.Info(ctx, "context of stream client cancelled")
		return

	case s.writeBuf <- msg:
	}
}

func (s *client) run() {
	for {
		select {
		case <-s.ctx.Done():
			s.closed = true
			if err := s.Close(); err != nil {
				log.Error(s.ctx, err)
			}
			return
		case <-s.interrupt:
			s.cancel()
			return
		case m := <-s.writeBuf:
			s.write([]byte(m))
		}
	}
}

func (s *client) write(msg []byte) {
	timeout := time.Second * 30
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

		switch msgType {
		case websocket.TextMessage:
			s.readBuf <- string(b)
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

var _ Client = (*client)(nil)
