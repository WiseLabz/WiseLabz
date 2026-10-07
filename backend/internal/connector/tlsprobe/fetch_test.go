package tlsprobe

import (
	"context"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func fetch(t *testing.T, config map[string]any) *connector.ServiceSnapshot {
	t.Helper()
	snap, err := (&Connector{}).Fetch(context.Background(), config)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	return snap
}

// roundTrip stores a snapshot the way the sync layer does: as JSON.
func roundTrip(t *testing.T, snap *connector.ServiceSnapshot) *connector.ServiceSnapshot {
	t.Helper()
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var out connector.ServiceSnapshot
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return &out
}

func now() time.Time { return time.Now().UTC().Truncate(time.Second) }

func TestFetchReadsValidCertificate(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	notBefore, notAfter := now().Add(-time.Hour), now().Add(90*24*time.Hour)
	ca, caKey, _ := issue(t, certSpec{subject: pkix.Name{CommonName: "Lab Root CA"}, notBefore: notBefore, notAfter: notAfter.Add(time.Hour), isCA: true}, nil, nil)
	_, key, der := issue(t, certSpec{
		subject: pkix.Name{CommonName: "nas.lab"}, dnsNames: []string{"nas.lab", "alt.lab", "ALT.lab"},
		ips: []net.IP{net.ParseIP("127.0.0.1")}, notBefore: notBefore, notAfter: notAfter,
	}, ca, caKey)
	srv := startTLSServer(t, tlsCertificate(der, key))

	snap := fetch(t, targetsConfig(srv.addr))
	if len(snap.Entities) != 1 {
		t.Fatalf("entities = %d", len(snap.Entities))
	}
	e := snap.Entities[0]
	if e.Kind != "certificate" || e.Name != srv.addr || e.ExternalID != srv.addr {
		t.Errorf("entity %+v", e)
	}
	want := map[string]any{
		"host": "127.0.0.1", "port": srv.port(), "not_after": notAfter.Format(time.RFC3339), "not_before": notBefore.Format(time.RFC3339),
		"issuer": "CN=Lab Root CA", "subject": "CN=nas.lab", "dns_names": []string{"ALT.lab", "alt.lab", "nas.lab"},
		"self_signed": false, "reachable": true, "source": "manual",
	}
	if !reflect.DeepEqual(e.Attributes, want) {
		t.Errorf("attributes = %#v\nwant %#v", e.Attributes, want)
	}
	if _, has := e.Attributes["error"]; has {
		t.Error("error attribute set on a reachable target")
	}
	if snap.Metadata[connector.MetadataHealthStatus] != "" {
		t.Error("reachable connector reported offline")
	}
}

func TestFetchSelfSignedCertificate(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	_, key, der := issue(t, certSpec{
		subject: pkix.Name{CommonName: "self.lab"}, dnsNames: []string{"self.lab"},
		notBefore: now().Add(-time.Hour), notAfter: now().Add(24 * time.Hour),
	}, nil, nil)
	srv := startTLSServer(t, tlsCertificate(der, key))
	snap := fetch(t, targetsConfig(srv.addr))
	e := snap.Entities[0]
	if e.Attributes["self_signed"] != true || e.Attributes["reachable"] != true || e.Attributes["not_after"] == nil {
		t.Errorf("attributes = %#v", e.Attributes)
	}
}

func TestFetchPrivateCAChain(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	root, rootKey, _ := issue(t, certSpec{subject: pkix.Name{CommonName: "Private Root"}, notBefore: now().Add(-time.Hour), notAfter: now().Add(1000 * time.Hour), isCA: true}, nil, nil)
	inter, interKey, interDER := issue(t, certSpec{subject: pkix.Name{CommonName: "Private Issuing CA"}, notBefore: now().Add(-time.Hour), notAfter: now().Add(900 * time.Hour), isCA: true}, root, rootKey)
	_, key, der := issue(t, certSpec{subject: pkix.Name{CommonName: "app.lab"}, dnsNames: []string{"app.lab"}, notBefore: now().Add(-time.Hour), notAfter: now().Add(72 * time.Hour)}, inter, interKey)
	srv := startTLSServer(t, tlsCertificate(der, key, interDER))
	e := fetch(t, targetsConfig(srv.addr)).Entities[0]
	if e.Attributes["issuer"] != "CN=Private Issuing CA" || e.Attributes["self_signed"] != false || e.Attributes["reachable"] != true {
		t.Errorf("attributes = %#v", e.Attributes)
	}
}

func selfSignedServer(t *testing.T) *tlsServer {
	t.Helper()
	_, key, der := issue(t, certSpec{subject: pkix.Name{CommonName: "x.lab"}, notBefore: now().Add(-time.Hour), notAfter: now().Add(48 * time.Hour)}, nil, nil)
	return startTLSServer(t, tlsCertificate(der, key))
}

func TestFetchSendsServerNameForHostsOnly(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	srv := selfSignedServer(t)
	fetch(t, targetsConfig(srv.addr))
	if name := <-srv.hello; name != "" {
		t.Errorf("SNI for an IP literal = %q", name)
	}
	fetch(t, targetsConfig("localhost:"+strconv.Itoa(srv.port())))
	if name := <-srv.hello; name != "localhost" {
		t.Errorf("SNI = %q, want localhost", name)
	}
}

func TestFetchWritesNothingAfterHandshake(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	srv := selfSignedServer(t)
	fetch(t, targetsConfig(srv.addr))
	select {
	case got := <-srv.postHandshakeRead:
		if got.n != 0 || got.err != io.EOF {
			t.Errorf("server received %d bytes, err %v after the handshake; want a bare close", got.n, got.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server never saw the connection close")
	}
}

func TestFetchBlocksLoopbackWithoutTestSeam(t *testing.T) {
	srv := selfSignedServer(t)
	for _, target := range []string{srv.addr, "localhost:" + strconv.Itoa(srv.port())} {
		snap := fetch(t, targetsConfig(target))
		e := snap.Entities[0]
		if e.Attributes["reachable"] != false || !strings.HasPrefix(e.Attributes["error"].(string), "blocked: ") {
			t.Errorf("%s: attributes = %#v", target, e.Attributes)
		}
	}
	if n := srv.accepted.Load(); n != 0 {
		t.Errorf("blocked loopback target was connected to %d times", n)
	}
}

func TestFetchUnreachableClasses(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	old := probeTimeout
	probeTimeout = 300 * time.Millisecond
	t.Cleanup(func() { probeTimeout = old })

	silent := startRawServer(t, func(_ net.Conn) { time.Sleep(time.Second) })
	plain := startRawServer(t, func(conn net.Conn) {
		_, _ = conn.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
	})
	cases := map[string]struct{ target, prefix string }{
		"refused":   {closedPort(t), "refused: connection refused"},
		"dns":       {"does-not-exist.invalid:443", "dns: "},
		"timeout":   {silent, "timeout: no response within 300ms"},
		"handshake": {plain, "handshake: TLS handshake failed"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			snap := fetch(t, targetsConfig(tc.target))
			e := snap.Entities[0]
			msg, _ := e.Attributes["error"].(string)
			if e.Attributes["reachable"] != false || !strings.HasPrefix(msg, tc.prefix) {
				t.Errorf("attributes = %#v", e.Attributes)
			}
			if _, has := e.Attributes["not_after"]; has {
				t.Error("never-reached target has not_after")
			}
			if e.Name != e.ExternalID || e.Kind != "certificate" {
				t.Errorf("entity %+v", e)
			}
		})
	}
}

func TestFetchOneTargetDownKeepsPreviousCertificate(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	up := selfSignedServer(t)
	flaky := selfSignedServer(t)
	config := targetsConfig(up.addr, flaky.addr)
	first := fetch(t, config)
	flakyAddr := flaky.addr
	flaky.close()

	config["_previous_snapshot"] = roundTrip(t, first)
	second := fetch(t, config)
	got := entityByID(t, second.Entities, flakyAddr)
	prev := entityByID(t, first.Entities, flakyAddr)
	if got.Attributes["reachable"] != false || got.Attributes["error"] == nil {
		t.Fatalf("attributes = %#v", got.Attributes)
	}
	for _, key := range carriedAttributes {
		if !reflect.DeepEqual(got.Attributes[key], prev.Attributes[key]) {
			t.Errorf("%s = %#v, want %#v", key, got.Attributes[key], prev.Attributes[key])
		}
	}
	if entityByID(t, second.Entities, up.addr).Attributes["reachable"] != true {
		t.Error("healthy target not reachable")
	}
	if _, offline := connector.SnapshotOfflineMessage(second); offline {
		t.Error("connector with one target up reported offline")
	}
}

func TestHealthRule(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	down1, down2 := closedPort(t), closedPort(t)
	up := selfSignedServer(t)

	snap := fetch(t, targetsConfig(down1, down2))
	msg, offline := connector.SnapshotOfflineMessage(snap)
	if !offline || msg != "All 2 targets unreachable" || !connector.IsAllTargetsUnreachable(msg) {
		t.Errorf("all down: offline=%v message=%q", offline, msg)
	}
	if len(snap.Entities) != 2 {
		t.Errorf("entities kept = %d", len(snap.Entities))
	}
	if _, offline := connector.SnapshotOfflineMessage(fetch(t, targetsConfig(down1, up.addr))); offline {
		t.Error("one target up reported offline")
	}
	empty := fetch(t, map[string]any{})
	if _, offline := connector.SnapshotOfflineMessage(empty); offline || len(empty.Entities) != 0 {
		t.Errorf("zero targets: offline=%v entities=%d", offline, len(empty.Entities))
	}
}

func TestFetchSnapshotsAreStable(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	up := selfSignedServer(t)
	down := closedPort(t)
	config := targetsConfig(up.addr, down, "does-not-exist.invalid:443")

	first := fetch(t, config)
	config["_previous_snapshot"] = roundTrip(t, first)
	second := fetch(t, config)
	config["_previous_snapshot"] = roundTrip(t, second)
	third := fetch(t, config)

	for name, snap := range map[string]*connector.ServiceSnapshot{"second": second, "third": third} {
		a, b := *first, *snap
		a.FetchedAt, b.FetchedAt = time.Time{}, time.Time{}
		aj, _ := json.Marshal(a)
		bj, _ := json.Marshal(b)
		if string(aj) != string(bj) {
			t.Errorf("%s fetch differs from the first:\n%s\n%s", name, aj, bj)
		}
	}
	for _, e := range first.Entities {
		for _, banned := range []string{"checked_at", "days_left", "last_checked"} {
			if _, has := e.Attributes[banned]; has {
				t.Errorf("churning attribute %q present", banned)
			}
		}
	}
}

func TestFetchCapsHostileCertificateFields(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	names := make([]string, 0, 200)
	for i := 0; i < 200; i++ {
		names = append(names, fmt.Sprintf("n%03d.%s.lab", i, strings.Repeat("x", 30)))
	}
	names = append(names, "evil\x00\x07name.lab")
	long := strings.Repeat("A", 5000)
	_, key, der := issue(t, certSpec{
		subject:  pkix.Name{CommonName: long, Organization: []string{"line1\nline2\x1b[31m"}},
		dnsNames: names, notBefore: now().Add(-time.Hour), notAfter: now().Add(time.Hour),
	}, nil, nil)
	srv := startTLSServer(t, tlsCertificate(der, key))
	e := fetch(t, targetsConfig(srv.addr)).Entities[0]
	subject := e.Attributes["subject"].(string)
	if len(subject) > maxFieldLen || strings.ContainsAny(subject, "\n\x1b") {
		t.Errorf("subject len %d: %q", len(subject), subject)
	}
	dns := e.Attributes["dns_names"].([]string)
	if len(dns) != maxDNSNames {
		t.Errorf("dns_names = %d", len(dns))
	}
	for i, n := range dns {
		if len(n) > maxFieldLen || strings.ContainsAny(n, "\x00\x07") || (i > 0 && dns[i-1] > n) {
			t.Errorf("dns name %d invalid: %q", i, n)
		}
	}
}

func TestFetchBoundsConcurrency(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	var inFlight, peak atomic.Int32
	addr := startRawServerOn(t, "0.0.0.0:0", func(_ net.Conn) {
		n := inFlight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
		inFlight.Add(-1)
	})
	_, port, _ := net.SplitHostPort(addr)
	lines := make([]string, 0, 24)
	for i := 0; i < 24; i++ {
		// One listener, many distinct targets: every 127.0.0.x reaches it.
		lines = append(lines, fmt.Sprintf("127.0.0.%d:%s", i+1, port))
	}
	snap := fetch(t, targetsConfig(lines...))
	if len(snap.Entities) != 24 {
		t.Fatalf("entities = %d", len(snap.Entities))
	}
	if p := peak.Load(); p > maxConcurrency || p < 2 {
		t.Errorf("peak concurrent connections = %d, want 2..%d", p, maxConcurrency)
	}
}

func TestFetchHonoursCancellation(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	addr := startRawServer(t, func(_ net.Conn) { time.Sleep(2 * time.Second) })
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	start := time.Now()
	_, err := (&Connector{}).Fetch(ctx, targetsConfig(addr))
	if err == nil {
		t.Fatal("cancelled fetch succeeded")
	}
	if time.Since(start) > time.Second {
		t.Errorf("cancellation took %s", time.Since(start))
	}
}
