package streamer_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"bitx/play/fusion/ops/streamer"
	"bitx/play/fusion/ops/streamer/mock"
)

func TestTopicManagement(t *testing.T) {
	s := streamer.NewChannel(nil, "1", streamer.WithoutAsyncFlush())
	topics := []string{"tweets", "buys", "sells"}
	for _, v := range topics {
		s.SetTopic(v)
		require.True(t, s.HasTopic(v))
		s.RemoveTopic(v)
		require.False(t, s.HasTopic(v))
	}
}

func TestAddMessageTimeout(t *testing.T) {
	stream := streamer.New()
	pool := mock.NewChannelPool(t, stream, 5)

	for _, ws := range pool {
		buffSize := 5
		c := streamer.NewChannel(ws.Conn, "test", streamer.WithWriteBufferSize(buffSize), streamer.WithoutAsyncFlush())
		messages := make([]int, 100)
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond*200))
		defer cancel()
		for k := range messages {
			err := c.Send(ctx, streamer.SendMessage{
				Message: "message",
			})
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
	stream := streamer.New()
	pool := mock.NewChannelPool(t, stream, 10)

	for _, ws := range pool {
		channel, err := stream.Collect(ws.ID)
		require.Nil(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
		defer cancel()
		err = channel.Send(ctx, streamer.SendMessage{
			Message: expectedMsg,
		})
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
	stream := streamer.New()
	pool := mock.NewChannelPool(t, stream, 5)

	for _, ws := range pool {
		channel, err := stream.Collect(ws.ID)
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
		channel, err := stream.Collect(ws.ID)
		require.Nil(t, err)
		require.True(t, channel.Closed())
	}
}

func TestReceive(t *testing.T) {
	stream := streamer.New(streamer.WithUpgrader(websocket.Upgrader{
		HandshakeTimeout: time.Millisecond,
	}))
	pool := mock.NewChannelPool(t, stream, 5)
	message := "hello from "
	go stream.Responder(func(m streamer.ReceiveMessage) {
		expected := message + m.ID
		require.Equal(t, expected, m.Message)
	})

	for _, ws := range pool {
		channel, err := stream.Collect(ws.ID)
		require.Nil(t, err)

		expectedMsg := message + ws.ID
		go channel.Recv(stream.Read())

		require.Nil(t, ws.Conn.WriteMessage(websocket.TextMessage, []byte(expectedMsg)))
	}
}

func TestReceiveArbMessageTypes(t *testing.T) {
	stream := streamer.New()
	pool := mock.NewChannelPool(t, stream, 5)
	go stream.Responder(func(m streamer.ReceiveMessage) {
		t.Fail()
	})

	for _, ws := range pool {
		channel, err := stream.Collect(ws.ID)
		require.Nil(t, err)

		expectedMsg := ws.ID
		go channel.Recv(stream.Read())

		require.Nil(t, ws.Conn.WriteMessage(websocket.BinaryMessage, []byte(expectedMsg)))
		require.Nil(t, ws.Conn.WriteControl(websocket.PingMessage, nil, time.Time{}))
		require.Nil(t, ws.Conn.WriteControl(websocket.PongMessage, nil, time.Time{}))
	}
}
