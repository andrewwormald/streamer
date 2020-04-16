package streamer

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStore tests the streams internal store functionality
func TestStore(t *testing.T) {
	s := New()
	clientID := "conn_1"

	c := newMockClient(nil, clientID)
	s.store(c)

	_, exists := s.pool[clientID]
	require.True(t, exists)
}

// TestStore tests the streams internal store functionality under concurrent stress
func TestStoreConcurrency(t *testing.T) {
	var wg sync.WaitGroup

	s := New()
	ls := make([]int32, 100)
	for k, _ := range ls {
		wg.Add(1)
		id := makeConnID(k)
		c := newMockClient(nil, id)

		go func() {
			s.store(c)
			wg.Done()
		}()
	}

	wg.Wait()
	for k, _ := range ls {
		id := makeConnID(k)
		_, exists := s.pool[id]
		require.True(t, exists)
	}
}

func TestRemove(t *testing.T) {
	var wg sync.WaitGroup

	s := New()
	ls := make([]int32, 100)
	// Add
	for k, _ := range ls {
		id := makeConnID(k)
		c := newMockClient(nil, id)
		s.store(c)
	}

	// Validate
	for k, _ := range ls {
		id := makeConnID(k)
		_, exists := s.pool[id]
		require.True(t, exists)
	}

	// Remove
	for k, _ := range ls {
		wg.Add(1)
		id := makeConnID(k)
		c := newMockClient(nil, id)

		go func() {
			s.remove(c)
			wg.Done()
		}()
	}

	wg.Wait()
	// Validate
	for k, _ := range ls {
		id := makeConnID(k)
		_, exists := s.pool[id]
		require.False(t, exists)
	}
}

func makeConnID(i int) string {
	return fmt.Sprintf("conn_%v", i)
}
