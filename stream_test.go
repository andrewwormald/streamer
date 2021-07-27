package streamer

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/andrewwormald/streamer/mock"
	"github.com/stretchr/testify/require"
)

func TestCollect(t *testing.T) {
	ctx := context.TODO()
	s := New(ctx)
	pool := mock.NewChannelPool(t, s, 2)
	for _, ws := range pool {
		c, err := s.collect(ws.ID)
		require.Nil(t, err)
		require.False(t, c.Closed())
		require.Equal(t, ws.ID, c.ID())
	}
}

// TestPublish validates that Publish sends messages to all the channels in the stream and in the same order.
func TestPublish(t *testing.T) {
	ctx := context.TODO()
	s := New(ctx)
	pool := mock.NewChannelPool(t, s, 10)
	var wg sync.WaitGroup
	msgSendsCount := 100
	messages := make([]string, msgSendsCount)
	for _, ws := range pool {
		wg.Add(1)
		go func(ws mock.WSChannel) {
			var msgCount int
			for {
				_, b, err := ws.Conn.ReadMessage()
				require.Nil(t, err)

				expectedMsg := strconv.FormatInt(int64(msgCount), 10)
				require.Equal(t, expectedMsg, string(b))

				msgCount++

				if msgCount == msgSendsCount-1 {
					wg.Done()
					return
				}
			}
		}(ws)
	}

	for k := range messages {
		s.Publish(strconv.FormatInt(int64(k), 10))
	}

	wg.Wait()
}

// TestStreamPool tests the efficiency of handling large amount of channels (ws connections)
func TestStreamOrder(t *testing.T) {
	ctx := context.TODO()
	s := New(ctx)
	pool := mock.NewChannelPool(t, s, 1000)
	var wg sync.WaitGroup
	messages := make([]string, 5)
	for _, ws := range pool {
		wg.Add(1)
		go func(ws mock.WSChannel) {
			var msgCount int
			for {
				_, b, err := ws.Conn.ReadMessage()
				require.Nil(t, err)

				expectedMsg := strconv.FormatInt(int64(msgCount), 10)
				require.Equal(t, expectedMsg, string(b))

				msgCount++

				if msgCount == len(messages)-1 {
					wg.Done()
					return
				}
			}
		}(ws)
	}

	for k := range messages {
		s.Publish(strconv.FormatInt(int64(k), 10))
	}

	wg.Wait()
}

func TestConnectionsCount(t *testing.T) {
	ctx := context.TODO()
	s := New(ctx)
	connectionCount := 1000
	_ = mock.NewChannelPool(t, s, connectionCount)
	require.Equal(t, connectionCount, s.Connections())
}
