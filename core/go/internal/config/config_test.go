package config

import (
	"encoding/json"
	"testing"
)

func TestParseKeepsOnlySupportedFields(t *testing.T) {
	raw := []byte(`{
		"bridge_rsa_id": "rsa",
		"bridge_ed25519_id": "ed",
		"doh_server": "https://dns.google/dns-query",
		"doh_server_ip": "8.8.8.8",
		"outbounds": [{"tag":"proxy","protocol":"vless"}],
		"skip_arti": true,
		"inbounds": [{"ignored": true}],
		"routing": {"ignored": true}
	}`)

	cfg, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.BridgeRSAID != "rsa" || cfg.BridgeEd25519ID != "ed" {
		t.Fatalf("bridge IDs were not parsed: %#v", cfg)
	}
	if cfg.DoHServerIP != "8.8.8.8" || !cfg.SkipArti {
		t.Fatalf("optional fields were not parsed: %#v", cfg)
	}
	var outbounds []map[string]string
	if err := json.Unmarshal(cfg.Outbounds, &outbounds); err != nil {
		t.Fatalf("outbounds invalid: %v", err)
	}
	if got := outbounds[0]["protocol"]; got != "vless" {
		t.Fatalf("outbounds protocol = %q", got)
	}
}

func TestParseRequiresDoHURL(t *testing.T) {
	_, err := Parse([]byte(`{
		"bridge_rsa_id": "rsa",
		"bridge_ed25519_id": "ed",
		"doh_server": "1.1.1.1",
		"outbounds": []
	}`))
	if err == nil {
		t.Fatal("Parse() expected an error")
	}
}
