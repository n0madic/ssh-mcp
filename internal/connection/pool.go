package connection

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/n0madic/ssh-mcp/internal/config"
)

// SessionID uniquely identifies a connection as "user@host:port".
type SessionID string

// ConnectionInfo provides metadata about a connection.
type ConnectionInfo struct {
	SessionID    SessionID `json:"session_id"`
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	User         string    `json:"user"`
	ConnectedAt  time.Time `json:"connected_at"`
	LastUsed     time.Time `json:"last_used"`
	CommandCount int       `json:"command_count"`
	Connected    bool      `json:"connected"`
}

// Connection wraps an SSH client with metadata.
type Connection struct {
	mu           sync.RWMutex
	ID           SessionID
	Client       *ssh.Client
	Host         string
	Port         int
	User         string
	ConnectedAt  time.Time
	LastUsed     time.Time
	CommandCount int
	Connected    bool
	clientConfig *ssh.ClientConfig // stored for auto-reconnect (no raw password)
	addr         string            // stored for auto-reconnect
	ready        chan struct{}      // closed when connection attempt completes
	connectErr   error             // non-nil if the connection attempt failed
}

// Pool manages a thread-safe pool of SSH connections.
type Pool struct {
	mu    sync.RWMutex
	conns map[SessionID]*Connection
	auth  *AuthDiscovery
	cfg   *config.SSHConfig
}

// NewPool creates a new connection pool.
func NewPool(cfg *config.SSHConfig, auth *AuthDiscovery) *Pool {
	return &Pool{
		conns: make(map[SessionID]*Connection),
		auth:  auth,
		cfg:   cfg,
	}
}

// StartIdleCleanup starts a background goroutine that checks for idle connections.
func (p *Pool) StartIdleCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.cleanupIdle()
			}
		}
	}()
}

func (p *Pool) cleanupIdle() {
	p.mu.RLock()
	var toDisconnect []SessionID
	for id, conn := range p.conns {
		// Skip pending connections (not yet ready).
		select {
		case <-conn.ready:
		default:
			continue
		}
		conn.mu.RLock()
		if conn.Connected && time.Since(conn.LastUsed) > p.cfg.MaxIdleTime {
			toDisconnect = append(toDisconnect, id)
		}
		conn.mu.RUnlock()
	}
	p.mu.RUnlock()

	for _, id := range toDisconnect {
		log.Printf("Closing idle connection: %s", id)
		p.Disconnect(id)
	}
}

// MakeSessionID constructs a SessionID from user, host, and port.
func MakeSessionID(user, host string, port int) SessionID {
	return SessionID(fmt.Sprintf("%s@%s:%d", user, host, port))
}

// Connect establishes or reuses an SSH connection.
// It uses a reservation pattern: a pending entry is stored in the pool before
// dialing, so that concurrent GetConnection calls can wait for the connection
// to become ready instead of returning "session not found".
func (p *Pool) Connect(ctx context.Context, params ConnectParams) (SessionID, error) {
	id := MakeSessionID(params.User, params.Host, params.Port)

	// Check for existing connection (alive, dead, or pending).
	p.mu.RLock()
	existing, exists := p.conns[id]
	p.mu.RUnlock()

	if exists {
		// Wait for any pending connection attempt to complete first.
		select {
		case <-existing.ready:
		case <-ctx.Done():
			return "", ctx.Err()
		}

		if existing.connectErr != nil {
			// Previous attempt failed; remove and retry below.
			p.mu.Lock()
			if cur, ok := p.conns[id]; ok && cur == existing {
				delete(p.conns, id)
			}
			p.mu.Unlock()
		} else {
			existing.mu.RLock()
			alive := existing.Connected && p.isAlive(existing.Client)
			existing.mu.RUnlock()
			if alive {
				existing.mu.Lock()
				existing.LastUsed = time.Now()
				existing.mu.Unlock()
				return id, nil
			}
			// Dead connection, remove and reconnect.
			p.mu.Lock()
			if cur, ok := p.conns[id]; ok && cur == existing {
				delete(p.conns, id)
			}
			p.mu.Unlock()
			if existing.Client != nil {
				existing.Client.Close()
			}
		}
	}

	clientConfig, err := p.auth.BuildClientConfig(params)
	if err != nil {
		return "", fmt.Errorf("auth config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", params.Host, params.Port)

	// Create a pending connection reservation before dialing.
	pending := &Connection{
		ID:    id,
		Host:  params.Host,
		Port:  params.Port,
		User:  params.User,
		ready: make(chan struct{}),
	}

	p.mu.Lock()

	// Enforce max connections limit.
	if p.cfg.MaxConnections > 0 && len(p.conns) >= p.cfg.MaxConnections {
		// Don't count entries that are ours to replace.
		if _, replacing := p.conns[id]; !replacing {
			p.mu.Unlock()
			close(pending.ready) // signal so no one waits forever
			return "", fmt.Errorf("connection pool is full (max %d connections)", p.cfg.MaxConnections)
		}
	}

	// Check if another goroutine placed a reservation while we were building config.
	if existing, exists := p.conns[id]; exists {
		p.mu.Unlock()

		// Wait for the other attempt to finish.
		select {
		case <-existing.ready:
		case <-ctx.Done():
			return "", ctx.Err()
		}

		if existing.connectErr == nil {
			existing.mu.RLock()
			alive := existing.Connected && p.isAlive(existing.Client)
			existing.mu.RUnlock()
			if alive {
				existing.mu.Lock()
				existing.LastUsed = time.Now()
				existing.mu.Unlock()
				return id, nil
			}
		}

		// Failed or dead — remove and re-acquire lock to place our reservation.
		p.mu.Lock()
		if cur, ok := p.conns[id]; ok && cur == existing {
			delete(p.conns, id)
			if existing.Client != nil {
				existing.Client.Close()
			}
		} else if cur, ok := p.conns[id]; ok && cur != pending {
			// Yet another goroutine beat us; give up and let caller retry.
			p.mu.Unlock()
			close(pending.ready)
			return "", fmt.Errorf("concurrent connection attempt for %s, please retry", id)
		}
	}

	// Place our pending reservation in the pool.
	p.conns[id] = pending
	p.mu.Unlock()

	// Dial without holding the pool lock.
	client, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		pending.connectErr = fmt.Errorf("SSH dial %s: %w", addr, err)
		// Remove the failed reservation from the pool.
		p.mu.Lock()
		if cur, ok := p.conns[id]; ok && cur == pending {
			delete(p.conns, id)
		}
		p.mu.Unlock()
		close(pending.ready)
		return "", pending.connectErr
	}

	now := time.Now()
	pending.mu.Lock()
	pending.Client = client
	pending.Connected = true
	pending.ConnectedAt = now
	pending.LastUsed = now
	pending.clientConfig = clientConfig
	pending.addr = addr
	pending.mu.Unlock()

	close(pending.ready)
	return id, nil
}

