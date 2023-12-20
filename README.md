# Streamer

[![Build](https://github.com/andrewwormald/streamer/workflows/Go/badge.svg?branch=master)](https://github.com/andrewwormald/streamer/actions?query=workflow%3AGo)
[![Go Report Card](https://goreportcard.com/badge/github.com/andrewwormald/streamer)](https://goreportcard.com/report/github.com/andrewwormald/streamer)
[![codecov](https://codecov.io/gh/andrewwormald/streamer/branch/master/graph/badge.svg)](https://codecov.io/gh/andrewwormald/streamer)


- stream aims at providing a no hassle, quick, and easy integration of websockets for your Go server. 
- stream is built on top of https://github.com/gorilla/websocket.

- To start a new stream just do the following:
```go
package main

import "github.com/andrewwormald/streamer"

func main() {
    s := streamer.New() // Returns a new stream
}
```
## API
```go
package streamer

// Accept takes ownership of upgrading the HTTP server connection to the WebSocket protocol and adding the new connection
// to the Stream's channel pool.
func (s *Stream) Accept(w http.ResponseWriter, r *http.Request, channelKey string) error {}

// Publish sends a message to all of the open channels in the Stream's channel pool and takes channelTimeout which it uses to
// set a deadline per channel.
func (s *Stream) Publish(m SendMessage) {}

// Responder is blocking method that should be run in a goroutine for responding and handling received messages
func (s *Stream) Responder(handler func(m ReceiveMessage)) {}

// Read returns the streams read buffer that it consumes from for handling messages from the stream's channels
func (s *Stream) Read() chan ReceiveMessage {}

// Collect uses the provided channelID to fetch the channel
func (s *Stream) Collect(channelID string) (*Channel, error) {}

// Connections returns the amount of valid channels that are in the Stream.
func (s *Stream) Connections() int {}

```
### Options
```go
package streamer

func WithReadBufferSize(size int) StreamOption {
	return func(s *Stream) {
		s.readBuff = make(chan ReceiveMessage, size)
	}
}

func WithUpgrader(u websocket.Upgrader) StreamOption {
	return func(s *Stream) {
		s.u = u
	}
}
```

### Errors
```go
package streamer

var ErrChannelDoesNotExist = errors.New("channel does not exist", j.C("ERR_bcd404068d4f7f1b"))
```

# Upcoming

> Add go doc

> refactor to not use gorilla and increase memory efficiency
