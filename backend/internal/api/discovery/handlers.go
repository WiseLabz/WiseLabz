// Package discovery provides the network discovery API: range suggestions and
// starting, reading and cancelling the instance's single network scan.
package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/WiseLabz/wiselabz/internal/auth"
	"github.com/WiseLabz/wiselabz/internal/connector"
	"github.com/WiseLabz/wiselabz/internal/connector/tlsprobe"
	disc "github.com/WiseLabz/wiselabz/internal/discovery"
	"github.com/WiseLabz/wiselabz/internal/httputil"
	"github.com/WiseLabz/wiselabz/internal/store"
	"github.com/WiseLabz/wiselabz/internal/ws"
)

// Suggestion sources.
const (
	SourceClient    = "client"
	SourceServer    = "server"
	SourceInventory = "inventory"
)

// maxAuditRange caps the submitted range echoed into a reject audit entry, so
// a long junk value cannot bloat the audit log.
const maxAuditRange = 64

// Options override the production wiring; the zero value is production.
type Options struct {
	// Runner replaces the network scanner (tests).
	Runner disc.Runner
	// Manager replaces the scan manager NewHandler would build. The server
	// builds it with NewManager so it can stop a running scan at shutdown.
	Manager *disc.Manager
	// InterfaceAddrs replaces net.InterfaceAddrs for range suggestions (tests).
	InterfaceAddrs func() ([]net.Addr, error)
	// InventoryQuery replaces the unscoped indexed-address query (tests).
	InventoryQuery func(context.Context) ([]store.IndexedAddress, error)
	// Passive replaces the multicast source used by a newly built manager (tests).
	Passive disc.PassiveRunner
}

// Handler holds dependencies for the network discovery endpoints.
type Handler struct {
	Store          *store.Store
	Manager        *disc.Manager
	TrustedProxies string
	interfaceAddrs func() ([]net.Addr, error)
	inventoryQuery func(context.Context) ([]store.IndexedAddress, error)
}

// NewManager builds the instance's scan manager: the network scanner (or
// runner, when not nil), audit entries in s, live events through hub and the
// already-connected marker from s's connectors. hub may be nil, in which case
// no live events are sent.
func NewManager(s *store.Store, hub *ws.Hub, runner disc.Runner) *disc.Manager {
	return newManager(s, hub, runner, disc.NewPassiveSource())
}

func newManager(s *store.Store, hub *ws.Hub, runner disc.Runner, passive disc.PassiveRunner) *disc.Manager {
	if runner == nil {
		runner = disc.NewScanner(connector.DiscoveryHints())
	}
	cfg := disc.Config{
		Runner:    runner,
		Audit:     s,
		Endpoints: connectorEndpoints(s),
		Passive:   passive,
	}
	if hub != nil {
		cfg.Events = hub
	}
	return disc.NewManager(cfg)
}

// NewHandler builds the handler, using opts.Manager or else a new scan
// manager (see NewManager).
func NewHandler(s *store.Store, hub *ws.Hub, trustedProxies string, opts Options) *Handler {
	mgr := opts.Manager
	if mgr == nil {
		passive := opts.Passive
		if passive == nil {
			passive = disc.NewPassiveSource()
		}
		mgr = newManager(s, hub, opts.Runner, passive)
	}
	addrs := opts.InterfaceAddrs
	if addrs == nil {
		addrs = net.InterfaceAddrs
	}
	inventory := opts.InventoryQuery
	if inventory == nil {
		inventory = s.ListIndexedAddresses
	}
	return &Handler{Store: s, Manager: mgr, TrustedProxies: trustedProxies, interfaceAddrs: addrs, inventoryQuery: inventory}
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
		urlFields["dnsresolver"] = ""
		urlFields["tlsprobe"] = ""
		var out []disc.Endpoint
		for _, rec := range records {
			field, discoverable := urlFields[rec.Type]
			if !discoverable {
				continue
			}
			value := rec.URL
			if rec.Type == "tlsprobe" {
				var cfg map[string]any
				if json.Unmarshal([]byte(rec.ConfigData), &cfg) != nil {
					continue
				}
				if importID, _ := cfg["import_connector_id"].(string); strings.TrimSpace(importID) != "" {
					related, snapshotErr := s.GetLatestSnapshot(ctx, strings.TrimSpace(importID))
					if snapshotErr == nil {
						var snapshot connector.ServiceSnapshot
						if json.Unmarshal([]byte(related.Data), &snapshot) == nil {
							cfg["_related_snapshots"] = map[string]*connector.ServiceSnapshot{strings.TrimSpace(importID): &snapshot}
						}
					} else if !errors.Is(snapshotErr, store.ErrNotFound) {
						slog.Warn("load related snapshot for discovery marker", "errorType", fmt.Sprintf("%T", snapshotErr))
					}
				}
				targets, _ := tlsprobe.ConfiguredTargets(cfg)
				lines := make([]string, 0, len(targets))
				for _, target := range targets {
					lines = append(lines, net.JoinHostPort(target.Host, strconv.Itoa(target.Port)))
				}
				value = strings.Join(lines, "\n")
			} else if field != "" {
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
// admin's own address, then the server's, then up to three known-host networks.
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
		slog.Warn("list interface addresses for network discovery", "errorType", fmt.Sprintf("%T", err))
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			if ip, ok := netip.AddrFromSlice(ipnet.IP); ok {
				add(ip, SourceServer)
			}
		}
	}
	rows, err := h.inventoryQuery(r.Context())
	if err != nil {
		slog.Warn("list known hosts for network discovery suggestions", "errorType", fmt.Sprintf("%T", err))
		rows = nil
	}
	known := disc.InventorySource(rows, nil)
	out = append(out, inventorySuggestions(known, seen)...)
	httputil.JSON(w, http.StatusOK, map[string]any{"suggestions": out, "knownHosts": len(known)})
}

