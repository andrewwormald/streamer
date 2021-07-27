package streamer

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/andrewwormald/streamer/mock"
)

func TestID(t *testing.T) {
	id := "test"
	ctx := context.TODO()
	c := NewChannel(ctx,nil, id)
	require.Equal(t, id, c.ID())
}

func TestClosed(t *testing.T) {
	ctx := context.TODO()
	c := NewChannel(ctx,nil, "1")
	c.ctx, c.cancel = context.WithCancel(context.Background())
	require.False(t, c.Closed())
	c.cancel()
	require.True(t, c.Closed())
}

func TestInterruptClose(t *testing.T) {
	ctx := context.TODO()
	stream := New(ctx)
	pool := mock.NewChannelPool(t, stream, 1)
	for _, ws := range pool {
		channel, err := stream.collect(ws.ID)
		require.Nil(t, err)
		require.False(t, channel.Closed())
	}

	for _, ws := range pool {
		channel, err := stream.collect(ws.ID)
		channel.interrupt <- os.Interrupt
		require.Nil(t, err)
		time.Sleep(time.Millisecond * 10)
		require.True(t, channel.Closed())
	}
}

func TestRecvReturnWhenContextClosed(t *testing.T) {
	ctx := context.TODO()
	stream := New(ctx)
	pool := mock.NewChannelPool(t, stream, 1)
	time.Sleep(time.Millisecond * 100)
	for _, ws := range pool {
		channel, err := stream.collect(ws.ID)
		require.Nil(t, err)

		channel.cancel()
		channel.Recv(stream.Read())
	}
}

func TestWriteOnClosedConnection(t *testing.T) {
	ctx := context.TODO()
	stream := New(ctx)
	pool := mock.NewChannelPool(t, stream, 1)
	time.Sleep(time.Millisecond * 10)
	channel, err := stream.collect(pool[0].ID)
	require.Nil(t, err)
	err = channel.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	require.Nil(t, err)
	channel.write([]byte("test"))
	require.True(t, channel.Closed())
}


func TestAddMessageTimeout(t *testing.T) {
	ctx := context.TODO()
	stream := New(ctx)
	pool := mock.NewChannelPool(t, stream, 5)

	for _, ws := range pool {
		buffSize := 5
		c := NewChannel(ctx, ws.Conn, "test", WithWriteBufferSize(buffSize), WithoutAsyncFlush())
		messages := make([]int, 100)
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond*200))
		defer cancel()
		for k := range messages {
			err := c.Send(ctx, "message")
			if k < buffSize {
				require.Nil(t, err)
				continue
			}
			require.Equal(t, context.DeadlineExceeded, err)
		}
	}
}

func TestWriteFlushAndManage(t *testing.T) {
	var wg sync.WaitGroup
	expectedMsg := "hello channel"
	ctx := context.TODO()
	stream := New(ctx)
	pool := mock.NewChannelPool(t, stream, 10)

	for _, ws := range pool {
		channel, err := stream.collect(ws.ID)
		require.Nil(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
		defer cancel()
		err = channel.Send(ctx, expectedMsg)
		require.Nil(t, err)

		// Imitate channel
		wg.Add(1)
		go func() {
			_, msg, err := ws.Conn.ReadMessage()
			require.Nil(t, err)
			require.Equal(t, expectedMsg, string(msg))
			wg.Done()
		}()

		wg.Wait()
	}
}

func TestCloseConn(t *testing.T) {
	var wg sync.WaitGroup
	ctx := context.TODO()
	stream := New(ctx)
	pool := mock.NewChannelPool(t, stream, 5)

	for _, ws := range pool {
		channel, err := stream.collect(ws.ID)
		require.Nil(t, err)

		// Send close message and expected channel to not be alive
		channel.Close(websocket.CloseGoingAway)

		// Imitate channel
		wg.Add(1)
		go func(ws mock.WSChannel) {
			for {
				_, _, err := ws.Conn.ReadMessage()
				require.NotNil(t, err)
				require.Equal(t, "websocket: close 1001 (going away)", err.Error())
				wg.Done()
				return
			}
		}(ws)
	}

	wg.Wait()
	for _, ws := range pool {
		channel, err := stream.collect(ws.ID)
		require.Nil(t, err)
		require.True(t, channel.Closed())
	}
}

func TestReceive(t *testing.T) {
	ctx := context.TODO()
	stream := New(ctx, WithUpgrader(websocket.Upgrader{
		HandshakeTimeout: time.Millisecond,
	}))
	pool := mock.NewChannelPool(t, stream, 5)
	message := "hello from "
	go stream.Responder(func(m ReceiveMessage) {
		expected := message + m.ChannelID
		require.Equal(t, expected, m.Message)
	})

	for _, ws := range pool {
		channel, err := stream.collect(ws.ID)
		require.Nil(t, err)

		expectedMsg := message + ws.ID
		go channel.Recv(stream.Read())

		require.Nil(t, ws.Conn.WriteMessage(websocket.TextMessage, []byte(expectedMsg)))
	}
}

func TestReceiveArbMessageTypes(t *testing.T) {
	ctx := context.TODO()
	stream := New(ctx)
	pool := mock.NewChannelPool(t, stream, 5)
	go stream.Responder(func(m ReceiveMessage) {
		t.Fail()
	})

	for _, ws := range pool {
		channel, err := stream.collect(ws.ID)
		require.Nil(t, err)

		expectedMsg := ws.ID
		go channel.Recv(stream.Read())

		require.Nil(t, ws.Conn.WriteMessage(websocket.BinaryMessage, []byte(expectedMsg)))
		require.Nil(t, ws.Conn.WriteControl(websocket.PingMessage, nil, time.Time{}))
		require.Nil(t, ws.Conn.WriteControl(websocket.PongMessage, nil, time.Time{}))
	}
}

