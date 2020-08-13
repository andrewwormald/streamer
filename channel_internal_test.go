package streamer

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/andrewwormald/streamer/mock"
)

func TestID(t *testing.T) {
	id := "test"
	c := NewChannel(nil, id)
	require.Equal(t, id, c.ID())
}

func TestClosed(t *testing.T) {
	c := NewChannel(nil, "1")
	c.ctx, c.cancel = context.WithCancel(context.Background())
	require.False(t, c.Closed())
	c.cancel()
	require.True(t, c.Closed())
}

func TestInterruptClose(t *testing.T) {
	stream := New()
	pool := mock.NewChannelPool(t, stream, 1)
	for _, ws := range pool {
		channel, err := stream.Collect(ws.ID)
		require.Nil(t, err)
		require.False(t, channel.Closed())
	}

	for _, ws := range pool {
		channel, err := stream.Collect(ws.ID)
		channel.interrupt <- os.Interrupt
		require.Nil(t, err)
		time.Sleep(time.Millisecond * 10)
		require.True(t, channel.Closed())
	}
}

func TestRecvReturnWhenContextClosed(t *testing.T) {
	stream := New()
	pool := mock.NewChannelPool(t, stream, 1)
	for _, ws := range pool {
		channel, err := stream.Collect(ws.ID)
		require.Nil(t, err)

		channel.cancel()

		// Blocking func
		channel.Recv(stream.Read())
	}
}

func TestWriteOnClosedConnection(t *testing.T) {
	stream := New()
	pool := mock.NewChannelPool(t, stream, 1)
	time.Sleep(time.Millisecond * 10)
	channel, err := stream.Collect(pool[0].ID)
	require.Nil(t, err)
	err = channel.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	require.Nil(t, err)
	channel.write([]byte("test"))
	require.True(t, channel.Closed())
}
