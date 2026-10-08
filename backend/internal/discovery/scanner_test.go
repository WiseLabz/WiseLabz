package discovery

import (
	"context"
	"crypto/x509"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

func rangeOf(t *testing.T, cidr string) Range {
	t.Helper()
	return Range{prefix: netip.MustParsePrefix(cidr)}
}

func portOf(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func hint(typ string, scheme string, port int, path string, match func(connector.DiscoveryResponse) bool) connector.TypeDiscovery {
	return connector.TypeDiscovery{
		Type: typ,
		Name: "Fake " + typ,
		DiscoveryHint: connector.DiscoveryHint{
			Probes:      []connector.DiscoveryProbe{{Port: port, Scheme: scheme, Path: path, Match: match}},
			URLTemplate: "{scheme}://{host}:{port}/api",
		},
	}
}

func bodyIs(want string) func(connector.DiscoveryResponse) bool {
	return func(r connector.DiscoveryResponse) bool { return r.Status == 200 && string(r.Body) == want }
}

type collector struct {
	mu         sync.Mutex
	candidates []Candidate
	progress   []Progress
}

func (c *collector) callbacks() Callbacks {
	return Callbacks{
		OnCandidate: func(cand Candidate) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.candidates = append(c.candidates, cand)
		},
		OnProgress: func(p Progress) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.progress = append(c.progress, p)
		},
	}
}

func TestScannerConfirmsProduct(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	var gotAuth, gotCookie atomic.Value
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		gotAuth.Store(r.Header.Get("Authorization"))
		gotCookie.Store(r.Header.Get("Cookie"))
		if r.URL.Path != "/id" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("fake-product"))
	}))
	defer srv.Close()
	port := portOf(t, srv.URL)

	var c collector
	s := NewScanner([]connector.TypeDiscovery{hint("fake_a", "http", port, "/id", bodyIs("fake-product"))})
	p := s.Run(context.Background(), rangeOf(t, "127.0.0.1/32"), c.callbacks())

	if len(c.candidates) != 1 {
		t.Fatalf("candidates = %+v, want 1", c.candidates)
	}
	got := c.candidates[0]
	want := Candidate{Type: "fake_a", Name: "Fake fake_a", Address: "127.0.0.1", Port: port, URL: "http://127.0.0.1:" + strconv.Itoa(port) + "/api", URLField: "url"}
	if got != want {
		t.Errorf("candidate = %+v, want %+v", got, want)
	}
	if p != (Progress{Done: 1, Total: 1, Answered: 1}) {
		t.Errorf("progress = %+v", p)
	}
	if hits.Load() != 1 {
		t.Errorf("requests = %d, want exactly 1 per type per port", hits.Load())
	}
	if gotAuth.Load() != "" || gotCookie.Load() != "" {
		t.Errorf("probe sent credentials: auth=%q cookie=%q", gotAuth.Load(), gotCookie.Load())
	}
	if len(c.progress) != 1 || c.progress[0] != p {
		t.Errorf("progress callbacks = %+v", c.progress)
	}
}

func TestScannerTLSWithSelfSignedCertificate(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("fake-tls"))
	}))
	defer srv.Close()
	port := portOf(t, srv.URL)

	var leaf atomic.Pointer[x509.Certificate]
	match := func(r connector.DiscoveryResponse) bool {
		leaf.Store(r.TLSLeaf)
		return string(r.Body) == "fake-tls"
	}
	var c collector
	NewScanner([]connector.TypeDiscovery{hint("fake_tls", "https", port, "/", match)}).
		Run(context.Background(), rangeOf(t, "127.0.0.1/32"), c.callbacks())
	if len(c.candidates) != 1 {
		t.Fatalf("candidates = %+v, want the self-signed server confirmed", c.candidates)
	}
	if !strings.HasPrefix(c.candidates[0].URL, "https://127.0.0.1:") {
		t.Errorf("URL = %q", c.candidates[0].URL)
	}
	if leaf.Load() == nil {
		t.Error("matcher got no TLS leaf certificate")
	}
}

func TestScannerOpenPortOfAnotherProductIsAnsweredNotReported(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("something else"))
	}))
	defer srv.Close()

	var c collector
	p := NewScanner([]connector.TypeDiscovery{hint("fake_a", "http", portOf(t, srv.URL), "/id", bodyIs("fake-product"))}).
		Run(context.Background(), rangeOf(t, "127.0.0.1/32"), c.callbacks())
	if len(c.candidates) != 0 {
		t.Errorf("candidates = %+v, want none", c.candidates)
	}
	if p.Answered != 1 {
		t.Errorf("answered = %d, want 1", p.Answered)
	}
}

