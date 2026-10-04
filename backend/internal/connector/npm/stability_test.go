package npm_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
	_ "github.com/WiseLabz/wiselabz/internal/connector/npm"
	"github.com/WiseLabz/wiselabz/internal/store"
	syncdiff "github.com/WiseLabz/wiselabz/internal/sync"
)

const stabilityConfigKey = "YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE="

func TestFetchStableAcrossReorderingAndSnapshotJSON(t *testing.T) {
	resourcePaths := []string{
		"/api/nginx/proxy-hosts",
		"/api/nginx/redirection-hosts",
		"/api/nginx/streams",
		"/api/nginx/dead-hosts",
		"/api/nginx/certificates",
		"/api/nginx/access-lists",
	}
	resources := [2]map[string]string{
		{
			resourcePaths[0]: `[{"id":1,"domain_names":["single.example.test"],"forward_host":"192.0.2.10","forward_port":8080,"forward_scheme":"http","enabled":true},{"id":2,"domain_names":["app.example.test","www.example.test","api.example.test"],"forward_host":"app.internal.test","forward_port":3000,"forward_scheme":"https","enabled":true}]`,
			resourcePaths[1]: `[{"id":11,"domain_names":["old.example.test"],"forward_http_code":301,"forward_scheme":"https","forward_domain_name":"app.example.test","enabled":true},{"id":12,"domain_names":["legacy.example.test"],"forward_http_code":302,"forward_scheme":"https","forward_domain_name":"www.example.test","enabled":true}]`,
			resourcePaths[2]: `[{"id":21,"incoming_port":2222,"forwarding_host":"192.0.2.20","forwarding_port":22,"tcp_forwarding":true,"enabled":true},{"id":22,"incoming_port":25565,"forwarding_host":"minecraft.internal.test","forwarding_port":25565,"tcp_forwarding":true,"udp_forwarding":true,"enabled":true}]`,
			resourcePaths[3]: `[{"id":31,"domain_names":["gone.example.test"],"enabled":true},{"id":32,"domain_names":["retired.example.test"],"enabled":false}]`,
			resourcePaths[4]: `[{"id":41,"provider":"letsencrypt","nice_name":"example.test","domain_names":["example.test","www.example.test"],"expires_on":"2030-01-01T00:00:00Z","meta":{"dns_challenge":true,"dns_provider":"test","dns_provider_credentials":"private-cert-credential-a"}},{"id":42,"provider":"other","nice_name":"internal","domain_names":["internal.test"],"expires_on":"2031-01-01T00:00:00Z","meta":{"dns_provider_credentials":"private-cert-credential-a"}}]`,
			resourcePaths[5]: `[{"id":51,"name":"internal","satisfy_any":false,"pass_auth":false,"proxy_host_count":1},{"id":52,"name":"staff","satisfy_any":true,"pass_auth":true,"proxy_host_count":2}]`,
		},
		{
			resourcePaths[0]: `[{"id":2,"domain_names":["app.example.test","api.example.test","www.example.test"],"forward_host":"app.internal.test","forward_port":3000,"forward_scheme":"https","enabled":true},{"id":1,"domain_names":["single.example.test"],"forward_host":"192.0.2.10","forward_port":8080,"forward_scheme":"http","enabled":true}]`,
			resourcePaths[1]: `[{"id":12,"domain_names":["legacy.example.test"],"forward_http_code":302,"forward_scheme":"https","forward_domain_name":"www.example.test","enabled":true},{"id":11,"domain_names":["old.example.test"],"forward_http_code":301,"forward_scheme":"https","forward_domain_name":"app.example.test","enabled":true}]`,
			resourcePaths[2]: `[{"id":22,"incoming_port":25565,"forwarding_host":"minecraft.internal.test","forwarding_port":25565,"tcp_forwarding":true,"udp_forwarding":true,"enabled":true},{"id":21,"incoming_port":2222,"forwarding_host":"192.0.2.20","forwarding_port":22,"tcp_forwarding":true,"enabled":true}]`,
			resourcePaths[3]: `[{"id":32,"domain_names":["retired.example.test"],"enabled":false},{"id":31,"domain_names":["gone.example.test"],"enabled":true}]`,
			resourcePaths[4]: `[{"id":42,"provider":"other","nice_name":"internal","domain_names":["internal.test"],"expires_on":"2031-01-01T00:00:00Z","meta":{"dns_provider_credentials":"private-cert-credential-b"}},{"id":41,"provider":"letsencrypt","nice_name":"example.test","domain_names":["www.example.test","example.test"],"expires_on":"2030-01-01T00:00:00Z","meta":{"dns_challenge":false,"dns_provider":"changed","dns_provider_credentials":"private-cert-credential-b"}}]`,
			resourcePaths[5]: `[{"id":52,"name":"staff","satisfy_any":true,"pass_auth":true,"proxy_host_count":2},{"id":51,"name":"internal","satisfy_any":false,"pass_auth":false,"proxy_host_count":1}]`,
		},
	}
	tokens := 0
	gets := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tokens" {
			if r.Method != http.MethodPost {
				t.Errorf("token endpoint method = %s, want POST", r.Method)
				http.Error(w, "token requests must use POST", http.StatusMethodNotAllowed)
				return
			}
			tokens++
			var credentials map[string]string
			if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil || credentials["identity"] != "npm@example.test" || credentials["secret"] != "npm-secret" {
				http.Error(w, "invalid credentials", http.StatusUnauthorized)
				return
			}
			_, _ = fmt.Fprintf(w, `{"token":"fresh-token-%d"}`, tokens)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/nginx/") && r.Method != http.MethodGet {
			t.Errorf("resource %s method = %s, want GET", r.URL.Path, r.Method)
			http.Error(w, "resource requests must use GET", http.StatusMethodNotAllowed)
			return
		}
		gets++
		if r.Header.Get("Authorization") != fmt.Sprintf("Bearer fresh-token-%d", tokens) {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		variant := 0
		if tokens >= 3 {
			variant = 1
		}
		body, ok := resources[variant][r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	connector.AllowLoopbackForTest(t)
	config := map[string]any{"url": server.URL, "email": "npm@example.test", "password": "npm-secret", "verify_tls": true}
	created, err := connector.Get("npm", config)
	if err != nil {
		t.Fatalf("connector.Get: %v", err)
	}
	prev, err := created.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("first Fetch: %v", err)
	}
	repeated, err := created.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	curr, err := created.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatalf("reordered Fetch: %v", err)
	}
	if tokens != 3 || gets != 3*len(resourcePaths) {
		t.Fatalf("token requests / resource requests = %d / %d, want 3 / %d", tokens, gets, 3*len(resourcePaths))
	}
	if len(prev.Entities) == 0 || len(prev.Dependencies) == 0 {
		t.Fatalf("fixture should exercise entities and dependencies: %d entities, %d dependencies", len(prev.Entities), len(prev.Dependencies))
	}
	assertNoSnapshotChanges(t, "identical direct snapshots", prev, repeated)
	assertNoSnapshotChanges(t, "reordered direct snapshots", prev, curr)

	prevJSON, err := json.Marshal(prev)
	if err != nil {
		t.Fatalf("marshal first snapshot: %v", err)
	}
	repeatedJSON, err := json.Marshal(repeated)
	if err != nil {
		t.Fatalf("marshal repeated snapshot: %v", err)
	}
	currJSON, err := json.Marshal(curr)
	if err != nil {
		t.Fatalf("marshal reordered snapshot: %v", err)
	}
	if strings.Contains(string(prevJSON), "private-cert-credential") || strings.Contains(string(repeatedJSON), "private-cert-credential") || strings.Contains(string(currJSON), "private-cert-credential") {
		t.Fatal("snapshot JSON exposed certificate metadata credentials")
	}
	var persistedPrev, persistedRepeated, persistedCurr connector.ServiceSnapshot
	if err := json.Unmarshal(prevJSON, &persistedPrev); err != nil {
		t.Fatalf("unmarshal first snapshot: %v", err)
	}
	if err := json.Unmarshal(repeatedJSON, &persistedRepeated); err != nil {
		t.Fatalf("unmarshal repeated snapshot: %v", err)
	}
	if err := json.Unmarshal(currJSON, &persistedCurr); err != nil {
		t.Fatalf("unmarshal reordered snapshot: %v", err)
	}
	assertNoSnapshotChanges(t, "JSON round-trip identical snapshots", &persistedPrev, &persistedRepeated)
	assertNoSnapshotChanges(t, "JSON round-trip reordered snapshots", &persistedPrev, &persistedCurr)
}