// privateSlash24 returns the /24 containing an allowed IPv4 address.
func privateSlash24(addr netip.Addr) (netip.Prefix, bool) {
	addr = addr.Unmap()
	if !addr.Is4() || !disc.Allowed(addr) {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(addr, 24).Masked(), true
}

func inventorySuggestions(known []netip.Addr, already map[netip.Prefix]bool) []Suggestion {
	counts := map[netip.Prefix]int{}
	for _, addr := range known {
		addr = addr.Unmap()
		if !addr.Is4() {
			continue
		}
		prefix := netip.PrefixFrom(addr, 24).Masked()
		counts[prefix]++
	}
	type ranked struct {
		prefix netip.Prefix
		count  int
	}
	rankedPrefixes := make([]ranked, 0, len(counts))
	for prefix, count := range counts {
		if !already[prefix] {
			rankedPrefixes = append(rankedPrefixes, ranked{prefix: prefix, count: count})
		}
	}
	sort.Slice(rankedPrefixes, func(i, j int) bool {
		if rankedPrefixes[i].count != rankedPrefixes[j].count {
			return rankedPrefixes[i].count > rankedPrefixes[j].count
		}
		return rankedPrefixes[i].prefix.Addr().Less(rankedPrefixes[j].prefix.Addr())
	})
	if len(rankedPrefixes) > 3 {
		rankedPrefixes = rankedPrefixes[:3]
	}
	out := make([]Suggestion, 0, len(rankedPrefixes))
	for _, item := range rankedPrefixes {
		out = append(out, Suggestion{CIDR: item.prefix.String(), Source: SourceInventory})
	}
	return out
}

// conflictResponse is the 409 body: the standard error envelope plus the scan
// that is already running.
type conflictResponse struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Scan    disc.Scan `json:"scan"`
}

// rateLimitedResponse is the 429 body: the standard error envelope plus how
// long to wait (the Retry-After header carries the same number).
type rateLimitedResponse struct {
	Code              string `json:"code"`
	Message           string `json:"message"`
	RetryAfterSeconds int    `json:"retryAfterSeconds"`
}

type startRequest struct {
	Ranges           requestRanges `json:"ranges"`
	IncludeInventory bool          `json:"includeInventory"`
	AskNetwork       bool          `json:"askNetwork"`
	CIDR             *string       `json:"cidr"`
}

type requestRanges struct {
	values  []string
	present bool
}

func (r *requestRanges) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("ranges must be an array")
	}
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	r.values = values
	r.present = true
	return nil
}