func TestScannerPortOutsideTheListIsNeverDialled(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	var unlisted atomic.Int32
	other, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = other.Close() })
	go func() {
		for {
			c, err := other.Accept()
			if err != nil {
				return
			}
			unlisted.Add(1)
			_ = c.Close()
		}
	}()

	// The listed port is closed: a listener that was opened and closed again.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	var c collector
	p := NewScanner([]connector.TypeDiscovery{hint("fake_a", "http", closedPort, "/", bodyIs("x"))}).
		Run(context.Background(), rangeOf(t, "127.0.0.1/32"), c.callbacks())
	time.Sleep(50 * time.Millisecond)
	if unlisted.Load() != 0 {
		t.Errorf("unlisted port received %d connections", unlisted.Load())
	}
	if p.Answered != 0 || len(c.candidates) != 0 {
		t.Errorf("progress = %+v candidates = %+v, want nothing answered", p, c.candidates)
	}
	if p.Done != 1 || p.Total != 1 {
		t.Errorf("progress = %+v, want 1 of 1 done", p)
	}
}

func TestScannerTwoProductsOnOneHost(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	mk := func(body string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
	}
	a, b := mk("product-a"), mk("product-b")
	defer a.Close()
	defer b.Close()

	var c collector
	p := NewScanner([]connector.TypeDiscovery{
		hint("fake_a", "http", portOf(t, a.URL), "/", bodyIs("product-a")),
		hint("fake_b", "http", portOf(t, b.URL), "/", bodyIs("product-b")),
	}).Run(context.Background(), rangeOf(t, "127.0.0.1/32"), c.callbacks())

	var types []string
	for _, cand := range c.candidates {
		types = append(types, cand.Type)
		if cand.Address != "127.0.0.1" {
			t.Errorf("address = %s", cand.Address)
		}
	}
	sort.Strings(types)
	if strings.Join(types, ",") != "fake_a,fake_b" {
		t.Errorf("candidate types = %v, want both products", types)
	}
	if p.Answered != 1 {
		t.Errorf("answered = %d, want the one host counted once", p.Answered)
	}
}

func TestScannerFirstConfirmingProbeWinsPerType(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	var second atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("yes")) }))
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		second.Add(1)
		_, _ = w.Write([]byte("yes"))
	}))
	defer first.Close()
	defer other.Close()

	h := connector.TypeDiscovery{Type: "fake_two", Name: "Fake two", DiscoveryHint: connector.DiscoveryHint{
		Probes: []connector.DiscoveryProbe{
			{Port: portOf(t, first.URL), Scheme: "http", Path: "/", Match: bodyIs("yes")},
			{Port: portOf(t, other.URL), Scheme: "http", Path: "/", Match: bodyIs("yes")},
		},
		URLTemplate: "{scheme}://{host}:{port}",
	}}
	var c collector
	NewScanner([]connector.TypeDiscovery{h}).Run(context.Background(), rangeOf(t, "127.0.0.1/32"), c.callbacks())
	if len(c.candidates) != 1 || c.candidates[0].Port != portOf(t, first.URL) {
		t.Errorf("candidates = %+v, want only the first confirming probe", c.candidates)
	}
	if second.Load() != 0 {
		t.Errorf("second probe sent %d requests after the type was confirmed", second.Load())
	}
}

func TestScannerDoesNotFollowRedirects(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	// The redirect target listens on 127.0.0.2, outside the scanned 127.0.0.1/32.
	var targetHits atomic.Int32
	targetLn, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("cannot bind 127.0.0.2: %v", err)
	}
	target := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHits.Add(1)
		_, _ = w.Write([]byte("target"))
	}))
	target.Listener = targetLn
	target.Start()
	defer target.Close()

	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/elsewhere", http.StatusFound)
	}))
	defer src.Close()

	var seen atomic.Value
	match := func(r connector.DiscoveryResponse) bool {
		seen.Store(r)
		return r.Status == http.StatusFound && strings.HasPrefix(r.Header.Get("Location"), target.URL)
	}
	var c collector
	NewScanner([]connector.TypeDiscovery{hint("fake_redirect", "http", portOf(t, src.URL), "/", match)}).
		Run(context.Background(), rangeOf(t, "127.0.0.1/32"), c.callbacks())

	if targetHits.Load() != 0 {
		t.Errorf("redirect target received %d requests, want 0", targetHits.Load())
	}
	if len(c.candidates) != 1 {
		t.Errorf("candidates = %+v, want the redirect response itself judged", c.candidates)
	}
	if r, _ := seen.Load().(connector.DiscoveryResponse); r.Status != http.StatusFound {
		t.Errorf("matcher saw status %d, want the 302", r.Status)
	}
}

