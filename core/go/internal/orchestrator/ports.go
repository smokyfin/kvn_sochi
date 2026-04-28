package orchestrator

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
)

type SecretEndpoint struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func AllocateEndpoint(prefix string) (SecretEndpoint, error) {
	port, err := randomLoopbackPort()
	if err != nil {
		return SecretEndpoint{}, err
	}
	user, err := randomSecret(18)
	if err != nil {
		return SecretEndpoint{}, err
	}
	pass, err := randomSecret(32)
	if err != nil {
		return SecretEndpoint{}, err
	}
	return SecretEndpoint{
		Host:     "127.0.0.1",
		Port:     port,
		Username: fmt.Sprintf("%s_%s", prefix, user[:8]),
		Password: pass,
	}, nil
}

func randomLoopbackPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected address type %T", listener.Addr())
	}
	return tcpAddr.Port, nil
}

func randomSecret(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
