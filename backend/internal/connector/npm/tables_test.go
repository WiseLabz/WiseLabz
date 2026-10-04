package npm

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func TestBuildersOnEmptyAndMalformedInput(t *testing.T) {
	builders := map[string]tableBuilder{
		"proxy hosts":       buildProxyHostTable,
		"redirection hosts": buildRedirectionHostTable,
		"streams":           buildStreamTable,
		"404 hosts":         buildDeadHostTable,
		"certificates":      buildCertificateTable,
		"access lists":      buildAccessListTable,
	}
	for noun, build := range builders {
		t.Run(noun+" empty", func(t *testing.T) {
			content, entities, deps, err := build([]byte("[]"))
			if err != nil || len(entities) != 0 || len(deps) != 0 || !strings.Contains(content, "_No ") {
				t.Fatalf("empty build = (%q, %d entities, %d dependencies, %v)", content, len(entities), len(deps), err)
			}
		})
		for _, raw := range []string{"not json", `{"id":1}`, `null`, `[`} {
			t.Run(noun+" malformed", func(t *testing.T) {
				content, entities, deps, err := build([]byte(raw))
				if err == nil || len(entities) != 0 || len(deps) != 0 || !strings.Contains(content, "malformed response") {
					t.Fatalf("malformed build = (%q, %d entities, %d dependencies, %v)", content, len(entities), len(deps), err)
				}
				if strings.Contains(err.Error(), "secret") {
					t.Fatalf("error exposed upstream data: %v", err)
				}
			})
		}
	}
}

func TestBuildProxyHostTable(t *testing.T) {
	content, entities, deps, err := buildProxyHostTable([]byte(proxyHostsFixture))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, `primary.example\|edge`) {
		t.Errorf("table did not escape pipe: %s", content)
	}
	if len(entities) != 3 || entities[0].ExternalID != "2" || entities[0].Name != "primary.example|edge" || entities[0].Hostname != "primary.example|edge" {
		t.Fatalf("unexpected proxy identities: %+v", entities)
	}
	if !reflect.DeepEqual(entities[0].Aliases, []string{"a.example", "z.example"}) {
		t.Errorf("aliases = %#v", entities[0].Aliases)
	}
	if entities[0].IP != "2001:db8::1" || entities[1].IP != "" || entities[2].IP != "10.0.0.8" {
		t.Errorf("proxy IP mapping = %q, %q, %q", entities[0].IP, entities[1].IP, entities[2].IP)
	}
	if !reflect.DeepEqual(deps, []connector.ServiceDependency{{Kind: "upstream_service", Name: "backend.internal"}}) {
		t.Errorf("dependencies = %#v", deps)
	}
	want := map[string]any{"domain_names": []string{"a.example", "primary.example|edge", "z.example"}, "forward_host": "2001:db8::1", "forward_port": 8443, "forward_scheme": "https", "access_list_id": 4, "certificate_id": 8, "ssl_forced": true, "enabled": true}
	if !reflect.DeepEqual(entities[0].Attributes, want) {
		t.Errorf("attributes = %#v, want %#v", entities[0].Attributes, want)
	}
	if strings.Contains(content, "advanced_config") || strings.Contains(content, "secret") {
		t.Errorf("sensitive upstream data appeared in table: %s", content)
	}
}

