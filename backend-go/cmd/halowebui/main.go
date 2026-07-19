package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend-go/internal/server"
)

var version = "0.0.0-dev"

func main() {
	cfg, err := server.LoadConfig(version)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := server.CheckHealth(cfg.HealthURL(), 2*time.Second); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	debug.SetMemoryLimit(cfg.GoMemoryLimitBytes)
	app, err := server.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer app.Close()

	httpServer := &http.Server{
		Addr:              cfg.ListenAddress(),
		Handler:           app,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       75 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	shutdownSignal, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	go func() {
		<-shutdownSignal.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(ctx)
	}()

	fmt.Printf(
		"HaloWebUI slim %s listening on %s (Go heap limit %d MiB)\n",
		cfg.Version,
		cfg.ListenAddress(),
		cfg.GoMemoryLimitBytes/(1024*1024),
	)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
