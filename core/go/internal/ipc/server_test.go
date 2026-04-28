package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/smokyfin/kvn_sochi/core/internal/config"
	"github.com/smokyfin/kvn_sochi/core/internal/service"
)

func TestServerRejectsBadToken(t *testing.T) {
	dir := t.TempDir()
	manager, err := service.NewManager(service.Options{
		PrivateDir: filepath.Join(dir, "runtime"),
		TUNName:    "utun",
		TUNGateway: "127.0.0.1",
		Config: config.Config{
			BridgeRSAID:     "rsa",
			BridgeEd25519ID: "ed",
			DoHServer:       "https://dns.google/dns-query",
			Outbounds:       []byte(`[]`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(filepath.Join(dir, "ipc"), manager)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(ctx) }()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := net.DialTimeout("unix", server.SocketPath(), 10*time.Millisecond); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	conn, err := net.DialTimeout("unix", server.SocketPath(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"token":"bad","action":"status"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	var resp Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.OK || resp.Error != "unauthorized" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	cancel()
	<-errCh
}
