package tcp

import (
	"context"
	"net"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/tonge3199/redis_go/interface/tcp"
)

// Config stores tcp server properties
type Config struct {
	Address    string        `yaml:"address"`
	MaxConnect int           `yaml:"max_connect"`
	Timeout    time.Duration `yaml:"timeout"`
}

// ClientCounter Tracks the number of active client connections in current redis server(atomic for thread safety).
var ClientCounter int32

// ListenAndServeWithSignal binds port and handle requests, blocking until receive stop signal
//
// How it works:
//
//	Sets up a channel to receive OS signals (SIGINT, SIGTERM, etc.).
//	When a signal is received, it triggers server shutdown.
//	Calls net.Listen to start listening on the configured address.
//	Calls ListenAndServe to handle connections.
//
// Graceful Shutdown:
//
//	On receiving a shutdown signal or a fatal accept error:
//	Closes the listener (stops accepting new connections).
//	Calls handler.Close() to close all active connections.
//	Waits for all handler goroutines to finish.
func ListenAndServeWithSignal(cfg *Config, handler tcp.Handler) error {
	closeChan := make(chan struct{})
	sigCh := make(chan os.Signal)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP)

	go func() {
		sig := <-sigCh
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGHUP:
			closeChan <- struct{}{}
		}
	}()
	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return err
	}
	// cfg.Address = listener.Addr().String()
	// TODO:logger.Info()
	ListenAndServe(listener, handler, closeChan)
	return nil
}

// ListenAndServe : Accepts and handles incoming TCP connections until a shutdown signal is received.
//
// How it works:
//
//	Starts a goroutine to listen for shutdown signals or accept errors.
//
// In the main loop:
//
//	Accepts new connections.
//	Increments ClientCounter.
//	Spawns a goroutine for each connection to handle it via handler.Handle.
//	Decrements ClientCounter when the connection is done.
//
// Waits for all connection handlers to finish before exiting.
func ListenAndServe(listener net.Listener, handler tcp.Handler, closeChan <-chan struct{}) {
	errCh := make(chan error, 1)
	defer close(errCh)

	go func() {
		select {
		case <-closeChan:
			// TODO: logger.Info
		case err := <-errCh:
			// TODO
		}
		_ = listener.Close()
		_ = handler.Close()
	}()

	ctx := context.Background()
	var waitDone sync.WaitGroup
	for {
		conn, err := listener.Accept()
		if err != nil {
			// learn from net/http/serve.go#Serve()
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// ...
				time.Sleep(5 * time.Millisecond)
				continue
			}
			errCh <- err
			break
		}
		// handle
		// logger.Info("accept link")
		ClientCounter++
		waitDone.Add(1)
		go func() {
			defer func() {
				waitDone.Done()
				atomic.AddInt32(&ClientCounter, -1)
			}()
			handler.Handle(ctx, conn)
		}()
	}
	waitDone.Wait()
}