func TestOtherBuildersMapUpstreamFields(t *testing.T) {
	content, redirects, _, err := buildRedirectionHostTable([]byte(redirectFixture))
	if err != nil || len(redirects) != 2 {
		t.Fatalf("redirect build: %v %+v", err, redirects)
	}
	if redirects[0].Name != "go.example" || !reflect.DeepEqual(redirects[0].Aliases, []string{"www.go.example"}) || redirects[0].Attributes["forward_http_code"] != 301 {
		t.Errorf("redirect mapping: %+v", redirects[0])
	}
	if !strings.Contains(content, "https://target.example") {
		t.Errorf("redirect table missing target: %s", content)
	}

	_, streams, deps, err := buildStreamTable([]byte(streamFixture))
	if err != nil || len(streams) != 2 {
		t.Fatalf("stream build: %v %+v", err, streams)
	}
	if streams[0].IP != "192.0.2.2" || streams[1].IP != "" {
		t.Errorf("stream IP mapping: %+v", streams)
	}
	if !reflect.DeepEqual(deps, []connector.ServiceDependency{{Kind: "upstream_service", Name: "stream.backend"}}) {
		t.Errorf("stream dependencies: %#v", deps)
	}

	_, dead, _, err := buildDeadHostTable([]byte(deadHostsFixture))
	if err != nil || len(dead) != 2 || dead[0].Hostname != "gone.example" {
		t.Errorf("dead host mapping: %v %+v", err, dead)
	}

	certContent, certs, _, err := buildCertificateTable([]byte(certificatesFixture))
	if err != nil || len(certs) != 2 || certs[0].Name != "Wildcard cert" || certs[0].Attributes["expires_on"] != "2026-11-15T04:17:54.000Z" {
		t.Errorf("certificate mapping: %v %+v", err, certs)
	}
	if !strings.Contains(certContent, "2026-11-15T04:17:54.000Z") {
		t.Errorf("certificate table lost reported timestamp: %s", certContent)
	}

	_, lists, _, err := buildAccessListTable([]byte(accessListsFixture))
	if err != nil || len(lists) != 2 || lists[0].Name != "Staff" || lists[0].Attributes["proxy_host_count"] != 3 {
		t.Errorf("access list mapping: %v %+v", err, lists)
	}
	encoded, _ := json.Marshal(lists[0])
	if strings.Contains(string(encoded), "password") || strings.Contains(string(encoded), "items") {
		t.Errorf("access list entity exposed credentials: %s", encoded)
	}
}

