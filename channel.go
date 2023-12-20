package streamer

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/log"
)

const defaultWriteBuffSize = 5

// Channel is the abstraction of the websocket connection. You interact with the websocket connection via the Channel
// struct and all clients are represented as a Channel as soon as they are accepted into the stream.
type Channel struct {
	mu *sync.Mutex
	id string

	subs map[string]bool

	writeBuf  chan string
	interrupt chan os.Signal

	ctx    context.Context
	cancel context.CancelFunc
	conn   *websocket.Conn

	asyncFlush bool
}

// ChannelOption provides the ability to configure the Channel to your own specification
type ChannelOption func(c *Channel)

// NewChannel provides a new Channel that the wraps the websocket connection and allows Stream to easily interface with
// it. NewChannel by default runs a an additional go routine to manage and flush all the writes to the connection.
func NewChannel(ctx context.Context, wc *websocket.Conn, id string, opts ...ChannelOption) *Channel {
	ctx, cancel := context.WithCancel(ctx)
	c := &Channel{
		mu:         &sync.Mutex{},
		id:         id,
		subs:       make(map[string]bool),
		writeBuf:   make(chan string, defaultWriteBuffSize),
		interrupt:  make(chan os.Signal, 1),
		ctx:        ctx,
		cancel:     cancel,
		conn:       wc,
		asyncFlush: true,
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.asyncFlush {
		go c.flushWriteAndManage()
	}

	signal.Notify(c.interrupt, os.Interrupt)

	return c
}

// WithoutAsyncFlush means that the Channel will not send any messages it is send and depending on the WriteBufferSize
// will wait for the first message to be consumed. This is largely used for tests and not intended to be used in
// production.
func WithoutAsyncFlush() ChannelOption {
	return func(c *Channel) {
		c.asyncFlush = false
	}
}

// WithWriteBufferSize changes the size of the underlying write buffer. The write buffer size correlates to the number
// of messages and not the actual size of the messages. Therefore a buffer size of 1 would allow 1 message to be queued
// at a time.
func WithWriteBufferSize(size int) ChannelOption {
	return func(c *Channel) {
		c.writeBuf = make(chan string, size)
	}
}

// ID returns the id of the channel that the stream uses as its key in the channel pool.
func (c *Channel) ID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.id
}

// Closed returns true if the Channel can no longer be interacted with.
func (c *Channel) Closed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ctx.Err() != nil
}

// Close sends a close connection message to the channel's underlying websocket connection. Regardless if this is
// successful the channel's context will be cancelled which results in this channel being marked as closed. Close takes
// a Close code defined in RFC 6455, section 11.7.
func (c *Channel) Close(closeCode int) {
	c.cancel()

	c.mu.Lock()
	err := c.conn.WriteMessage(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(closeCode, ""))
	c.mu.Unlock()
	if errors.Is(err, websocket.ErrCloseSent) {
		// NoReturnErr: Connection already closed, ensure context is still cancelled by not returning
	} else if err != nil {
		// NoReturnErr: Connection tainted and marked for closing
		log.Error(c.ctx, err)
	}
}

// Send pushes a string to the channel's write buffer. If the buffer is full this method will hang until the
// context is cancelled or until items are removed from the writeBuf.
//
// Send must be provided with a context.WithTimeout() or context.WithDeadline() as to prevent Send from becoming a
// blocking method that is dependant on it's consumption of the write buffer.
//
// Send returns a context error if the context's Done is closed.
func (c *Channel) Send(ctx context.Context, msg string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.writeBuf <- msg:
	}
	return nil
}

// Recv is a blocking method that waits for a websocket Text message from the channel and progresses to write to the
// parameter Go channel. If the provided Go channel is full for more that 200 milliseconds an error will be logged and
// will retry when the next message is read.
func (c *Channel) Recv(ch chan ReceiveMessage) {
	for {
		if c.ctx.Err() != nil {
			return
		}

		msgType, b, err := c.conn.ReadMessage()
		if err != nil {
			// NoReturnErr: Once error occurs the same error will be returned and thus needed to close the connection
			// and cancel the context of the channel (github.com/gorilla/websocket@v1.4.1/conn.go:969)
			c.Close(websocket.CloseInternalServerErr)
			log.Error(c.ctx, err)
			return
		}

		switch msgType {
		case websocket.TextMessage:
			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
			select {
			case <-ctx.Done():
				log.Error(ctx, errors.New("context closed before chance to read message from channel"))
			case ch <- ReceiveMessage{
				ChannelID: c.ID(),
				Message:   string(b),
			}:
			}
			cancel()
		case websocket.BinaryMessage:
			// Channel sending unsupported message type (https://tools.ietf.org/html/rfc6455#section-7.4)
			c.Close(websocket.CloseUnsupportedData)
		default:
			continue
		}
	}
}

// flushWriteAndManage is a blocking method that continuously flushes the write buffer and writes to the channel over the
// websocket's tcp connection. flushWriteAndManage will also handle system interruptions and the channel's context to
// minimise the amount of goroutines per channel.
func (c *Channel) flushWriteAndManage() {
	for {
		if c.ctx.Err() != nil {
			return
		}

		select {
		case <-c.interrupt:
			c.Close(websocket.CloseServiceRestart)
			return
		case m := <-c.writeBuf:
			c.write([]byte(m))
		}
	}
}

// write entails a writing deadline and writes a text message to the channel. If the operation fails the channel's
// context is cancelled and an error is logged.
func (c *Channel) write(msg []byte) {
	timeout := time.Second
	err := c.conn.SetWriteDeadline(time.Now().Add(timeout))
	if err != nil {
		// NoReturnErr: SetWriteDeadline never returns an error (github.com/gorilla/websocket@v1.4.1/conn.go:780)
		log.Error(c.ctx, err)
	}

	c.mu.Lock()
	err = c.conn.WriteMessage(websocket.TextMessage, msg)
	c.mu.Unlock()
	if err != nil {
		// NoReturnErr: Connection closed and will be removed by sweeper
		if !errors.Is(err, websocket.ErrCloseSent) {
			log.Error(c.ctx, err)
		}
		c.Close(websocket.CloseGoingAway)
	}
}
