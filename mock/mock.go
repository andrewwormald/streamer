package mock

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/require"
)

// WSChannel has the client side connection and it's ID refers to the ID the stream would use to reference it.
type WSChannel struct {
	ID   string
	Conn *websocket.Conn
}

// Acceptor is an acceptor interface for the NewChannelPool as to not be dependant on the streamer package
type Acceptor interface {
	Accept(w http.ResponseWriter, r *http.Request, channelKey string) error
}

// NewChannelPool returns a pool of channel side ws connections that get injected via the Accept method which Stream
// satisfies.
func NewChannelPool(t *testing.T, a Acceptor, connectionCount int) []WSChannel {
	var pool []WSChannel

	server := httptest.NewServer(http.HandlerFunc(simpleAcceptHandler(t, a)))
	for i := 0; i < connectionCount; i++ {
		id := uuid.New()
		u := "ws" + strings.TrimPrefix(server.URL, "http") + "?token=" + id
		conn, _, err := websocket.DefaultDialer.Dial(u, nil)
		require.Nil(t, err)
		pool = append(pool, WSChannel{
			ID:   id,
			Conn: conn,
		})
	}

	t.Cleanup(func() {
		server.Close()
	})

	return pool
}

func simpleAcceptHandler(t *testing.T, a Acceptor) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		keys := r.URL.Query()
		tk := keys.Get("token")

		err := a.Accept(w, r, tk)
		require.Nil(t, err)
	}
}
