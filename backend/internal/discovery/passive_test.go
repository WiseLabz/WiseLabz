package discovery

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakePassiveClock struct{ now time.Time }

func (c fakePassiveClock) Now() time.Time { return c.now }

type fakePassiveInterfaceLister struct {
	interfaces []passiveInterface
	err        error
}

func (l fakePassiveInterfaceLister) List() ([]passiveInterface, error) {
	return l.interfaces, l.err
}

type passiveRead struct {
	payload []byte
	source  net.Addr
	err     error
}

type fakePassiveConn struct {
	mu         sync.Mutex
	reads      []passiveRead
	deadline   time.Time
	readCap    int
	readSizes  []int
	closed     chan struct{}
	closeOnce  sync.Once
	started    chan struct{}
	startOnce  sync.Once
	blockReads bool
}

func newFakePassiveConn(reads ...passiveRead) *fakePassiveConn {
	return &fakePassiveConn{reads: reads, closed: make(chan struct{}), started: make(chan struct{})}
}

func newBlockingFakePassiveConn() *fakePassiveConn {
	conn := newFakePassiveConn()
	conn.blockReads = true
	return conn
}

func (c *fakePassiveConn) ReadFrom(buffer []byte) (int, net.Addr, error) {
	c.startOnce.Do(func() { close(c.started) })
	if c.blockReads {
		<-c.closed
		return 0, nil, net.ErrClosed
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readCap = len(buffer)
	if len(c.reads) == 0 {
		return 0, nil, errors.New("no more fake datagrams")
	}
	read := c.reads[0]
	c.reads = c.reads[1:]
	if read.err != nil {
		return 0, nil, read.err
	}
	n := copy(buffer, read.payload)
	c.readSizes = append(c.readSizes, n)
	return n, read.source, nil
}

func (c *fakePassiveConn) SetReadDeadline(deadline time.Time) error {
	c.mu.Lock()
	c.deadline = deadline
	c.mu.Unlock()
	return nil
}

func (c *fakePassiveConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}

type sentPassiveQuery struct {
	device      net.Interface
	local       netip.Addr
	destination netip.AddrPort
	payload     []byte
}

type fakePassiveSender struct {
	mu      sync.Mutex
	queries []sentPassiveQuery
	conns   []*fakePassiveConn
}

func (s *fakePassiveSender) Open(_ context.Context, device net.Interface, local netip.Addr, destination netip.AddrPort, payload []byte) (passivePacketConn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := sentPassiveQuery{device: device, local: local, destination: destination, payload: append([]byte(nil), payload...)}
	s.queries = append(s.queries, query)
	if len(s.queries) > len(s.conns) {
		return nil, errors.New("no fake socket configured")
	}
	return s.conns[len(s.queries)-1], nil
}

func TestPassiveSourceQueriesAndReportsDistinctAllowedSources(t *testing.T) {
	now := time.Date(2026, time.October, 16, 12, 0, 0, 0, time.UTC)
	firstSource := netip.MustParseAddr("192.168.1.50")
	secondSource := netip.MustParseAddr("fd00::50")
	largePayload := []byte("LOCATION: http://10.20.30.40/" + strings.Repeat("x", 10000))
	conns := []*fakePassiveConn{
		newFakePassiveConn(passiveRead{payload: []byte("answer"), source: udpSource(firstSource)}),
		newFakePassiveConn(
			passiveRead{payload: largePayload, source: udpSource(firstSource)},
			passiveRead{payload: []byte("LOCATION: http://10.20.30.40/"), source: udpSource(netip.MustParseAddr("8.8.8.8"))},
		),
		newFakePassiveConn(
			passiveRead{payload: []byte("answer"), source: udpSource(netip.MustParseAddr("fe80::1"))},
			passiveRead{payload: []byte("answer"), source: udpSource(netip.MustParseAddr("::ffff:192.168.1.50"))},
			passiveRead{payload: []byte("answer"), source: udpSource(secondSource.WithZone("eth-test"))},
		),
		newFakePassiveConn(),
	}
	sender := &fakePassiveSender{conns: conns}
	iface := net.Interface{Name: "eth-test", Index: 7, Flags: net.FlagUp | net.FlagMulticast}
	source := &PassiveSource{
		interfaces: fakePassiveInterfaceLister{interfaces: []passiveInterface{
			{device: iface, addresses: []netip.Addr{
				netip.MustParseAddr("192.168.1.2"),
				netip.MustParseAddr("192.168.1.3"),
				netip.MustParseAddr("2001:db8::1"),
				netip.MustParseAddr("fd00::2"),
			}},
			{device: net.Interface{Name: "down-test", Index: 8, Flags: net.FlagMulticast}, addresses: []netip.Addr{netip.MustParseAddr("10.0.0.2")}},
		}},
		sender: sender,
		clock:  fakePassiveClock{now: now},
	}

	got := make(map[netip.Addr]int)
	source.Run(context.Background(), func(addr netip.Addr) { got[addr]++ })
	if !reflect.DeepEqual(got, map[netip.Addr]int{firstSource: 1, secondSource: 1}) {
		t.Fatalf("reported sources = %v, want each allowed source exactly once", got)
	}
	if len(sender.queries) != 4 {
		t.Fatalf("sent %d queries, want one mDNS and SSDP query per eligible family", len(sender.queries))
	}
	wantDestinations := []string{
		"224.0.0.251:5353",
		"239.255.255.250:1900",
		"[ff02::fb%eth-test]:5353",
		"[ff02::c%eth-test]:1900",
	}
	wantMDNS := []byte{
		0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0,
		9, '_', 's', 'e', 'r', 'v', 'i', 'c', 'e', 's',
		7, '_', 'd', 'n', 's', '-', 's', 'd',
		4, '_', 'u', 'd', 'p',
		5, 'l', 'o', 'c', 'a', 'l', 0,
		0, 12, 0, 1,
	}
	wantPayloads := [][]byte{
		wantMDNS,
		[]byte("M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: ssdp:all\r\n\r\n"),
		wantMDNS,
		[]byte("M-SEARCH * HTTP/1.1\r\nHOST: [FF02::C]:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: ssdp:all\r\n\r\n"),
	}
	for i, query := range sender.queries {
		if query.device.Name != "eth-test" {
			t.Errorf("query %d interface = %q, want eth-test", i, query.device.Name)
		}
		if got := query.destination.String(); got != wantDestinations[i] {
			t.Errorf("query %d destination = %q, want %q", i, got, wantDestinations[i])
		}
		if !reflect.DeepEqual(query.payload, wantPayloads[i]) {
			t.Errorf("query %d payload = %q, want %q", i, query.payload, wantPayloads[i])
		}
		if got, want := query.local, []netip.Addr{netip.MustParseAddr("192.168.1.2"), netip.MustParseAddr("192.168.1.2"), netip.MustParseAddr("fd00::2"), netip.MustParseAddr("fd00::2")}[i]; got != want {
			t.Errorf("query %d local address = %s, want %s", i, got, want)
		}
	}
	for i, conn := range conns {
		conn.mu.Lock()
		deadline, readCap := conn.deadline, conn.readCap
		readSizes := append([]int(nil), conn.readSizes...)
		conn.mu.Unlock()
		if want := now.Add(PassiveWindow); !deadline.Equal(want) {
			t.Errorf("socket %d deadline = %s, want shared deadline %s", i, deadline, want)
		}
		if readCap != passiveReadCap {
			t.Errorf("socket %d read buffer = %d bytes, want %d", i, readCap, passiveReadCap)
		}
		if i == 1 && (len(readSizes) == 0 || readSizes[0] != passiveReadCap) {
			t.Errorf("oversized datagram read sizes = %v, want first read capped at %d", readSizes, passiveReadCap)
		}
	}
}

func TestPassiveQueriesEachAllowedAddressFamilyIndependently(t *testing.T) {
	tests := []struct {
		name      string
		addresses []netip.Addr
		want      []string
	}{
		{
			name:      "IPv4 only",
			addresses: []netip.Addr{netip.MustParseAddr("192.168.1.2")},
			want:      []string{"224.0.0.251:5353", "239.255.255.250:1900"},
		},
		{
			name:      "ULA IPv6 only",
			addresses: []netip.Addr{netip.MustParseAddr("fd00::2")},
			want:      []string{"[ff02::fb%eth-test]:5353", "[ff02::c%eth-test]:1900"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := queriesFor(passiveInterface{
				device:    net.Interface{Name: "eth-test"},
				addresses: tt.addresses,
			})
			got := make([]string, len(queries))
			for i, query := range queries {
				got[i] = query.destination.String()
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("query destinations = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPassiveSourceCancellationClosesReaders(t *testing.T) {
	conns := []*fakePassiveConn{newBlockingFakePassiveConn(), newBlockingFakePassiveConn()}
	source := &PassiveSource{
		interfaces: fakePassiveInterfaceLister{interfaces: []passiveInterface{{
			device:    net.Interface{Name: "eth-test", Index: 7, Flags: net.FlagUp | net.FlagMulticast},
			addresses: []netip.Addr{netip.MustParseAddr("192.168.1.2")},
		}}},
		sender: &fakePassiveSender{conns: conns},
		clock:  fakePassiveClock{now: time.Now()},
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		source.Run(ctx, nil)
		close(done)
	}()
	for _, conn := range conns {
		select {
		case <-conn.started:
		case <-time.After(time.Second):
			t.Fatal("reader did not start")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancellation")
	}
	for i, conn := range conns {
		select {
		case <-conn.closed:
		default:
			t.Errorf("socket %d was not closed", i)
		}
	}
}

func TestPassiveSourceCallbackPanicDoesNotEscapeRun(t *testing.T) {
	source := &PassiveSource{
		interfaces: fakePassiveInterfaceLister{interfaces: []passiveInterface{{
			device:    net.Interface{Name: "eth-test", Index: 7, Flags: net.FlagUp | net.FlagMulticast},
			addresses: []netip.Addr{netip.MustParseAddr("192.168.1.2")},
		}}},
		sender: &fakePassiveSender{conns: []*fakePassiveConn{
			newFakePassiveConn(passiveRead{payload: []byte("answer"), source: udpSource(netip.MustParseAddr("192.168.1.50"))}),
		}},
		clock: fakePassiveClock{now: time.Now()},
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		source.Run(context.Background(), func(netip.Addr) { panic("boom") })
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after the callback panicked")
	}
}

func TestPassiveSourceCallbackPanicDoesNotBlockOtherReaders(t *testing.T) {
	first, second := netip.MustParseAddr("192.168.1.50"), netip.MustParseAddr("192.168.1.51")
	source := &PassiveSource{
		interfaces: fakePassiveInterfaceLister{interfaces: []passiveInterface{{
			device:    net.Interface{Name: "eth-test", Index: 7, Flags: net.FlagUp | net.FlagMulticast},
			addresses: []netip.Addr{netip.MustParseAddr("192.168.1.2")},
		}}},
		sender: &fakePassiveSender{conns: []*fakePassiveConn{
			newFakePassiveConn(passiveRead{payload: []byte("answer"), source: udpSource(first)}),
			newFakePassiveConn(passiveRead{payload: []byte("answer"), source: udpSource(second)}),
		}},
		clock: fakePassiveClock{now: time.Now()},
	}
	var mu sync.Mutex
	var calls []netip.Addr
	done := make(chan struct{})
	go func() {
		defer close(done)
		source.Run(context.Background(), func(addr netip.Addr) {
			mu.Lock()
			calls = append(calls, addr)
			n := len(calls)
			mu.Unlock()
			if n == 1 {
				panic("boom")
			}
		})
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run hung after a callback panic")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Errorf("callbacks = %v, want both addresses delivered", calls)
	}
}

func TestSystemPacketSenderLocalUDPPair(t *testing.T) {
	interfaces, err := (systemInterfaceLister{}).List()
	if err != nil {
		t.Fatalf("list interfaces: %v", err)
	}
	var device net.Interface
	var local netip.Addr
	for _, iface := range interfaces {
		if iface.device.Flags&net.FlagUp == 0 || iface.device.Flags&net.FlagMulticast == 0 {
			continue
		}
		for _, addr := range iface.addresses {
			addr = addr.Unmap()
			if addr.Is4() && Allowed(addr) {
				device, local = iface.device, addr
				break
			}
		}
		if local.IsValid() {
			break
		}
	}
	if !local.IsValid() {
		t.Skip("no up multicast interface has an allowed local IPv4 address")
	}

	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IP(local.AsSlice())})
	if err != nil {
		t.Fatalf("bind local UDP peer on %s: %v", local, err)
	}
	defer func() { _ = peer.Close() }()

	query := []byte("local UDP pair")
	peerAddr := peer.LocalAddr().(*net.UDPAddr)
	conn, err := (systemPacketSender{}).Open(context.Background(), device, local, peerAddr.AddrPort(), query)
	if err != nil {
		t.Fatalf("open local passive UDP socket on %s: %v", local, err)
	}
	defer func() { _ = conn.Close() }()
	udpConn := conn.(*net.UDPConn)
	localSocket := udpConn.LocalAddr().(*net.UDPAddr)
	if localSocket.Port == 5353 || localSocket.Port == 1900 {
		t.Fatalf("passive socket used reserved port %d", localSocket.Port)
	}

	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 128)
	n, source, err := peer.ReadFromUDP(buffer)
	if err != nil {
		t.Fatalf("receive local request: %v", err)
	}
	if string(buffer[:n]) != string(query) || source.IP.String() != local.String() {
		t.Fatalf("received %q from %s, want %q from %s", buffer[:n], source.IP, query, local)
	}

	response := []byte("local UDP response")
	if _, err := peer.WriteToUDP(response, source); err != nil {
		t.Fatalf("send local response: %v", err)
	}
	if err := udpConn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	n, replySource, err := udpConn.ReadFrom(buffer)
	if err != nil {
		t.Fatalf("receive local response: %v", err)
	}
	if string(buffer[:n]) != string(response) || replySource.(*net.UDPAddr).IP.String() != local.String() {
		t.Fatalf("received %q from %s, want %q from %s", buffer[:n], replySource, response, local)
	}
}

func udpSource(addr netip.Addr) net.Addr {
	return &net.UDPAddr{IP: net.IP(addr.AsSlice()), Port: 5353, Zone: addr.Zone()}
}
