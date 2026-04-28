package ipc

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/smokyfin/kvn_sochi/core/internal/service"
)

type Server struct {
	privateDir string
	socketPath string
	tokenPath  string
	token      string
	listener   net.Listener
	manager    *service.Manager
}

type Request struct {
	Token  string `json:"token"`
	Action string `json:"action"`
}

type Response struct {
	OK     bool           `json:"ok"`
	Status service.Status `json:"status"`
	Error  string         `json:"error,omitempty"`
}

func NewServer(privateDir string, manager *service.Manager) (*Server, error) {
	if err := os.MkdirAll(privateDir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(privateDir, 0o700); err != nil {
		return nil, err
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	socketPath := filepath.Join(privateDir, "kvn-core.sock")
	tokenPath := filepath.Join(privateDir, "kvn-core.token")
	if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0o600); err != nil {
		return nil, err
	}
	return &Server{
		privateDir: privateDir,
		socketPath: socketPath,
		tokenPath:  tokenPath,
		token:      token,
		manager:    manager,
	}, nil
}

func (s *Server) Serve(ctx context.Context) error {
	_ = os.Remove(s.socketPath)
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return err
	}
	if err := os.Chmod(s.socketPath, 0o600); err != nil {
		listener.Close()
		return err
	}
	s.listener = listener
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return ctx.Err()
			}
			return err
		}
		go s.handle(conn)
	}
}

func (s *Server) Close() error {
	if s.listener != nil {
		_ = s.listener.Close()
	}
	_ = os.Remove(s.socketPath)
	return os.Remove(s.tokenPath)
}

func (s *Server) SocketPath() string {
	return s.socketPath
}

func (s *Server) TokenPath() string {
	return s.tokenPath
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		_ = json.NewEncoder(conn).Encode(Response{OK: false, Error: err.Error()})
		return
	}
	var req Request
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &req); err != nil {
		_ = json.NewEncoder(conn).Encode(Response{OK: false, Error: err.Error()})
		return
	}
	if req.Token != s.token {
		_ = json.NewEncoder(conn).Encode(Response{OK: false, Error: "unauthorized"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var actionErr error
	switch req.Action {
	case "connect":
		actionErr = s.manager.Connect(ctx)
	case "disconnect":
		actionErr = s.manager.Disconnect(ctx)
	case "status":
	default:
		actionErr = errors.New("unsupported action")
	}
	resp := Response{OK: actionErr == nil, Status: s.manager.Status()}
	if actionErr != nil {
		resp.Error = actionErr.Error()
	}
	_ = json.NewEncoder(conn).Encode(resp)
}

func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
