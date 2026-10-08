// Package discovery provides the network discovery API: range suggestions and
// starting, reading and cancelling the instance's single network scan.
package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	disc "github.com/WiseLabz/wiselabz/internal/discovery"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// Suggestion sources.
const (
	SourceClient = "client"
	SourceServer = "server"
)

// maxAuditRange caps the submitted range echoed into a reject audit entry, so
// a long junk value cannot bloat the audit log.
const maxAuditRange = 64

// Options override the production wiring; the zero value is production.
type Options struct {
	// Runner replaces the network scanner (tests).
	Runner disc.Runner
	// InterfaceAddrs replaces net.InterfaceAddrs for range suggestions (tests).
	InterfaceAddrs func() ([]net.Addr, error)
}

// Handler holds dependencies for the network discovery endpoints.
type Handler struct {
	Store          *store.Store
	Manager        *disc.Manager
	TrustedProxies string
	interfaceAddrs func() ([]net.Addr, error)
}

// NewHandler builds the handler and the instance's scan manager. hub may be
// nil, in which case no live events are sent.
func NewHandler(s *store.Store, hub *ws.Hub, trustedProxies string, opts Options) *Handler {
	runner := opts.Runner
	if runner == nil {
		runner = disc.NewScanner(connector.DiscoveryHints())
	}
	cfg := disc.Config{
		Runner:    runner,
		Audit:     s,
		Endpoints: connectorEndpoints(s),
	}
	if hub != nil {
		cfg.Events = hub
	}
	addrs := opts.InterfaceAddrs
	if addrs == nil {
		addrs = net.InterfaceAddrs
	}
	return &Handler{Store: s, Manager: disc.NewManager(cfg), TrustedProxies: trustedProxies, interfaceAddrs: addrs}
}

// connectorEndpoints lists where the existing connectors of discoverable types
// point, for the already-connected marker. A type that keeps its endpoint in a
// config field other than the top-level url (Docker's host) is read from there.
func connectorEndpoints(s *store.Store) func(ctx context.Context) ([]disc.Endpoint, error) {
	return func(ctx context.Context) ([]disc.Endpoint, error) {
		records, err := s.ListAllConnectors(ctx)
		if err != nil {
			return nil, err
		}
		urlFields := map[string]string{}
		for _, h := range connector.DiscoveryHints() {
			urlFields[h.Type] = h.URLField
		}
		var out []disc.Endpoint
		for _, rec := range records {
			field, discoverable := urlFields[rec.Type]
			if !discoverable {
				continue
			}
			value := rec.URL
			if field != "" {
				var cfg map[string]any
				if json.Unmarshal([]byte(rec.ConfigData), &cfg) != nil {
					continue
				}
				value, _ = cfg[field].(string)
			}
			out = append(out, disc.Endpoint{ID: rec.ID, Type: rec.Type, Value: value})
		}
		return out, nil
	}
}

type scanResponse struct {
	Scan disc.Scan `json:"scan"`
}

// Suggestion is a range the admin may scan.
type Suggestion struct {
	CIDR   string `json:"cidr"`
	Source string `json:"source"`
}

// Suggestions handles GET /api/discovery/suggestions: the /24 around the
// admin's own address, then around each of the server's private interface
// addresses, without duplicates.
func (h *Handler) Suggestions(w http.ResponseWriter, r *http.Request) {
	out := []Suggestion{}
	seen := map[netip.Prefix]bool{}
	add := func(addr netip.Addr, source string) {
		p, ok := privateSlash24(addr)
		if !ok || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, Suggestion{CIDR: p.String(), Source: source})
	}

	if ip, err := netip.ParseAddr(httputil.ClientIP(r, h.TrustedProxies)); err == nil {
		add(ip, SourceClient)
	}
	addrs, err := h.interfaceAddrs()
	if err != nil {
		slog.Warn("list interface addresses for network discovery", "error", err)
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			if ip, ok := netip.AddrFromSlice(ipnet.IP); ok {
				add(ip, SourceServer)
			}
		}
	}
	httputil.JSON(w, http.StatusOK, map[string]any{"suggestions": out})
}

// privateSlash24 returns the /24 containing an RFC 1918 IPv4 address.
func privateSlash24(addr netip.Addr) (netip.Prefix, bool) {
	addr = addr.Unmap()
	if !addr.Is4() || !addr.IsPrivate() {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(addr, 24).Masked(), true
}

type startRequest struct {
	CIDR string `json:"cidr"`
}

// Start handles POST /api/discovery/scan (behind elevation when step-up is on).
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	req, ok := httputil.DecodeJSON[startRequest](w, r)
	if !ok {
		return
	}

	rng, err := disc.ParseRange(req.CIDR)
	if err != nil {
		var re *disc.RangeError
		reason, msg := "invalid_cidr", "invalid range"
		if errors.As(err, &re) {
			reason, msg = re.Reason, re.Message
		}
		h.reject(r, req.CIDR, reason)
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", msg, []httputil.FieldError{{Field: "cidr", Msg: msg}})
		return
	}

	scan, err := h.Manager.Start(rng, disc.Actor{
		UserID:        auth.UserIDFromContext(r.Context()),
		InstanceAdmin: auth.InstanceAdminFromContext(r.Context()),
	})
	var running *disc.ScanRunningError
	var limited *disc.RateLimitError
	switch {
	case errors.As(err, &running):
		h.reject(r, rng.String(), "scan_in_progress")
		httputil.JSON(w, http.StatusConflict, httputil.ErrorResponse{
			Code:    "scan_in_progress",
			Message: "A network scan is already running",
			Details: map[string]any{"scan": running.Scan},
		})
		return
	case errors.As(err, &limited):
		h.reject(r, rng.String(), "rate_limited")
		seconds := int(math.Ceil(limited.RetryAfter.Seconds()))
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		httputil.JSON(w, http.StatusTooManyRequests, httputil.ErrorResponse{
			Code:    "rate_limited",
			Message: "Too many network scans, try again later",
			Details: map[string]any{"retryAfterSeconds": seconds},
		})
		return
	case err != nil:
		httputil.Errorf(w, err)
		return
	}

	if err := h.Store.RecordAuditFromContext(r.Context(), disc.AuditStart, disc.AuditTargetType, scan.ID, map[string]any{"range": scan.CIDR}); err != nil {
		slog.Error("record discovery scan start audit entry", "error", err)
	}
	httputil.JSON(w, http.StatusAccepted, scanResponse{Scan: scan})
}

// reject writes the discovery.scan.reject audit entry. The range is what the
// admin submitted, cut to a sane length; no scanned host is involved.
func (h *Handler) reject(r *http.Request, submitted, reason string) {
	if len(submitted) > maxAuditRange {
		submitted = submitted[:maxAuditRange]
	}
	detail := map[string]any{"range": submitted, "reason": reason}
	if err := h.Store.RecordAuditFromContext(r.Context(), disc.AuditReject, disc.AuditTargetType, "", detail); err != nil {
		slog.Error("record discovery scan reject audit entry", "error", err)
	}
}

// Get handles GET /api/discovery/scan: the current scan, or the most recent
// one until it expires.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	scan, ok := h.Manager.Current(r.Context())
	if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found", "No network scan")
		return
	}
	httputil.JSON(w, http.StatusOK, scanResponse{Scan: scan})
}

// Cancel handles DELETE /api/discovery/scan.
func (h *Handler) Cancel(w http.ResponseWriter, _ *http.Request) {
	scan, ok := h.Manager.Cancel()
	if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found", "No network scan is running")
		return
	}
	httputil.JSON(w, http.StatusOK, scanResponse{Scan: scan})
}
