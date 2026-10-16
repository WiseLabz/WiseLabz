package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	// PassiveWindow bounds how long network answers are collected.
	PassiveWindow  = 3 * time.Second
	passiveReadCap = 9000
)

// PassiveSource sends one mDNS and SSDP multicast query per eligible interface
// and address family, then reports distinct allowed UDP source addresses.
type PassiveSource struct {
	interfaces passiveInterfaceLister
	sender     passivePacketSender
	clock      passiveClock
}

// NewPassiveSource creates a source backed by the host's interfaces and UDP
// sockets.
func NewPassiveSource() *PassiveSource {
	return &PassiveSource{
		interfaces: systemInterfaceLister{},
		sender:     systemPacketSender{},
		clock:      wallClock{},
	}
}

type passiveInterface struct {
	device    net.Interface
	addresses []netip.Addr
}

type passiveInterfaceLister interface {
	List() ([]passiveInterface, error)
}

type passivePacketConn interface {
	ReadFrom([]byte) (int, net.Addr, error)
	SetReadDeadline(time.Time) error
	Close() error
}

type passivePacketSender interface {
	Open(context.Context, net.Interface, netip.Addr, netip.AddrPort, []byte) (passivePacketConn, error)
}

type passiveClock interface {
	Now() time.Time
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

type systemInterfaceLister struct{}

func (systemInterfaceLister) List() ([]passiveInterface, error) {
	devices, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	result := make([]passiveInterface, 0, len(devices))
	for _, device := range devices {
		addresses, err := device.Addrs()
		if err != nil {
			continue
		}
		entry := passiveInterface{device: device}
		for _, address := range addresses {
			prefix, err := netip.ParsePrefix(address.String())
			if err == nil {
				entry.addresses = append(entry.addresses, prefix.Addr())
			}
		}
		result = append(result, entry)
	}
	return result, nil
}

type systemPacketSender struct{}

func (systemPacketSender) Open(ctx context.Context, device net.Interface, local netip.Addr, destination netip.AddrPort, payload []byte) (passivePacketConn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !local.IsValid() || !destination.IsValid() || local.Is4() != destination.Addr().Is4() {
		return nil, fmt.Errorf("passive UDP address family mismatch")
	}

	network := "udp6"
	if local.Is4() {
		network = "udp4"
	}
	conn, err := listenPassiveUDP(network, net.UDPAddrFromAddrPort(netip.AddrPortFrom(local, 0)))
	if err != nil {
		return nil, err
	}
	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()
	closeOnError := func(err error) (passivePacketConn, error) {
		stopClose()
		_ = conn.Close()
		return nil, err
	}

	if local.Is4() {
		err = ipv4.NewPacketConn(conn).SetMulticastInterface(&device)
	} else {
		err = ipv6.NewPacketConn(conn).SetMulticastInterface(&device)
	}
	if err != nil {
		return closeOnError(err)
	}
	if err := ctx.Err(); err != nil {
		return closeOnError(err)
	}

	// These fixed multicast probes bypass the guarded dialer, which blocks
	// multicast by design. Passive discovery never sends unicast to a target.
	if _, err := conn.WriteToUDP(payload, net.UDPAddrFromAddrPort(destination)); err != nil {
		return closeOnError(err)
	}
	if err := ctx.Err(); err != nil {
		return closeOnError(err)
	}
	if !stopClose() {
		if err := ctx.Err(); err != nil {
			return closeOnError(err)
		}
		return closeOnError(context.Canceled)
	}
	return conn, nil
}

func listenPassiveUDP(network string, local *net.UDPAddr) (*net.UDPConn, error) {
	for range 8 {
		conn, err := net.ListenUDP(network, local)
		if err != nil {
			return nil, err
		}
		port := conn.LocalAddr().(*net.UDPAddr).Port
		if port != 5353 && port != 1900 {
			return conn, nil
		}
		_ = conn.Close()
	}
	return nil, fmt.Errorf("could not allocate a passive UDP port outside 5353 and 1900")
}

type passiveQuery struct {
	local       netip.Addr
	destination netip.AddrPort
	payload     []byte
}

// Run listens for up to PassiveWindow after sending the queries. The callback
// receives each distinct allowed UDP source address once; payloads are never
// inspected. Callbacks are serialized and Run waits for them to finish.
func (s *PassiveSource) Run(ctx context.Context, onAddress func(netip.Addr)) {
	if s == nil || ctx.Err() != nil {
		return
	}
	interfaces, err := s.interfaces.List()
	if err != nil {
		return
	}

	var conns []passivePacketConn
	for _, iface := range interfaces {
		if ctx.Err() != nil {
			break
		}
		if iface.device.Flags&net.FlagUp == 0 || iface.device.Flags&net.FlagMulticast == 0 {
			continue
		}
		for _, query := range queriesFor(iface) {
			conn, err := s.sender.Open(ctx, iface.device, query.local, query.destination, query.payload)
			if err == nil {
				conns = append(conns, conn)
			}
		}
	}
	if len(conns) == 0 {
		return
	}

	deadline := s.clock.Now().Add(PassiveWindow)
	var closeOnce sync.Once
	closeAll := func() {
		closeOnce.Do(func() {
			for _, conn := range conns {
				_ = conn.Close()
			}
		})
	}
	stopClose := context.AfterFunc(ctx, closeAll)
	defer func() {
		stopClose()
		closeAll()
	}()

	seen := make(map[netip.Addr]struct{})
	var seenMu, callbackMu sync.Mutex
	var readers sync.WaitGroup
	for _, conn := range conns {
		readers.Add(1)
		go func(conn passivePacketConn) {
			defer readers.Done()
			defer func() {
				if rec := recover(); rec != nil {
					// The address is deliberately not logged.
					slog.Error("network discovery passive reader panicked")
				}
			}()
			if conn.SetReadDeadline(deadline) != nil {
				return
			}
			buffer := make([]byte, passiveReadCap)
			for {
				_, source, err := conn.ReadFrom(buffer)
				if err != nil {
					return
				}
				addr, ok := passiveSourceAddress(source)
				if !ok || !Allowed(addr) {
					continue
				}
				addr = addr.Unmap()
				seenMu.Lock()
				_, duplicate := seen[addr]
				if !duplicate {
					seen[addr] = struct{}{}
				}
				seenMu.Unlock()
				if duplicate || onAddress == nil || ctx.Err() != nil {
					continue
				}
				callbackMu.Lock()
				if ctx.Err() == nil {
					onAddress(addr)
				}
				callbackMu.Unlock()
			}
		}(conn)
	}
	readers.Wait()
}

func queriesFor(iface passiveInterface) []passiveQuery {
	var ipv4Address, ipv6Address netip.Addr
	for _, address := range iface.addresses {
		address = address.Unmap()
		if !Allowed(address) {
			continue
		}
		if address.Is4() && !ipv4Address.IsValid() {
			ipv4Address = address
		} else if address.Is6() && !ipv6Address.IsValid() {
			ipv6Address = address
		}
	}

	queries := make([]passiveQuery, 0, 4)
	if ipv4Address.IsValid() {
		queries = append(queries,
			passiveQuery{local: ipv4Address, destination: netip.MustParseAddrPort("224.0.0.251:5353"), payload: mdnsQuery()},
			passiveQuery{local: ipv4Address, destination: netip.MustParseAddrPort("239.255.255.250:1900"), payload: ssdpQuery(false)},
		)
	}
	if ipv6Address.IsValid() {
		mdns := netip.MustParseAddr("ff02::fb").WithZone(iface.device.Name)
		ssdp := netip.MustParseAddr("ff02::c").WithZone(iface.device.Name)
		queries = append(queries,
			passiveQuery{local: ipv6Address, destination: netip.AddrPortFrom(mdns, 5353), payload: mdnsQuery()},
			passiveQuery{local: ipv6Address, destination: netip.AddrPortFrom(ssdp, 1900), payload: ssdpQuery(true)},
		)
	}
	return queries
}

func mdnsQuery() []byte {
	return []byte{
		0, 0, // ID
		0, 0, // flags
		0, 1, // questions
		0, 0, 0, 0, 0, 0, // resource records
		9, '_', 's', 'e', 'r', 'v', 'i', 'c', 'e', 's',
		7, '_', 'd', 'n', 's', '-', 's', 'd',
		4, '_', 'u', 'd', 'p',
		5, 'l', 'o', 'c', 'a', 'l',
		0,
		0, 12, // PTR
		0, 1, // IN
	}
}

func ssdpQuery(ipv6 bool) []byte {
	host := "239.255.255.250:1900"
	if ipv6 {
		host = "[FF02::C]:1900"
	}
	return []byte(fmt.Sprintf("M-SEARCH * HTTP/1.1\r\nHOST: %s\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: ssdp:all\r\n\r\n", host))
}

func passiveSourceAddress(source net.Addr) (netip.Addr, bool) {
	udp, ok := source.(*net.UDPAddr)
	if !ok || udp == nil {
		return netip.Addr{}, false
	}
	addr, ok := netip.AddrFromSlice(udp.IP)
	if !ok {
		return netip.Addr{}, false
	}
	// Zone identifies the receiving interface, not another target address.
	// Allowed still rejects link-local sources after dropping that metadata.
	return addr, true
}
