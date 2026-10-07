package tlsprobe

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WiseLabz/wiselabz/internal/connector"
)

type certSpec struct {
	subject   pkix.Name
	dnsNames  []string
	ips       []net.IP
	notBefore time.Time
	notAfter  time.Time
	isCA      bool
}

var serialCounter atomic.Int64

// issue creates a certificate signed by parent (self-signed when parent is nil).
func issue(t *testing.T, spec certSpec, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(serialCounter.Add(1)),
		Subject:               spec.subject,
		DNSNames:              spec.dnsNames,
		IPAddresses:           spec.ips,
		NotBefore:             spec.notBefore,
		NotAfter:              spec.notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  spec.isCA,
	}
	signer, signerKey := tmpl, key
	if parent != nil {
		signer, signerKey = parent, parentKey
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signer, &key.PublicKey, signerKey)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key, der
}

func tlsCertificate(der []byte, key *ecdsa.PrivateKey, chain ...[]byte) tls.Certificate {
	return tls.Certificate{Certificate: append([][]byte{der}, chain...), PrivateKey: key}
}

// tlsServer is a local TLS listener that completes handshakes and closes.
type tlsServer struct {
	addr     string
	ln       net.Listener
	hello    chan string // server names sent by clients
	accepted atomic.Int32
	// postHandshakeRead records what the client sent after the handshake.
	postHandshakeRead chan readResult
}

type readResult struct {
	n   int
	err error
}

func startTLSServer(t *testing.T, cert tls.Certificate) *tlsServer {
	t.Helper()
	srv := &tlsServer{hello: make(chan string, 64), postHandshakeRead: make(chan readResult, 64)}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		GetConfigForClient: func(h *tls.ClientHelloInfo) (*tls.Config, error) {
			srv.hello <- h.ServerName
			return nil, nil
		},
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	srv.ln = ln
	srv.addr = ln.Addr().String()
	go func() {
		for {
			raw, err := ln.Accept()
			if err != nil {
				return
			}
			srv.accepted.Add(1)
			go func() {
				defer raw.Close() //nolint:errcheck
				_ = raw.SetDeadline(time.Now().Add(2 * time.Second))
				counted := &countingConn{Conn: raw}
				conn := tls.Server(counted, cfg)
				if err := conn.Handshake(); err != nil {
					return
				}
				// Count the raw bytes the client sends after its handshake
				// flight: application data and a close_notify alert are both
				// bytes on the wire.
				before := counted.read.Load()
				_, err := conn.Read(make([]byte, 16))
				srv.postHandshakeRead <- readResult{n: int(counted.read.Load() - before), err: err}
			}()
		}
	}()
	return srv
}

// countingConn counts the bytes read from the wire.
type countingConn struct {
	net.Conn
	read atomic.Int64
}

func (c *countingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.read.Add(int64(n))
	return n, err
}

func (s *tlsServer) port() int {
	_, p, _ := net.SplitHostPort(s.addr)
	n, _ := strconv.Atoi(p)
	return n
}

// startRawServer accepts TCP connections and runs handle on each.
func startRawServer(t *testing.T, handle func(net.Conn)) string {
	t.Helper()
	return startRawServerOn(t, "127.0.0.1:0", handle)
}

func startRawServerOn(t *testing.T, listen string, handle func(net.Conn)) string {
	t.Helper()
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	t.Cleanup(func() { _ = ln.Close(); wg.Wait() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() { defer wg.Done(); defer conn.Close(); handle(conn) }() //nolint:errcheck
		}
	}()
	return ln.Addr().String()
}

// closedPort returns a loopback address nothing listens on.
func closedPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func targetsConfig(lines ...string) map[string]any {
	text := ""
	for _, l := range lines {
		text += l + "\n"
	}
	return map[string]any{"targets": text}
}

func entityByID(t *testing.T, entities []connector.SnapshotEntity, id string) connector.SnapshotEntity {
	t.Helper()
	for _, e := range entities {
		if e.ExternalID == id {
			return e
		}
	}
	t.Fatalf("no entity %q", id)
	return connector.SnapshotEntity{}
}

func (s *tlsServer) close() { _ = s.ln.Close() }
