package streamer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	c := newMockClient(nil, "test")
	require.NotNil(t, c)
}

func TestID(t *testing.T) {
	id := "test"
	c := newMockClient(nil, id)
	require.Equal(t, id, c.ID())
}

func TestAlive(t *testing.T) {
	id := "test"
	c := newMockClient(nil, id)
	require.True(t, c.Alive())
	c.closed = true
	require.False(t, c.Alive())
}