func TestScannerCapsBodyAndTimesOutSlowServers(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", maxBody*2)))
	}))
	defer big.Close()
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { <-release }))
	defer slow.Close()
	defer close(release)

	var bodyLen atomic.Int64
	var c collector
	s := NewScanner([]connector.TypeDiscovery{
		hint("fake_big", "http", portOf(t, big.URL), "/", func(r connector.DiscoveryResponse) bool {
			bodyLen.Store(int64(len(r.Body)))
			return true
		}),
		hint("fake_slow", "http", portOf(t, slow.URL), "/", func(connector.DiscoveryResponse) bool { return true }),
	})
	s.requestTimeout = 150 * time.Millisecond
	start := time.Now()
	p := s.Run(context.Background(), rangeOf(t, "127.0.0.1/32"), c.callbacks())

	if bodyLen.Load() != maxBody {
		t.Errorf("matcher saw %d body bytes, want %d", bodyLen.Load(), maxBody)
	}
	if len(c.candidates) != 1 || c.candidates[0].Type != "fake_big" {
		t.Errorf("candidates = %+v, want only the fast server", c.candidates)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("slow server held the scan for %v", time.Since(start))
	}
	if p.Answered != 1 {
		t.Errorf("answered = %d, want 1", p.Answered)
	}
}

func TestScannerDialsOnlyInsideTheRange(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	var mu sync.Mutex
	var dialled []string
	s := NewScanner([]connector.TypeDiscovery{hint("fake_a", "http", port, "/", bodyIs("x"))})
	s.newDialer = func(timeout time.Duration, allowed *net.IPNet) *net.Dialer {
		d := connector.GuardedDialerForRange(timeout, allowed)
		inner := d.Control
		d.Control = func(network, address string, c syscall.RawConn) error {
			mu.Lock()
			dialled = append(dialled, address)
			mu.Unlock()
			return inner(network, address, c)
		}
		return d
	}
	r := rangeOf(t, "127.0.0.0/30")
	p := s.Run(context.Background(), r, Callbacks{})
	if p.Total != 2 || p.Done != 2 {
		t.Fatalf("progress = %+v, want 2 hosts (network and broadcast skipped)", p)
	}
	if len(dialled) != 2 {
		t.Fatalf("dialled = %v, want one connect per host", dialled)
	}
	for _, a := range dialled {
		host, _, _ := net.SplitHostPort(a)
		if !r.IPNet().Contains(net.ParseIP(host)) || host == "127.0.0.0" || host == "127.0.0.3" {
			t.Errorf("dialled %s outside the probe set", a)
		}
	}
}

func TestScannerCancellationStopsFurtherDials(t *testing.T) {
	connector.AllowLoopbackForTest(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	var dials atomic.Int32
	s := NewScanner([]connector.TypeDiscovery{hint("fake_a", "http", port, "/", bodyIs("x"))})
	s.maxConns = 1
	s.newDialer = func(timeout time.Duration, allowed *net.IPNet) *net.Dialer {
		d := connector.GuardedDialerForRange(timeout, allowed)
		inner := d.Control
		d.Control = func(network, address string, c syscall.RawConn) error {
			dials.Add(1)
			return inner(network, address, c)
		}
		return d
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var once sync.Once
	p := s.Run(ctx, rangeOf(t, "127.0.0.0/24"), Callbacks{OnProgress: func(Progress) { once.Do(cancel) }})

	atReturn := dials.Load()
	time.Sleep(100 * time.Millisecond)
	if dials.Load() != atReturn {
		t.Errorf("dials grew from %d to %d after Run returned", atReturn, dials.Load())
	}
	if atReturn >= 100 {
		t.Errorf("%d dials for a scan cancelled after the first host, want far fewer than 254", atReturn)
	}
	if p.Done >= p.Total {
		t.Errorf("progress = %+v, want the scan cut short", p)
	}
}

func TestScannerNoHintsOrNoHosts(t *testing.T) {
	if p := NewScanner(nil).Run(context.Background(), rangeOf(t, "127.0.0.1/32"), Callbacks{}); p.Done != 0 {
		t.Errorf("no hints: progress = %+v", p)
	}
	h := hint("fake_a", "http", 1, "/", bodyIs("x"))
	if p := NewScanner([]connector.TypeDiscovery{h}).Run(context.Background(), Range{}, Callbacks{}); p != (Progress{}) {
		t.Errorf("zero range: progress = %+v", p)
	}
}