// Start handles POST /api/discovery/scan (behind elevation when step-up is on).
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	req, ok := httputil.DecodeJSON[startRequest](w, r)
	if !ok {
		return
	}

	rawRanges, legacy := startRanges(req)
	ranges, err := disc.ParseRanges(rawRanges)
	if err != nil {
		var re *disc.RangeError
		reason, msg, field := "invalid_cidr", "invalid range", "ranges"
		if errors.As(err, &re) {
			reason, msg, field = re.Reason, re.Message, re.Field
		}
		if legacy {
			field = "cidr"
		}
		h.rejectRanges(r, rawRanges, legacy, reason)
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", msg, []httputil.FieldError{{Field: field, Msg: msg}})
		return
	}
	includeInventory, askNetwork := req.IncludeInventory, req.AskNetwork
	if legacy {
		includeInventory, askNetwork = false, false
	}
	if len(ranges) == 0 && !includeInventory {
		msg := "enter a range or include known hosts"
		h.rejectRanges(r, rawRanges, legacy, disc.ReasonNoTargets)
		field := "ranges"
		if legacy {
			field = "cidr"
		}
		httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", msg, []httputil.FieldError{{Field: field, Msg: msg}})
		return
	}
	var inventory []netip.Addr
	if includeInventory {
		rows, err := h.inventoryQuery(r.Context())
		if err != nil {
			h.rejectRanges(r, rawRanges, legacy, "internal")
			slog.Error("read known hosts for network discovery", "errorType", fmt.Sprintf("%T", err))
			httputil.Error(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
			return
		}
		inventory = disc.InventorySource(rows, ranges)
	}

	// The start entry is written before the scan begins, so it always
	// precedes the scan's own end entry.
	recordStart := func(scan disc.Scan) {
		detail := map[string]any{
			"ranges":           scan.Ranges,
			"includeInventory": scan.IncludeInventory,
			"askNetwork":       scan.AskNetwork,
			"planned": map[string]int{
				"range":     scan.Sources.Range,
				"inventory": scan.Sources.Inventory,
			},
			"skipped": map[string]int{"inventory": scan.Skipped.Inventory},
		}
		if err := h.Store.RecordAuditFromContext(r.Context(), disc.AuditStart, disc.AuditTargetType, scan.ID, detail); err != nil {
			slog.Error("record discovery scan start audit entry", "errorType", fmt.Sprintf("%T", err))
		}
	}
	scan, err := h.Manager.Start(disc.StartRequest{
		Ranges:           ranges,
		IncludeInventory: includeInventory,
		AskNetwork:       askNetwork,
		Inventory:        inventory,
	}, actorFrom(r), recordStart)
	var running *disc.ScanRunningError
	var limited *disc.RateLimitError
	switch {
	case errors.As(err, &running):
		h.rejectRanges(r, rawRanges, legacy, "scan_in_progress")
		httputil.JSON(w, http.StatusConflict, conflictResponse{
			Code:    "scan_in_progress",
			Message: "A network scan is already running",
			Scan:    running.Scan,
		})
		return
	case errors.As(err, &limited):
		h.rejectRanges(r, rawRanges, legacy, "rate_limited")
		seconds := int(math.Ceil(limited.RetryAfter.Seconds()))
		w.Header().Set("Retry-After", strconv.Itoa(seconds))
		httputil.JSON(w, http.StatusTooManyRequests, rateLimitedResponse{
			Code:              "rate_limited",
			Message:           "Too many network scans, try again later",
			RetryAfterSeconds: seconds,
		})
		return
	case err != nil:
		var re *disc.RangeError
		if errors.As(err, &re) {
			field := re.Field
			if legacy {
				field = "cidr"
			}
			h.rejectRanges(r, rawRanges, legacy, re.Reason)
			httputil.ErrorWithDetails(w, http.StatusBadRequest, "invalid_request", re.Message, []httputil.FieldError{{Field: field, Msg: re.Message}})
			return
		}
		h.rejectRanges(r, rawRanges, legacy, "internal")
		slog.Error("start network discovery scan", "errorType", fmt.Sprintf("%T", err))
		httputil.Error(w, http.StatusInternalServerError, "internal_error", "An internal error occurred")
		return
	}

	httputil.JSON(w, http.StatusAccepted, scanResponse{Scan: scan})
}

func startRanges(req startRequest) ([]string, bool) {
	if req.Ranges.present {
		return req.Ranges.values, false
	}
	if req.CIDR != nil {
		return []string{*req.CIDR}, true
	}
	return []string{}, false
}

func actorFrom(r *http.Request) disc.Actor {
	return disc.Actor{
		UserID:        auth.UserIDFromContext(r.Context()),
		InstanceAdmin: auth.InstanceAdminFromContext(r.Context()),
	}
}

// reject writes the discovery.scan.reject audit entry. The range is what the
// admin submitted, cut to a sane length; no scanned host is involved.
func (h *Handler) rejectRanges(r *http.Request, submitted []string, legacy bool, reason string) {
	if legacy {
		value := ""
		if len(submitted) > 0 {
			value = submitted[0]
		}
		if len(value) > maxAuditRange {
			value = value[:maxAuditRange]
		}
		if err := h.Store.RecordAuditFromContext(r.Context(), disc.AuditReject, disc.AuditTargetType, "", map[string]any{"cidr": value, "reason": reason}); err != nil {
			slog.Error("record discovery scan reject audit entry", "errorType", fmt.Sprintf("%T", err))
		}
		return
	}
	if len(submitted) > disc.MaxRanges {
		submitted = submitted[:disc.MaxRanges]
	}
	ranges := make([]string, 0, len(submitted))
	for _, value := range submitted {
		if len(value) > maxAuditRange {
			value = value[:maxAuditRange]
		}
		ranges = append(ranges, value)
	}
	detail := map[string]any{"ranges": ranges, "reason": reason}
	if err := h.Store.RecordAuditFromContext(r.Context(), disc.AuditReject, disc.AuditTargetType, "", detail); err != nil {
		slog.Error("record discovery scan reject audit entry", "errorType", fmt.Sprintf("%T", err))
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
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	scan, ok := h.Manager.Cancel(actorFrom(r))
	if !ok {
		httputil.Error(w, http.StatusNotFound, "not_found", "No network scan is running")
		return
	}
	httputil.JSON(w, http.StatusOK, scanResponse{Scan: scan})
}
