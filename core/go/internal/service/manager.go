package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/smokyfin/kvn_sochi/core/internal/config"
	"github.com/smokyfin/kvn_sochi/core/internal/dns"
	"github.com/smokyfin/kvn_sochi/core/internal/orchestrator"
)

type State string

const (
	StateIdle       State = "Idle"
	StateConnecting State = "Connecting"
	StateConnected  State = "Connected"
)

type Options struct {
	PrivateDir string
	TUNName    string
	TUNGateway string
	Config     config.Config
	Binaries   orchestrator.Binaries
}

type Status struct {
	State       State    `json:"state"`
	Flow        []string `json:"flow"`
	SkipArti    bool     `json:"skip_arti"`
	LastError   string   `json:"last_error,omitempty"`
	ConnectedAt string   `json:"connected_at,omitempty"`
}

type Manager struct {
	mu         sync.Mutex
	opts       Options
	state      State
	lastError  string
	connected  time.Time
	cancel     context.CancelFunc
	runtime    *orchestrator.Runtime
	dnsServer  *dns.Server
	statusPath string
}

func NewManager(opts Options) (*Manager, error) {
	if opts.PrivateDir == "" {
		return nil, errors.New("private dir is required")
	}
	if err := os.MkdirAll(opts.PrivateDir, 0o700); err != nil {
		return nil, err
	}
	return &Manager{
		opts:       opts,
		state:      StateIdle,
		statusPath: filepath.Join(opts.PrivateDir, "status.json"),
	}, nil
}

func (m *Manager) Connect(ctx context.Context) error {
	m.mu.Lock()
	if m.state == StateConnected || m.state == StateConnecting {
		m.mu.Unlock()
		return nil
	}
	m.state = StateConnecting
	m.lastError = ""
	runCtx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.mu.Unlock()

	runtime, err := orchestrator.NewRuntime(orchestrator.Options{
		PrivateDir: m.opts.PrivateDir,
		TUNName:    m.opts.TUNName,
		TUNGateway: m.opts.TUNGateway,
		Config:     m.opts.Config,
		Binaries:   m.opts.Binaries,
	})
	if err != nil {
		m.failConnecting(cancel, err)
		return err
	}

	dnsAddr := net.JoinHostPort(m.opts.TUNGateway, "53")
	dnsServer := dns.NewServer(dnsAddr, m.opts.Config.DoHServer, m.opts.Config.DoHServerIP)
	if err := dnsServer.Start(); err != nil {
		m.failConnecting(cancel, err)
		return err
	}
	if err := runtime.Start(runCtx); err != nil {
		_ = dnsServer.Shutdown(ctx)
		m.failConnecting(cancel, err)
		return err
	}

	m.mu.Lock()
	m.runtime = runtime
	m.dnsServer = dnsServer
	m.state = StateConnected
	m.connected = time.Now().UTC()
	m.mu.Unlock()
	return m.writeStatus()
}

func (m *Manager) Disconnect(ctx context.Context) error {
	m.mu.Lock()
	cancel := m.cancel
	runtime := m.runtime
	dnsServer := m.dnsServer
	m.cancel = nil
	m.runtime = nil
	m.dnsServer = nil
	m.state = StateIdle
	m.connected = time.Time{}
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	var firstErr error
	if runtime != nil {
		if err := runtime.Stop(ctx); err != nil {
			firstErr = err
		}
	}
	if dnsServer != nil {
		if err := dnsServer.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := m.writeStatus(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	status := Status{
		State:     m.state,
		SkipArti:  m.opts.Config.SkipArti,
		LastError: m.lastError,
	}
	if m.runtime != nil {
		status.Flow = m.runtime.Flow()
	}
	if !m.connected.IsZero() {
		status.ConnectedAt = m.connected.Format(time.RFC3339)
	}
	return status
}

func (m *Manager) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return m.Disconnect(ctx)
}

func (m *Manager) failConnecting(cancel context.CancelFunc, err error) {
	cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = StateIdle
	m.lastError = err.Error()
}

func (m *Manager) writeStatus() error {
	status := m.Status()
	bytes := []byte(fmt.Sprintf("{\"state\":%q,\"skip_arti\":%t,\"last_error\":%q,\"connected_at\":%q}\n", status.State, status.SkipArti, status.LastError, status.ConnectedAt))
	return os.WriteFile(m.statusPath, bytes, 0o600)
}