func TestBuilderFixturesMapEntityIdentityAndAttributes(t *testing.T) {
	cases := []struct {
		name          string
		build         tableBuilder
		raw           string
		kind, id, key string
		value         any
	}{
		{"proxy hosts", buildProxyHostTable, proxyHostsFixture, "proxy_host", "2", "forward_port", 8443},
		{"redirection hosts", buildRedirectionHostTable, redirectFixture, "redirection_host", "5", "forward_domain_name", "target.example"},
		{"streams", buildStreamTable, streamFixture, "stream", "1", "incoming_port", 53},
		{"404 hosts", buildDeadHostTable, deadHostsFixture, "dead_host", "7", "certificate_id", 3},
		{"certificates", buildCertificateTable, certificatesFixture, "certificate", "4", "expires_on", "2026-11-15T04:17:54.000Z"},
		{"access lists", buildAccessListTable, accessListsFixture, "access_list", "6", "proxy_host_count", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, entities, _, err := tc.build([]byte(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			if len(entities) == 0 {
				t.Fatal("fixture produced no entities")
			}
			entity := entities[0]
			if entity.Kind != tc.kind || entity.ExternalID != tc.id {
				t.Errorf("identity = %q/%q, want %q/%q", entity.Kind, entity.ExternalID, tc.kind, tc.id)
			}
			if got := entity.Attributes[tc.key]; !reflect.DeepEqual(got, tc.value) {
				t.Errorf("Attributes[%q] = %#v, want %#v", tc.key, got, tc.value)
			}
		})
	}
}

func TestBuildersDeterministicAndPreservePrimaryDomain(t *testing.T) {
	first := []byte(`[ {"id":9,"domain_names":["primary.test","z.test","a.test"],"forward_host":"upstream.test","forward_port":80,"forward_scheme":"http","access_list_id":0,"certificate_id":0,"ssl_forced":false,"enabled":true}, {"id":1,"domain_names":["other.test"],"forward_host":"10.1.0.1","forward_port":8080,"forward_scheme":"http","access_list_id":0,"certificate_id":0,"ssl_forced":false,"enabled":true} ]`)
	second := []byte(`[ {"id":1,"domain_names":["other.test"],"forward_host":"10.1.0.1","forward_port":8080,"forward_scheme":"http","access_list_id":0,"certificate_id":0,"ssl_forced":false,"enabled":true}, {"id":9,"domain_names":["primary.test","a.test","z.test"],"forward_host":"upstream.test","forward_port":80,"forward_scheme":"http","access_list_id":0,"certificate_id":0,"ssl_forced":false,"enabled":true} ]`)
	c1, e1, d1, err := buildProxyHostTable(first)
	if err != nil {
		t.Fatal(err)
	}
	c2, e2, d2, err := buildProxyHostTable(second)
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 || !reflect.DeepEqual(e1, e2) || !reflect.DeepEqual(d1, d2) {
		t.Fatalf("reordered resources changed output\nfirst: %s\nsecond: %s", c1, c2)
	}
	if e1[1].Name != "primary.test" || !reflect.DeepEqual(e1[1].Aliases, []string{"a.test", "z.test"}) {
		t.Errorf("primary/aliases = %q / %#v", e1[1].Name, e1[1].Aliases)
	}
}

func TestEveryBuilderIsDeterministic(t *testing.T) {
	fixtures := []struct {
		name  string
		build tableBuilder
		raw   string
	}{
		{"proxy hosts", buildProxyHostTable, proxyHostsFixture},
		{"redirection hosts", buildRedirectionHostTable, redirectFixture},
		{"streams", buildStreamTable, streamFixture},
		{"404 hosts", buildDeadHostTable, deadHostsFixture},
		{"certificates", buildCertificateTable, certificatesFixture},
		{"access lists", buildAccessListTable, accessListsFixture},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			firstContent, firstEntities, firstDeps, err := fixture.build([]byte(fixture.raw))
			if err != nil {
				t.Fatal(err)
			}
			var resources []json.RawMessage
			if err := json.Unmarshal([]byte(fixture.raw), &resources); err != nil {
				t.Fatal(err)
			}
			for left, right := 0, len(resources)-1; left < right; left, right = left+1, right-1 {
				resources[left], resources[right] = resources[right], resources[left]
			}
			reordered, err := json.Marshal(resources)
			if err != nil {
				t.Fatal(err)
			}
			secondContent, secondEntities, secondDeps, err := fixture.build(reordered)
			if err != nil {
				t.Fatal(err)
			}
			if firstContent != secondContent || !reflect.DeepEqual(firstEntities, secondEntities) || !reflect.DeepEqual(firstDeps, secondDeps) {
				t.Errorf("resource order changed output\nfirst: %s\nsecond: %s", firstContent, secondContent)
			}
		})
	}
}

func TestSetLikeDomainsAndDependenciesAreCanonical(t *testing.T) {
	certA := []byte(`[{"id":1,"provider":"letsencrypt","domain_names":["b.example","a.example"],"expires_on":"2026-01-01T00:00:00Z"}]`)
	certB := []byte(`[{"id":1,"provider":"letsencrypt","domain_names":["a.example","b.example"],"expires_on":"2026-01-01T00:00:00Z"}]`)
	c1, e1, _, err := buildCertificateTable(certA)
	if err != nil {
		t.Fatal(err)
	}
	c2, e2, _, err := buildCertificateTable(certB)
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 || !reflect.DeepEqual(e1, e2) {
		t.Fatalf("certificate domain order changed output: %s / %s", c1, c2)
	}

	proxyA := []byte(`[{"id":1,"domain_names":["primary.example","z.example","a.example"],"forward_host":"backend.internal","forward_port":80,"forward_scheme":"http"}]`)
	proxyB := []byte(`[{"id":1,"domain_names":["primary.example","a.example","z.example"],"forward_host":"backend.internal","forward_port":80,"forward_scheme":"http"}]`)
	pc1, pe1, pd1, err := buildProxyHostTable(proxyA)
	if err != nil {
		t.Fatal(err)
	}
	pc2, pe2, pd2, err := buildProxyHostTable(proxyB)
	if err != nil {
		t.Fatal(err)
	}
	if pc1 != pc2 || !reflect.DeepEqual(pe1, pe2) || !reflect.DeepEqual(pd1, pd2) {
		t.Fatal("alias remainder order changed proxy output")
	}

	duplicates := []byte(`[{"id":1,"domain_names":["one.example"],"forward_host":"z.backend","forward_port":80},{"id":2,"domain_names":["two.example"],"forward_host":"a.backend","forward_port":80},{"id":3,"domain_names":["three.example"],"forward_host":"z.backend","forward_port":80}]`)
	_, _, deps, err := buildProxyHostTable(duplicates)
	if err != nil {
		t.Fatal(err)
	}
	want := []connector.ServiceDependency{{Kind: "upstream_service", Name: "a.backend"}, {Kind: "upstream_service", Name: "z.backend"}}
	if !reflect.DeepEqual(deps, want) {
		t.Errorf("dependencies = %#v, want %#v", deps, want)
	}
}

