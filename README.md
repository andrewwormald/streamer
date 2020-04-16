# stream

- stream aims at providing a no hassle, quick, and easy integration of websockets for your Go server. 
- stream is not a complete package and will be extended in the future.
- stream is built on top of https://github.com/gorilla/websocket.

- To start a new stream just do the following:
```go
package main

import "github.com/SwiftySpartan/streamer"

func main() {
    s := streamer.New() // Returns a new stream
}
```

```go
package streamer

type Stream struct {
    // Accept takes ownership of upgrading the HTTP server connection to the WebSocket protocol and adding the new connection
    // to the stream's client pool.
    Accept(ctx context.Context, w http.ResponseWriter, r *http.Request, clientKey string) error
    
    // Exists uses the provided clientID to determine if the client is alive and part of the stream's client pool.
    Exists(clientKey string) bool

    // Publish sends a message to all of the open clients in the stream's client pool with a context.WithTimeout set to one
    // second to ensure the loop does not hang due to a client struggling to consume it's write buffer.
    Publish(message string) 

    // OpenConnections returns the amount of clients are in the client pool which are the same as open connections/
    OpenConnections() int
}
```
# Development roadmap

> over 90% code coverage

> finish consuming the clients read buffer in the stream

> refactor to not use gorilla and increase memory efficiency
