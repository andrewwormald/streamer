package streamer

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStreamBufferSize(t *testing.T) {
	ctx := context.TODO()
	stream := New(ctx, WithReadBufferSize(1))
	expectedMessage := "hello client "
	go stream.Responder(func(m ReceiveMessage) {
		expected := expectedMessage + m.ChannelID
		require.Equal(t, expected, m.Message)
	})

	for i := 0; i < 10; i++ {
		id := strconv.FormatInt(int64(i), 10)
		stream.readBuff <- ReceiveMessage{
			ChannelID: id,
			Message:   expectedMessage + id,
		}
	}
}

// TestStore tests the streams internal store functionality
func TestStore(t *testing.T) {
	ctx := context.TODO()
	s := New(ctx)
	channelID := "conn_1"

	c := &Channel{
		mu:         &sync.Mutex{},
		id:         channelID,
		subs:       make(map[string]bool),
		writeBuf:   make(chan string, defaultWriteBuffSize),
		interrupt:  make(chan os.Signal, 1),
		ctx:        ctx,
		asyncFlush: true,
	}
	s.store(c)

	_, exists := s.pool[channelID]
	require.True(t, exists)
}

// TestStore tests the streams internal store functionality under concurrent stress
func TestStoreConcurrency(t *testing.T) {
	var wg sync.WaitGroup

	ctx := context.TODO()
	s := New(ctx)
	ls := make([]int32, 100)
	for k := range ls {
		wg.Add(1)
		id := makeConnID(k)
		c := &Channel{
			mu:         &sync.Mutex{},
			id:         id,
			subs:       make(map[string]bool),
			writeBuf:   make(chan string, defaultWriteBuffSize),
			interrupt:  make(chan os.Signal, 1),
			ctx:        ctx,
			asyncFlush: true,
		}

		go func() {
			s.store(c)
			wg.Done()
		}()
	}

	wg.Wait()
	for k := range ls {
		id := makeConnID(k)
		_, exists := s.pool[id]
		require.True(t, exists)
	}
}

func TestRemove(t *testing.T) {
	var wg sync.WaitGroup

	ctx := context.TODO()
	s := New(ctx)
	ls := make([]int32, 100)
	// Add
	for k := range ls {
		id := makeConnID(k)
		c := &Channel{
			mu:         &sync.Mutex{},
			id:         id,
			subs:       make(map[string]bool),
			writeBuf:   make(chan string, defaultWriteBuffSize),
			interrupt:  make(chan os.Signal, 1),
			ctx:        ctx,
			asyncFlush: true,
		}
		s.store(c)
	}

	// Validate
	for k := range ls {
		id := makeConnID(k)
		_, exists := s.pool[id]
		require.True(t, exists)
	}

	// Remove
	for k := range ls {
		wg.Add(1)
		id := makeConnID(k)
		c := &Channel{
			mu:         &sync.Mutex{},
			id:         id,
			subs:       make(map[string]bool),
			writeBuf:   make(chan string, defaultWriteBuffSize),
			interrupt:  make(chan os.Signal, 1),
			ctx:        ctx,
			asyncFlush: true,
		}

		go func() {
			s.remove(c)
			wg.Done()
		}()
	}

	wg.Wait()
	// Validate
	for k := range ls {
		id := makeConnID(k)
		_, exists := s.pool[id]
		require.False(t, exists)
	}
}

func makeConnID(i int) string {
	return fmt.Sprintf("conn_%v", i)
}