func assertNoSnapshotChanges(t *testing.T, label string, prev, curr *connector.ServiceSnapshot) {
	t.Helper()
	if changes := syncdiff.Compare(prev, curr); len(changes) != 0 {
		t.Errorf("%s section changes = %+v, want none", label, changes)
	}
	if changes := syncdiff.CompareEntities(prev.Entities, curr.Entities); len(changes) != 0 {
		t.Errorf("%s entity changes = %+v, want none", label, changes)
	}
	if changes := syncdiff.CompareDependencies(prev.Dependencies, curr.Dependencies); len(changes) != 0 {
		t.Errorf("%s dependency changes = %+v, want none", label, changes)
	}
}

func TestNPMPasswordIsEncryptedAtRest(t *testing.T) {
	config := map[string]any{
		"url":        "https://npm.example.test",
		"email":      "npm@example.test",
		"password":   "npm-at-rest-secret",
		"verify_tls": true,
	}
	stored, err := store.MarshalConnectorConfig("npm", config, stabilityConfigKey)
	if err != nil {
		t.Fatalf("MarshalConnectorConfig: %v", err)
	}
	if strings.Contains(stored, config["password"].(string)) {
		t.Fatalf("stored config contains plaintext password: %s", stored)
	}
	key, err := base64.StdEncoding.DecodeString(stabilityConfigKey)
	if err != nil || len(key) != 32 {
		t.Fatalf("test encryption key must decode to 32 bytes, got %d bytes, %v", len(key), err)
	}
	decoded, err := store.ParseConnectorConfig("npm", stored, stabilityConfigKey)
	if err != nil {
		t.Fatalf("ParseConnectorConfig: %v", err)
	}
	if decoded["password"] != config["password"] {
		t.Fatalf("decrypted password = %v, want original", decoded["password"])
	}
}
