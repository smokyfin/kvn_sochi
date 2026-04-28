package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/smokyfin/kvn_sochi/core/internal/config"
	"github.com/smokyfin/kvn_sochi/core/internal/supervisor"
)

type Options struct {
	PrivateDir string
	TUNName    string
	TUNGateway string
	Config     config.Config
	Binaries   Binaries
}

type Binaries struct {
	HevSocks5Tunnel string
	Arti            string
	Xray            string
}

type Runtime struct {
	opts        Options
	TUNSocks    SecretEndpoint `json:"tun_socks"`
	ArtiSocks   SecretEndpoint `json:"arti_socks"`
	XSocks      SecretEndpoint `json:"xray_socks"`
	processes   []*supervisor.Process
	generated   []string
	trafficFlow []string
}

func NewRuntime(opts Options) (*Runtime, error) {
	if opts.PrivateDir == "" {
		return nil, fmt.Errorf("private directory is required")
	}
	tunSocks, err := AllocateEndpoint("tun")
	if err != nil {
		return nil, err
	}
	artiSocks, err := AllocateEndpoint("arti")
	if err != nil {
		return nil, err
	}
	xSocks, err := AllocateEndpoint("xray")
	if err != nil {
		return nil, err
	}
	flow := []string{"TUN", "hev-socks5-tunnel", "Arti", "xray-core", "Internet"}
	if opts.Config.SkipArti {
		flow = []string{"TUN", "hev-socks5-tunnel", "xray-core", "Internet"}
	}
	return &Runtime{
		opts:        opts,
		TUNSocks:    tunSocks,
		ArtiSocks:   artiSocks,
		XSocks:      xSocks,
		trafficFlow: flow,
	}, nil
}

func (r *Runtime) Start(ctx context.Context) error {
	if err := os.MkdirAll(r.opts.PrivateDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(r.opts.PrivateDir, 0o700); err != nil {
		return err
	}
	paths, err := r.writeConfigs()
	if err != nil {
		return err
	}

	if r.opts.Binaries.Xray != "" {
		proc, err := supervisor.Start(ctx, "xray-core", r.opts.Binaries.Xray, []string{"run", "-config", paths.Xray}, nil, r.opts.PrivateDir)
		if err != nil {
			return err
		}
		r.processes = append(r.processes, proc)
	}
	if !r.opts.Config.SkipArti && r.opts.Binaries.Arti != "" {
		proc, err := supervisor.Start(ctx, "arti", r.opts.Binaries.Arti, []string{"proxy", "-c", paths.Arti}, nil, r.opts.PrivateDir)
		if err != nil {
			return err
		}
		r.processes = append(r.processes, proc)
	}
	if r.opts.Binaries.HevSocks5Tunnel != "" {
		proc, err := supervisor.Start(ctx, "hev-socks5-tunnel", r.opts.Binaries.HevSocks5Tunnel, []string{paths.Hev}, nil, r.opts.PrivateDir)
		if err != nil {
			return err
		}
		r.processes = append(r.processes, proc)
	}
	return nil
}

func (r *Runtime) Stop(ctx context.Context) error {
	var firstErr error
	for i := len(r.processes) - 1; i >= 0; i-- {
		if err := r.processes[i].Stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	r.processes = nil
	for _, path := range r.generated {
		_ = os.Remove(path)
	}
	r.generated = nil
	return firstErr
}

func (r *Runtime) Flow() []string {
	return append([]string(nil), r.trafficFlow...)
}

type ConfigPaths struct {
	Hev  string
	Arti string
	Xray string
}

func (r *Runtime) writeConfigs() (ConfigPaths, error) {
	xrayBytes, err := r.opts.Config.XrayConfig(r.XSocks.Host, r.XSocks.Port, r.XSocks.Username, r.XSocks.Password)
	if err != nil {
		return ConfigPaths{}, err
	}

	target := r.XSocks
	if !r.opts.Config.SkipArti {
		target = r.ArtiSocks
	}
	hev := map[string]any{
		"tun": map[string]any{
			"name":    r.opts.TUNName,
			"address": r.opts.TUNGateway,
			"mtu":     8500,
		},
		"socks5": map[string]any{
			"address":  target.Host,
			"port":     target.Port,
			"username": target.Username,
			"password": target.Password,
			"udp":      true,
		},
		"misc": map[string]any{
			"task-stack-size": 24576,
			"connect-timeout": 8000,
		},
	}
	hevBytes, err := json.MarshalIndent(hev, "", "  ")
	if err != nil {
		return ConfigPaths{}, err
	}

	arti := fmt.Sprintf(`[application]
allow_running_as_root = false

[proxy]
socks_listen = "%s:%d"

[storage]
state_dir = "%s/arti-state"
cache_dir = "%s/arti-cache"

[bridges]
enabled = true
bridges = [
  "Bridge obfs4 0.0.0.0:1 %s cert=%s iat-mode=0",
]

[bridges.transports.obfs4]
path = "xray-core"
arguments = ["run", "-config", "%s"]
`, r.ArtiSocks.Host, r.ArtiSocks.Port, r.opts.PrivateDir, r.opts.PrivateDir, r.opts.Config.BridgeRSAID, r.opts.Config.BridgeEd25519ID, filepath.Join(r.opts.PrivateDir, "xray.json"))

	paths := ConfigPaths{
		Hev:  filepath.Join(r.opts.PrivateDir, "hev-socks5-tunnel.json"),
		Arti: filepath.Join(r.opts.PrivateDir, "arti.toml"),
		Xray: filepath.Join(r.opts.PrivateDir, "xray.json"),
	}
	for path, bytes := range map[string][]byte{
		paths.Hev:  hevBytes,
		paths.Arti: []byte(arti),
		paths.Xray: xrayBytes,
	} {
		if err := os.WriteFile(path, bytes, 0o600); err != nil {
			return ConfigPaths{}, err
		}
		r.generated = append(r.generated, path)
	}
	return paths, nil
}
