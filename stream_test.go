package streamer_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/andrewwormald/streamer"
)

// TestExists validates the stream's ability to accept, return open connection count, and determine if a connection
// with a specified ID exists.
func TestExists(t *testing.T) {
	s := streamer.New()
	server := httptest.NewServer(http.HandlerFunc(acceptHandler(t, s)))
	defer server.Close()

	tc := []struct {
		name     string
		id       string
		expected int
	}{
		{
			name:     "First connection, expect single connection",
			id:       "1",
			expected: 1,
		},
		{
			name:     "Second connection, invalid as id is taken",
			id:       "1",
			expected: 1,
		},
		{
			name:     "Third connection, expect 2 connections",
			id:       "3",
			expected: 2,
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			url := server.URL + "?token=" + tt.id
			u := "ws" + strings.TrimPrefix(url, "http")

			_, _, err := websocket.DefaultDialer.Dial(u, nil)
			require.Nil(t, err)

			time.Sleep(time.Millisecond)
			require.True(t, s.Exists(tt.id))
			require.Equal(t, tt.expected, s.OpenConnections())
		})
	}
}

// TestPublish validates that Publish sends messages to all the clients in the stream and in the same order.
func TestPublish(t *testing.T) {
	s := streamer.New()

	server := httptest.NewServer(http.HandlerFunc(acceptHandler(t, s)))
	defer server.Close()

	tc := []struct {
		name string
		id   string
	}{
		{
			name: "First connection",
			id:   "1",
		},
		{
			name: "Second Connection",
			id:   "2",
		},
	}

	var wg sync.WaitGroup

	msgSendsCount := 100
	messages := make([]string, msgSendsCount)
	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			url := server.URL + "?token=" + tt.id
			u := "ws" + strings.TrimPrefix(url, "http")

			w, _, err := websocket.DefaultDialer.Dial(u, nil)
			require.Nil(t, err)

			wg.Add(1)
			go func() {
				var msgCount int
				for {
					_, b, err := w.ReadMessage()
					require.Nil(t, err)

					expectedMsg := strconv.FormatInt(int64(msgCount), 10)
					require.Equal(t, expectedMsg, string(b))

					msgCount++

					if msgCount == msgSendsCount-1 {
						wg.Done()
						return
					}
				}
			}()
		})
	}

	for k, _ := range messages {
		m := strconv.FormatInt(int64(k), 10)
		s.Publish(m)
	}

	wg.Wait()
}

// TestStreamPool tests the efficiency of handling large amount of clients (ws connections).
func TestStreamPoolStressTest(t *testing.T) {
	s := streamer.New()
	server := httptest.NewServer(http.HandlerFunc(acceptHandler(t, s)))

	var wg sync.WaitGroup

	clients := make([]string, 1000)
	messages := make([]string, 5)
	for i, _ := range clients {
		index := strconv.FormatInt(int64(i), 10)
		url := server.URL + "?token=" + index
		u := "ws" + strings.TrimPrefix(url, "http")

		w, _, err := websocket.DefaultDialer.Dial(u, nil)
		require.Nil(t, err)

		wg.Add(1)

		go func() {
			var msgCount int
			for {
				_, b, err := w.ReadMessage()
				require.Nil(t, err)

				expectedMsg := strconv.FormatInt(int64(msgCount), 10)
				require.Equal(t, expectedMsg, string(b))

				msgCount++

				if msgCount == len(messages)-1 {
					wg.Done()
					return
				}
			}
		}()
	}

	for k, _ := range messages {
		m := strconv.FormatInt(int64(k), 10)
		s.Publish(m)
	}

	wg.Wait()
	server.Close()
}

func acceptHandler(t *testing.T, s *streamer.Stream) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		keys := r.URL.Query()
		tk := keys.Get("token")

		err := s.Accept(r.Context(), w, r, tk)
		require.Nil(t, err)
	}
}
