package tunnel

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

// TunnelID uniquely identifies an SSH tunnel.
type TunnelID string

// TunnelSession represents an active local port forwarding tunnel.
type TunnelSession struct {
	ID         TunnelID
	SessionID  string
	LocalAddr  string
	LocalPort  int
	RemoteAddr string

	listener  net.Listener
	sshClient *ssh.Client
	connCount atomic.Int64
	createdAt time.Time
	lastUsed  time.Time

	mu     sync.Mutex
	closed bool
	done   chan struct{}
	wg     sync.WaitGroup
}

// TunnelInfo provides read-only metadata about an active tunnel.
type TunnelInfo struct {
	ID         TunnelID
	SessionID  string
	LocalAddr  string
	LocalPort  int
	RemoteAddr string
	ConnCount  int64
	CreatedAt  time.Time
	LastUsed   time.Time
}

// TunnelPool manages active SSH tunnels.
type TunnelPool struct {
	mu         sync.RWMutex
	tunnels    map[TunnelID]*TunnelSession
	counter    atomic.Int64
	maxTunnels int
}

// NewTunnelPool creates a new TunnelPool.
// maxTunnels limits the number of concurrent tunnels (0 = unlimited).
func NewTunnelPool(maxTunnels int) *TunnelPool {
	return &TunnelPool{
		tunnels:    make(map[TunnelID]*TunnelSession),
		maxTunnels: maxTunnels,
	}
}

// Open creates a new local port forwarding tunnel.
// localPort of 0 means auto-assign a free port. remoteAddr is the address the
// SSH server should dial (e.g. "localhost:5432").
func (tp *TunnelPool) Open(sessionID string, client *ssh.Client, localPort int, remoteAddr string) (*TunnelSession, error) {
	// Bind local listener.
	listenAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", listenAddr, err)
	}

	// Parse the actual port assigned.
	_, portStr, _ := net.SplitHostPort(listener.Addr().String())
	var actualPort int
	fmt.Sscanf(portStr, "%d", &actualPort)

	id := TunnelID(fmt.Sprintf("%s-%d", sessionID, tp.counter.Add(1)))
	now := time.Now()

	ts := &TunnelSession{
		ID:         id,
		SessionID:  sessionID,
		LocalAddr:  listener.Addr().String(),
		LocalPort:  actualPort,
		RemoteAddr: remoteAddr,
		listener:   listener,
		sshClient:  client,
		createdAt:  now,
		lastUsed:   now,
		done:       make(chan struct{}),
	}

	// Check pool limit with lock held before inserting.
	tp.mu.Lock()
	if tp.maxTunnels > 0 && len(tp.tunnels) >= tp.maxTunnels {
		tp.mu.Unlock()
		listener.Close()
		return nil, fmt.Errorf("maximum number of tunnels (%d) reached", tp.maxTunnels)
	}
	tp.tunnels[id] = ts
	tp.mu.Unlock()

	// Start accept loop.
	ts.wg.Add(1)
	go ts.acceptLoop()

	return ts, nil
}

// acceptLoop accepts local connections and forwards them to the remote host.
func (ts *TunnelSession) acceptLoop() {
	defer ts.wg.Done()

	for {
		localConn, err := ts.listener.Accept()
		if err != nil {
			ts.mu.Lock()
			isClosed := ts.closed
			ts.mu.Unlock()
			if !isClosed {
				log.Printf("tunnel %s accept error: %v", ts.ID, err)
			}
			return
		}

		ts.mu.Lock()
		ts.lastUsed = time.Now()
		ts.mu.Unlock()
		ts.connCount.Add(1)

		ts.wg.Add(1)
		go func() {
			defer ts.wg.Done()
			ts.forward(localConn)
		}()
	}
}

// forward establishes a connection to the remote address via SSH and copies
// data bidirectionally between the local and remote connections.
func (ts *TunnelSession) forward(localConn net.Conn) {
	remoteConn, err := ts.sshClient.Dial("tcp", ts.RemoteAddr)
	if err != nil {
		log.Printf("tunnel %s: dial remote %s: %v", ts.ID, ts.RemoteAddr, err)
		localConn.Close()
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// local → remote
	go func() {
		defer wg.Done()
		io.Copy(remoteConn, localConn)
		// Signal the other direction that we're done writing.
		if tc, ok := remoteConn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	// remote → local
	go func() {
		defer wg.Done()
		io.Copy(localConn, remoteConn)
		if tc, ok := localConn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	wg.Wait()
	localConn.Close()
	remoteConn.Close()
}

// Get retrieves a TunnelSession by ID.
func (tp *TunnelPool) Get(id TunnelID) (*TunnelSession, error) {
	tp.mu.RLock()
	ts, ok := tp.tunnels[id]
	tp.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tunnel %s not found", id)
	}
	return ts, nil
}

// List returns metadata for all active tunnels. If sessionID is non-empty,
// only tunnels belonging to that session are included.
func (tp *TunnelPool) List(sessionID string) []TunnelInfo {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	result := make([]TunnelInfo, 0, len(tp.tunnels))
	for _, ts := range tp.tunnels {
		if sessionID != "" && ts.SessionID != sessionID {
			continue
		}
		ts.mu.Lock()
		info := TunnelInfo{
			ID:         ts.ID,
			SessionID:  ts.SessionID,
			LocalAddr:  ts.LocalAddr,
			LocalPort:  ts.LocalPort,
			RemoteAddr: ts.RemoteAddr,
			ConnCount:  ts.connCount.Load(),
			CreatedAt:  ts.createdAt,
			LastUsed:   ts.lastUsed,
		}
		ts.mu.Unlock()
		result = append(result, info)
	}
	return result
}

// closeTunnel closes a tunnel session's resources.
func closeTunnel(ts *TunnelSession) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.closed {
		return
	}
	ts.closed = true
	ts.listener.Close()
	close(ts.done)
}

// Close terminates a tunnel and removes it from the pool.
func (tp *TunnelPool) Close(id TunnelID) error {
	tp.mu.Lock()
	ts, ok := tp.tunnels[id]
	if ok {
		delete(tp.tunnels, id)
	}
	tp.mu.Unlock()

	if !ok {
		return fmt.Errorf("tunnel %s not found", id)
	}

	closeTunnel(ts)
	return nil
}

// CloseBySession closes all tunnels associated with the given session ID.
func (tp *TunnelPool) CloseBySession(sessionID string) {
	tp.mu.Lock()
	var toClose []*TunnelSession
	for id, ts := range tp.tunnels {
		if ts.SessionID == sessionID {
			toClose = append(toClose, ts)
			delete(tp.tunnels, id)
		}
	}
	tp.mu.Unlock()

	for _, ts := range toClose {
		closeTunnel(ts)
	}
}

// CloseAll terminates all tunnels (used during server shutdown).
func (tp *TunnelPool) CloseAll() {
	tp.mu.Lock()
	tunnels := make(map[TunnelID]*TunnelSession, len(tp.tunnels))
	for id, ts := range tp.tunnels {
		tunnels[id] = ts
	}
	tp.tunnels = make(map[TunnelID]*TunnelSession)
	tp.mu.Unlock()

	for _, ts := range tunnels {
		closeTunnel(ts)
	}
}
