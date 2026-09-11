package api

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

// Server provides an echo server.
type Server struct {
	Echo *echo.Echo
	Addr string
}

// NewServer creates and configures an APIServer serving all application routes.
func NewServer() (*Server, error) {
	log.Println("configuring server...")
	e, err := New(viper.GetBool("enable_cors"))
	if err != nil {
		return nil, err
	}

	var addr string
	port := viper.GetString("port")

	// allow port to be set as localhost:3000 in env during development to avoid "accept incoming network connection" request on restarts
	if strings.Contains(port, ":") {
		addr = port
	} else {
		addr = ":" + port
	}

	return &Server{Echo: e, Addr: addr}, nil
}

// Start runs the echo server with graceful shutdown.
func (srv *Server) Start() {
	log.Println("starting server...")
	go func() {
		if err := srv.Echo.Start(srv.Addr); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()
	log.Printf("Listening on %s\n", srv.Addr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	sig := <-quit
	log.Println("Shutting down server... Reason:", sig)
	// teardown logic...

	if err := srv.Echo.Shutdown(context.Background()); err != nil {
		panic(err)
	}
	log.Println("Server gracefully stopped")
}
