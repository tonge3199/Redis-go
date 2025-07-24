package tcp

import (
	"context"
	"net"
)

// HandlerFunc represents application handler function
type HandlerFunc func(ctx context.Context, conn net.Conn)

type Handler interface {
	Handle(ctx context.Context, conn net.Conn)
	Close() error
}
