package discovery

import (
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/httpx"
)

const (
	// connectTimeout bounds one TCP connect to a listed port.
	connectTimeout = 500 * time.Millisecond
	// requestTimeout bounds one probe request, connect included.
	requestTimeout = 2 * time.Second
	// maxBody caps the response body handed to a matcher, and the response
	// header block the transport buffers (the default is 10 MiB per response,
	// which hostile hosts times maxConns probes could turn into gigabytes).
	maxBody = 64 << 10
	// maxConns bounds concurrent connections, TCP connects and probe requests
	// together.
	maxConns = 128

	userAgent = "WiseLabz-network-discovery"
)

// Candidate is a confirmed product on the network.
type Candidate struct {
	// Type is the connector type; Name is its display name.
	Type    string `json:"type"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	// URL is the connector URL to prefill, in the config field URLField
	// ("url" unless the type keeps its endpoint elsewhere).
	URL      string `json:"url"`
	URLField string `json:"urlField"`
	// ConnectorID is set when an existing connector of the same type already
	// points at this address and port.
	ConnectorID string `json:"connectorId,omitempty"`
}

// Scanner probes the hosts of a Range for the products its hints describe.
// The zero value is not usable; build one with NewScanner.
type Scanner struct {
	hints []connector.TypeDiscovery
	ports []int

	connectTimeout time.Duration
	requestTimeout time.Duration
	maxConns       int
	// newDialer builds the range-scoped dialer for a scan; tests wrap it to
	// count dials.
	newDialer func(timeout time.Duration, allowed *net.IPNet) *net.Dialer
}

// NewScanner returns a Scanner for the given hints. The ports it connects to
// are exactly the ports the hints probe, so adding a connector type's hint is
// the only change needed to discover it.
func NewScanner(hints []connector.TypeDiscovery) *Scanner {
	seen := map[int]bool{}
	var ports []int
	for _, h := range hints {
		for _, p := range h.Probes {
			if !seen[p.Port] {
				seen[p.Port] = true
				ports = append(ports, p.Port)
			}
		}
	}
	sort.Ints(ports)
	return &Scanner{
		hints:          hints,
		ports:          ports,
		connectTimeout: connectTimeout,
		requestTimeout: requestTimeout,
		maxConns:       maxConns,
		newDialer:      connector.GuardedDialerForRange,
	}
}

// Progress reports scan progress: hosts finished out of total, and how many of
// those had a listed port accept a connection.
type Progress struct {
	Done     int
	Total    int
	Answered int
}

// Callbacks receive scan events. Both are optional and are called from
// scanner goroutines, possibly concurrently, so they must be safe for that.
type Callbacks struct {
	// OnProgress is called after each host finishes.
	OnProgress func(Progress)
	// OnCandidate is called once for each confirmed product, as it is found.
	OnCandidate func(Candidate)
}

// Run scans every host of r and returns the final progress. It returns early
// when ctx ends: hosts not yet started are skipped and no further connection
// is opened. Every connection is made through a dialer scoped to r.
func (s *Scanner) Run(ctx context.Context, r Range, cb Callbacks) Progress {
	hosts := r.Hosts()
	total := len(hosts)
	if total == 0 || len(s.ports) == 0 {
		return Progress{Total: total}
	}

	dialer := s.newDialer(s.connectTimeout, r.IPNet())
	client := s.httpClient(dialer)
	sem := make(chan struct{}, s.maxConns)

	var done, answered atomic.Int64
	var warned sync.Map // matcher panics already logged this scan
	var wg sync.WaitGroup
	for _, host := range hosts {
		wg.Add(1)
		go func(host netip.Addr) {
			defer wg.Done()
			if ctx.Err() != nil {
				return
			}
			open := s.openPorts(ctx, dialer, sem, host)
			if len(open) > 0 {
				answered.Add(1)
				s.probeHost(ctx, client, sem, &warned, host, open, cb.OnCandidate)
			}
			// A host abandoned mid-way by cancellation is not counted as done.
			if ctx.Err() != nil {
				return
			}
			d := done.Add(1)
			if cb.OnProgress != nil {
				cb.OnProgress(Progress{Done: int(d), Total: total, Answered: int(answered.Load())})
			}
		}(host)
	}
	wg.Wait()
	return Progress{Done: int(done.Load()), Total: total, Answered: int(answered.Load())}
}

// httpClient builds the probe client. It dials only through the scoped dialer
// (so only IP literals inside the range), never follows redirects and never
// uses a proxy.
func (s *Scanner) httpClient(dialer *net.Dialer) *http.Client {
	return &http.Client{
		CheckRedirect: httpx.NoRedirect,
		Transport: &http.Transport{
			Proxy:       nil,
			DialContext: dialer.DialContext,
			// Home-lab products ship self-signed certificates, so a probe cannot
			// require a trusted one. This is safe here because the scan sends no
			// credentials and reads only public response fields (status, headers,
			// the first bytes of the body, the certificate itself) to recognise a
			// product; nothing read is trusted beyond that. This is the only
			// transport in the app that skips verification unconditionally.
			TLSClientConfig:        &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // see above
			DisableKeepAlives:      true,
			TLSHandshakeTimeout:    s.requestTimeout,
			ResponseHeaderTimeout:  s.requestTimeout,
			MaxResponseHeaderBytes: maxBody,
		},
	}
}

func acquire(ctx context.Context, sem chan struct{}) bool {
	select {
	case sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

// openPorts returns the listed ports on host that accept a TCP connection.
func (s *Scanner) openPorts(ctx context.Context, dialer *net.Dialer, sem chan struct{}, host netip.Addr) map[int]bool {
	var mu sync.Mutex
	open := map[int]bool{}
	var wg sync.WaitGroup
	for _, port := range s.ports {
		wg.Add(1)
		go func(port int) {
			defer wg.Done()
			if !acquire(ctx, sem) {
				return
			}
			defer func() { <-sem }()
			conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host.String(), strconv.Itoa(port)))
			if err != nil {
				return
			}
			_ = conn.Close()
			mu.Lock()
			open[port] = true
			mu.Unlock()
		}(port)
	}
	wg.Wait()
	return open
}

// probeHost sends each type's probes for the open ports, in the order the
// type declares them, and stops at the first that confirms the type, so a
// type is reported at most once per host.
func (s *Scanner) probeHost(ctx context.Context, client *http.Client, sem chan struct{}, warned *sync.Map, host netip.Addr, open map[int]bool, onCandidate func(Candidate)) {
	var wg sync.WaitGroup
	for _, hint := range s.hints {
		wg.Add(1)
		go func(hint connector.TypeDiscovery) {
			defer wg.Done()
			for _, p := range hint.Probes {
				if !open[p.Port] {
					continue
				}
				if !acquire(ctx, sem) {
					return
				}
				resp, ok := s.probe(ctx, client, host, p)
				<-sem
				if !ok || !safeMatch(warned, hint.Type, p, resp) {
					continue
				}
				if onCandidate != nil && ctx.Err() == nil {
					field := hint.URLField
					if field == "" {
						field = "url"
					}
					onCandidate(Candidate{
						Type:     hint.Type,
						Name:     hint.Name,
						Address:  host.String(),
						Port:     p.Port,
						URL:      hint.URL(p.Scheme, host.String(), p.Port),
						URLField: field,
					})
				}
				return
			}
		}(hint)
	}
	wg.Wait()
}

// safeMatch runs a probe's matcher on bytes any host on the scanned network
// controls. A matcher that panics, or is missing, counts as no match: the
// panic would otherwise end the process, since this runs outside the scan
// goroutine's recover. It logs the first panic per connector type and port in
// a scan, never the host.
func safeMatch(warned *sync.Map, typ string, p connector.DiscoveryProbe, resp connector.DiscoveryResponse) (matched bool) {
	if p.Match == nil {
		return false
	}
	defer func() {
		if recover() != nil {
			matched = false
			if _, dup := warned.LoadOrStore(typ+":"+strconv.Itoa(p.Port), true); !dup {
				slog.Warn("network discovery matcher panicked", "type", typ, "port", p.Port)
			}
		}
	}()
	return p.Match(resp)
}

// probe sends one unauthenticated GET and captures the response for a matcher.
func (s *Scanner) probe(ctx context.Context, client *http.Client, host netip.Addr, p connector.DiscoveryProbe) (connector.DiscoveryResponse, bool) {
	ctx, cancel := context.WithTimeout(ctx, s.requestTimeout)
	defer cancel()
	target := p.Scheme + "://" + net.JoinHostPort(host.String(), strconv.Itoa(p.Port)) + p.Path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return connector.DiscoveryResponse{}, false
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return connector.DiscoveryResponse{}, false
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	out := connector.DiscoveryResponse{Status: resp.StatusCode, Header: resp.Header, Body: body}
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		out.TLSLeaf = resp.TLS.PeerCertificates[0]
	}
	return out, true
}
