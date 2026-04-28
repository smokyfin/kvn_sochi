package dns

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	mdns "github.com/miekg/dns"
)

type Server struct {
	addr        string
	dohURL      string
	dohServerIP string
	server      *mdns.Server
	client      *http.Client
}

func NewServer(addr string, dohURL string, dohServerIP string) *Server {
	return &Server{
		addr:        addr,
		dohURL:      dohURL,
		dohServerIP: dohServerIP,
		client:      &http.Client{Timeout: 8 * time.Second},
	}
}

func (s *Server) Start() error {
	mux := mdns.NewServeMux()
	mux.HandleFunc(".", s.handleDNS)
	s.server = &mdns.Server{Addr: s.addr, Net: "udp", Handler: mux}
	go func() {
		_ = s.server.ListenAndServe()
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- s.server.Shutdown() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) handleDNS(writer mdns.ResponseWriter, request *mdns.Msg) {
	response, err := s.forward(request)
	if err != nil {
		response = new(mdns.Msg)
		response.SetRcode(request, mdns.RcodeServerFailure)
	}
	_ = writer.WriteMsg(response)
}

func (s *Server) forward(query *mdns.Msg) (*mdns.Msg, error) {
	wire, err := query.Pack()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, s.dohURL+"?dns="+base64.RawURLEncoding.EncodeToString(wire), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/dns-message")
	if s.dohServerIP != "" {
		req.Host = hostOnly(req.URL.Host)
		req.URL.Host = net.JoinHostPort(s.dohServerIP, "443")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("doh status: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return nil, err
	}
	msg := new(mdns.Msg)
	if err := msg.Unpack(body); err != nil {
		return nil, err
	}
	return msg, nil
}

func hostOnly(hostPort string) string {
	host, _, err := net.SplitHostPort(hostPort)
	if err == nil {
		return host
	}
	return hostPort
}
