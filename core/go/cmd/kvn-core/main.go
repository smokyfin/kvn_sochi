package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/smokyfin/kvn_sochi/core/internal/config"
	"github.com/smokyfin/kvn_sochi/core/internal/ipc"
	"github.com/smokyfin/kvn_sochi/core/internal/service"
)

func main() {
	privateDir := flag.String("private-dir", "", "private app directory for sockets, generated configs, and runtime files")
	configSource := flag.String("config", config.DefaultURL, "configuration URL or JSON string")
	tunName := flag.String("tun", "utun", "platform TUN interface name")
	tunGateway := flag.String("tun-gateway", "10.88.0.1", "local TUN gateway IP used by DNS")
	flag.Parse()

	if *privateDir == "" {
		slog.Error("private-dir is required")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	provider := config.NewProvider(http.DefaultClient, config.DefaultURL)
	parsed, err := provider.Load(ctx, *configSource)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	manager, err := service.NewManager(service.Options{
		PrivateDir: *privateDir,
		TUNName:    *tunName,
		TUNGateway: *tunGateway,
		Config:     parsed,
	})
	if err != nil {
		slog.Error("create manager", "error", err)
		os.Exit(1)
	}
	defer manager.Close()

	server, err := ipc.NewServer(*privateDir, manager)
	if err != nil {
		slog.Error("create ipc server", "error", err)
		os.Exit(1)
	}
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(ctx)
	}()

	slog.Info("kvn core ready", "socket", server.SocketPath(), "token_file", server.TokenPath())

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("ipc server failed", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := manager.Disconnect(shutdownCtx); err != nil {
		slog.Warn("disconnect during shutdown", "error", err)
	}
}
