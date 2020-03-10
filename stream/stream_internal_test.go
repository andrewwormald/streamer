// Copyright 2015 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stream

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"streamer/client"
)

func TestStore(t *testing.T) {
	s := New()
	clientID := "conn_1"
	c := client.NewMock(nil, clientID)
	s.store(c)

	_, exists := s.pool[clientID]
	assert.True(t, exists)
}

