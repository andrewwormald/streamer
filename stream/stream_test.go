// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stream_test

import (
	"context"
	"github.com/stretchr/testify/require"
	"streamer/stream"
	"testing"

	"github.com/gorilla/websocket"
)

func TestStreamAccept(t *testing.T) {
	s := stream.New()
	clientID := "test_1"
	ctx := context.Background()

	s.Accept(ctx, &websocket.Conn{}, clientID)
	require.True(t, s.ClientExists(clientID))
}
