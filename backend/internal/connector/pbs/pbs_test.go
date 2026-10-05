package pbs_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	_ "github.com/WiseLabz/wiselabz/internal/connector/pbs"
	"github.com/WiseLabz/wiselabz/internal/store"
)

const pbsTokenSecretKey = "YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE="

func TestRegistrationAndIdentity(t *testing.T) {
	created, err := connector.Get("pbs", nil)
	if err != nil {
		t.Fatalf("empty config factory: %v", err)
	}
	if created.Name() != "Proxmox Backup Server" || created.Type() != "pbs" || created.Category() != "virtualization" {
		t.Fatalf("identity = %s/%s/%s", created.Name(), created.Type(), created.Category())
	}
	schema, err := connector.GetTypeSchema("pbs")
	if err != nil {
		t.Fatalf("GetTypeSchema: %v", err)
	}
	if schema.Name != "Proxmox Backup Server" || schema.Category != "virtualization" {
		t.Fatalf("schema identity = %+v", schema)
	}
	fields := map[string]connector.SchemaField{}
	for _, field := range schema.Fields {
		fields[field.Key] = field
	}
	for _, key := range []string{"url", "token_id", "token_secret"} {
		if !fields[key].Required {
			t.Errorf("%s should be required", key)
		}
	}
	if fields["token_secret"].Type != "password" {
		t.Errorf("token_secret type = %s, want password", fields["token_secret"].Type)
	}
	if fields["verify_tls"].Default != "true" {
		t.Errorf("verify_tls default = %s, want true", fields["verify_tls"].Default)
	}
	if _, ok := connector.AttributeCatalog()["pbs"]; !ok {
		t.Errorf("attribute catalog missing for pbs")
	}
}

func TestValidateRequiresConfigurationAndAuthenticates(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	for _, config := range []map[string]any{
		nil,
		{"token_id": "root@pam!monitoring", "token_secret": "secret"},
		{"url": "http://example.test", "token_secret": "secret"},
		{"url": "http://example.test", "token_id": "root@pam!monitoring"},
	} {
		created, err := connector.Get("pbs", config)
		if err != nil {
			t.Fatalf("connector.Get: %v", err)
		}
		if err := created.Validate(context.Background(), nil); err == nil {
			t.Errorf("Validate(%v) succeeded without required fields", config)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "PBSAPIToken=root@pam!monitoring:test-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprintf(w, `{"data":[]}`)
	}))
	defer server.Close()

	config := map[string]any{"url": server.URL, "token_id": "root@pam!monitoring", "token_secret": "test-secret"}
	created, err := connector.Get("pbs", config)
	if err != nil {
		t.Fatalf("connector.Get: %v", err)
	}
	if err := created.Validate(context.Background(), nil); err != nil {
		t.Fatalf("valid Validate: %v", err)
	}
}

func TestVerifyTLSDefaultsTrueAndCanBeDisabled(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"data":[]}`)
	}))
	defer server.Close()

	connector.AllowLoopbackForTest(t)
	created, err := connector.Get("pbs", map[string]any{
		"url": server.URL, "token_id": "root@pam!monitoring", "token_secret": "secret",
	})
	if err != nil {
		t.Fatalf("connector.Get: %v", err)
	}
	if err := created.Validate(context.Background(), nil); err == nil {
		t.Fatal("default TLS verification accepted a self-signed certificate")
	}

	created, err = connector.Get("pbs", map[string]any{
		"url": server.URL, "token_id": "root@pam!monitoring", "token_secret": "secret",
		"verify_tls": false,
	})
	if err != nil {
		t.Fatalf("connector.Get with verify_tls=false: %v", err)
	}
	if err := created.Validate(context.Background(), nil); err != nil {
		t.Fatalf("verify_tls=false Validate: %v", err)
	}
}

func TestPBSTokenSecretIsEncryptedAtRest(t *testing.T) {
	config := map[string]any{
		"url":          "https://pbs.example.test:8007",
		"token_id":     "root@pam!monitoring",
		"token_secret": "pbs-secret-at-rest",
		"verify_tls":   true,
	}
	stored, err := store.MarshalConnectorConfig("pbs", config, pbsTokenSecretKey)
	if err != nil {
		t.Fatalf("MarshalConnectorConfig: %v", err)
	}
	if strings.Contains(stored, config["token_secret"].(string)) {
		t.Fatalf("stored config contains plaintext token_secret: %s", stored)
	}
	key, err := base64.StdEncoding.DecodeString(pbsTokenSecretKey)
	if err != nil || len(key) != 32 {
		t.Fatalf("test encryption key must decode to 32 bytes, got %d bytes, %v", len(key), err)
	}
	decoded, err := store.ParseConnectorConfig("pbs", stored, pbsTokenSecretKey)
	if err != nil {
		t.Fatalf("ParseConnectorConfig: %v", err)
	}
	if decoded["token_secret"] != config["token_secret"] {
		t.Fatalf("decrypted token_secret = %v, want original", decoded["token_secret"])
	}
}