// GetConnection retrieves a connection by ID, attempting auto-reconnect if dead.
// If a connection attempt is in progress, it waits for it to complete.
func (p *Pool) GetConnection(ctx context.Context, id SessionID) (*Connection, error) {
	p.mu.RLock()
	conn, exists := p.conns[id]
	p.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session %s not found", id)
	}

	// Wait for pending connection to complete.
	select {
	case <-conn.ready:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if conn.connectErr != nil {
		return nil, fmt.Errorf("session %s connection failed: %w", id, conn.connectErr)
	}

	conn.mu.RLock()
	alive := conn.Connected && p.isAlive(conn.Client)
	conn.mu.RUnlock()

	if alive {
		conn.mu.Lock()
		conn.LastUsed = time.Now()
		conn.mu.Unlock()
		return conn, nil
	}

	// Auto-reconnect using stored clientConfig (no raw credentials needed).
	log.Printf("Connection %s lost, attempting reconnect...", id)

	// Close old client.
	conn.mu.Lock()
	if conn.Client != nil {
		conn.Client.Close()
	}
	conn.Connected = false
	savedConfig := conn.clientConfig
	savedAddr := conn.addr
	conn.mu.Unlock()

	if savedConfig == nil {
		return nil, fmt.Errorf("cannot reconnect %s: no saved client config", id)
	}

	client, err := ssh.Dial("tcp", savedAddr, savedConfig)
	if err != nil {
		return nil, fmt.Errorf("reconnect SSH dial %s: %w", savedAddr, err)
	}

	conn.mu.Lock()
	conn.Client = client
	conn.Connected = true
	conn.LastUsed = time.Now()
	conn.mu.Unlock()

	log.Printf("Reconnected to %s", id)
	return conn, nil
}

// Disconnect closes and removes a connection.
// If a connection attempt is still pending, it waits for it to complete first.
func (p *Pool) Disconnect(id SessionID) error {
	p.mu.Lock()
	conn, exists := p.conns[id]
	if !exists {
		p.mu.Unlock()
		return fmt.Errorf("session %s not found", id)
	}
	delete(p.conns, id)
	p.mu.Unlock()

	// Wait for pending connection to complete before closing.
	<-conn.ready

	conn.mu.Lock()
	defer conn.mu.Unlock()

	conn.Connected = false
	if conn.Client != nil {
		return conn.Client.Close()
	}
	return nil
}

// ListConnections returns info about all connections.
// Pending connections (still being established) are included with Connected=false.
func (p *Pool) ListConnections() []ConnectionInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	infos := make([]ConnectionInfo, 0, len(p.conns))
	for _, conn := range p.conns {
		// Check if connection is still pending.
		select {
		case <-conn.ready:
			// Ready — read actual state.
			conn.mu.RLock()
			infos = append(infos, ConnectionInfo{
				SessionID:    conn.ID,
				Host:         conn.Host,
				Port:         conn.Port,
				User:         conn.User,
				ConnectedAt:  conn.ConnectedAt,
				LastUsed:     conn.LastUsed,
				CommandCount: conn.CommandCount,
				Connected:    conn.Connected,
			})
			conn.mu.RUnlock()
		default:
			// Still pending — report as connecting.
			infos = append(infos, ConnectionInfo{
				SessionID: conn.ID,
				Host:      conn.Host,
				Port:      conn.Port,
				User:      conn.User,
				Connected: false,
			})
		}
	}
	return infos
}

// CloseAll closes all connections (for graceful shutdown).
func (p *Pool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, conn := range p.conns {
		// Wait for pending connections before closing.
		<-conn.ready
		conn.mu.Lock()
		conn.Connected = false
		if conn.Client != nil {
			conn.Client.Close()
		}
		conn.mu.Unlock()
		delete(p.conns, id)
	}
}

// IncrementCommandCount increments the command counter for a connection.
func (c *Connection) IncrementCommandCount() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CommandCount++
}

func (p *Pool) isAlive(client *ssh.Client) bool {
	if client == nil {
		return false
	}
	_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
	return err == nil
}
