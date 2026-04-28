package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultURL = "https://incss.ru/vless.conf"

type Config struct {
	BridgeRSAID     string          `json:"bridge_rsa_id"`
	BridgeEd25519ID string          `json:"bridge_ed25519_id"`
	DoHServer       string          `json:"doh_server"`
	DoHServerIP     string          `json:"doh_server_ip,omitempty"`
	Outbounds       json.RawMessage `json:"outbounds"`
	SkipArti        bool            `json:"skip_arti,omitempty"`
}

type Provider struct {
	client     *http.Client
	defaultURL string
}

func NewProvider(client *http.Client, defaultURL string) Provider {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if defaultURL == "" {
		defaultURL = DefaultURL
	}
	return Provider{client: client, defaultURL: defaultURL}
}

func (p Provider) Load(ctx context.Context, source string) (Config, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		source = p.defaultURL
	}
	if isHTTPURL(source) {
		return p.loadURL(ctx, source)
	}
	return Parse([]byte(source))
}

func (p Provider) loadURL(ctx context.Context, rawURL string) (Config, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Config{}, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return Config{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Config{}, fmt.Errorf("config fetch failed: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Config{}, err
	}
	return Parse(body)
}

func Parse(raw []byte) (Config, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		var permissive map[string]json.RawMessage
		if unmarshalErr := json.Unmarshal(raw, &permissive); unmarshalErr != nil {
			return Config{}, err
		}
		cfg = Config{}
		if err := take(permissive, "bridge_rsa_id", &cfg.BridgeRSAID); err != nil {
			return Config{}, err
		}
		if err := take(permissive, "bridge_ed25519_id", &cfg.BridgeEd25519ID); err != nil {
			return Config{}, err
		}
		if err := take(permissive, "doh_server", &cfg.DoHServer); err != nil {
			return Config{}, err
		}
		if err := takeOptional(permissive, "doh_server_ip", &cfg.DoHServerIP); err != nil {
			return Config{}, err
		}
		if rawOutbounds, ok := permissive["outbounds"]; ok {
			cfg.Outbounds = append(json.RawMessage(nil), rawOutbounds...)
		}
		if err := takeOptional(permissive, "skip_arti", &cfg.SkipArti); err != nil {
			return Config{}, err
		}
	}

	if cfg.BridgeRSAID == "" {
		return Config{}, errors.New("bridge_rsa_id is required")
	}
	if cfg.BridgeEd25519ID == "" {
		return Config{}, errors.New("bridge_ed25519_id is required")
	}
	if cfg.DoHServer == "" {
		return Config{}, errors.New("doh_server is required")
	}
	if !isHTTPURL(cfg.DoHServer) {
		return Config{}, errors.New("doh_server must be an http or https URL")
	}
	if len(cfg.Outbounds) == 0 || !json.Valid(cfg.Outbounds) {
		return Config{}, errors.New("outbounds must be valid JSON")
	}
	return cfg, nil
}

func (c Config) XrayConfig(socksListen string, socksPort int, socksUser string, socksPass string) ([]byte, error) {
	type inbound struct {
		Tag      string         `json:"tag"`
		Listen   string         `json:"listen"`
		Port     int            `json:"port"`
		Protocol string         `json:"protocol"`
		Settings map[string]any `json:"settings"`
	}
	doc := map[string]any{
		"log": map[string]string{"loglevel": "warning"},
		"inbounds": []inbound{{
			Tag:      "arti-pt-socks",
			Listen:   socksListen,
			Port:     socksPort,
			Protocol: "socks",
			Settings: map[string]any{
				"udp":  true,
				"auth": "password",
				"accounts": []map[string]string{{
					"user": socksUser,
					"pass": socksPass,
				}},
			},
		}},
		"outbounds": json.RawMessage(c.Outbounds),
		"dns": map[string]any{
			"queryStrategy": "UseIP",
			"servers":       []string{c.DoHServer},
		},
	}
	return json.MarshalIndent(doc, "", "  ")
}

func take(values map[string]json.RawMessage, key string, dst any) error {
	raw, ok := values[key]
	if !ok {
		return fmt.Errorf("%s is required", key)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	return nil
}

func takeOptional(values map[string]json.RawMessage, key string, dst any) error {
	raw, ok := values[key]
	if !ok {
		return nil
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	return nil
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}