const proxyHostsFixture = `[
 {"id":8,"domain_names":["ip.example"],"forward_host":"10.0.0.8","forward_port":80,"forward_scheme":"http","access_list_id":0,"certificate_id":0,"ssl_forced":false,"enabled":false,"advanced_config":"secret config","meta":{"secret":"certificate key"}},
 {"id":2,"domain_names":["primary.example|edge","z.example","a.example","z.example"],"forward_host":"2001:db8::1","forward_port":8443,"forward_scheme":"https","access_list_id":4,"certificate_id":8,"ssl_forced":true,"enabled":true,"advanced_config":"secret config","meta":{"secret":"certificate key"}},
 {"id":3,"domain_names":["app.example"],"forward_host":"backend.internal","forward_port":3000,"forward_scheme":"http","access_list_id":0,"certificate_id":0,"ssl_forced":false,"enabled":true}
]`

const redirectFixture = `[{"id":9,"domain_names":["later.example"],"forward_http_code":302,"forward_scheme":"http","forward_domain_name":"other.example","preserve_path":false,"certificate_id":0,"ssl_forced":false,"enabled":false},{"id":5,"domain_names":["go.example","www.go.example"],"forward_http_code":301,"forward_scheme":"https","forward_domain_name":"target.example","preserve_path":true,"certificate_id":2,"ssl_forced":true,"enabled":true,"advanced_config":"secret"}]`
const streamFixture = `[{"id":2,"incoming_port":3306,"forwarding_host":"stream.backend","forwarding_port":3306,"tcp_forwarding":true,"udp_forwarding":false,"certificate_id":0,"enabled":true},{"id":1,"incoming_port":53,"forwarding_host":"192.0.2.2","forwarding_port":53,"tcp_forwarding":false,"udp_forwarding":true,"certificate_id":0,"enabled":true}]`
const deadHostsFixture = `[{"id":10,"domain_names":["extra.example"],"certificate_id":0,"ssl_forced":false,"enabled":false},{"id":7,"domain_names":["gone.example"],"certificate_id":3,"ssl_forced":false,"enabled":true}]`
const certificatesFixture = `[{"id":9,"provider":"other","nice_name":"Other cert","domain_names":["other.example"],"expires_on":"2027-01-01T00:00:00Z"},{"id":4,"provider":"letsencrypt","nice_name":"Wildcard cert","domain_names":["z.example","a.example"],"expires_on":"2026-11-15T04:17:54.000Z","meta":{"certificate_key":"secret"}}]`
const accessListsFixture = `[{"id":10,"name":"Guests","satisfy_any":false,"pass_auth":true,"proxy_host_count":0},{"id":6,"name":"Staff","satisfy_any":true,"pass_auth":false,"proxy_host_count":3,"items":[{"username":"u","password":"secret"}]}]`
