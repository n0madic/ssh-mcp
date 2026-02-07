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
func (p *Pool) Connect(ctx context.Context, params ConnectParams) (SessionID, error) {
	id := MakeSessionID(params.User, params.Host, params.Port)

	// Check for existing alive connection.
	p.mu.RLock()
	existing, exists := p.conns[id]
	p.mu.RUnlock()

	if exists {
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
		delete(p.conns, id)
		p.mu.Unlock()
		if existing.Client != nil {
			existing.Client.Close()
		}
	}

	clientConfig, err := p.auth.BuildClientConfig(params)
	if err != nil {
		return "", fmt.Errorf("auth config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", params.Host, params.Port)

	// Acquire write lock to create connection.
	p.mu.Lock()

	// Enforce max connections limit (must be checked while holding lock).
	if p.cfg.MaxConnections > 0 {
		count := len(p.conns)
		if count >= p.cfg.MaxConnections {
			p.mu.Unlock()
			return "", fmt.Errorf("connection pool is full (max %d connections)", p.cfg.MaxConnections)
		}
	}
	// Double-check: did someone else connect while we were preparing?
	if existing, exists := p.conns[id]; exists {
		p.mu.Unlock() // Unlock before checking liveliness to avoid holding lock during network ops (if any)

		existing.mu.RLock()
		alive := existing.Connected && p.isAlive(existing.Client)
		existing.mu.RUnlock()

		if alive {
			existing.mu.Lock()
			existing.LastUsed = time.Now()
			existing.mu.Unlock()
			return id, nil
		}

		// It's dead, we need to replace it. Re-acquire lock to delete/overwrite.
		p.mu.Lock()
		// Re-check existence again mostly for safety, though unlikely to change state from dead to alive spontaneously without us.
		if existing2, exists2 := p.conns[id]; exists2 && existing2 == existing {
			delete(p.conns, id)
			if existing.Client != nil {
				existing.Client.Close()
			}
		}
	}
	// We hold the lock here.

	// We have to dial *without* the lock held to avoid blocking the whole pool,
	// but that opens the race window again.
	// The standard pattern for a connection pool is:
	// 1. Lock
	// 2. Check if exists
	// 3. If not, create a "placeholder" or reservation
	// 4. Unlock
	// 5. Dial
	// 6. Lock
	// 7. Store connection
	// 8. Unlock
	//
	// However, for simplicity in this existing codebase, we'll dial first (optimistically)
	// and then check again on insert. If we lose the race, we close our new connection and use the winner.
	p.mu.Unlock()

	client, err := ssh.Dial("tcp", addr, clientConfig)
	if err != nil {
		return "", fmt.Errorf("SSH dial %s: %w", addr, err)
	}

	now := time.Now()
	conn := &Connection{
		ID:           id,
		Client:       client,
		Host:         params.Host,
		Port:         params.Port,
		User:         params.User,
		ConnectedAt:  now,
		LastUsed:     now,
		Connected:    true,
		clientConfig: clientConfig,
		addr:         addr,
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Final check before inserting.
	if existing, exists := p.conns[id]; exists {
		// Someone beat us to it.
		existing.mu.RLock()
		alive := existing.Connected && p.isAlive(existing.Client)
		existing.mu.RUnlock()

		if alive {
			// The existing one is good. Close ours and return existing.
			client.Close()
			existing.mu.Lock()
			existing.LastUsed = time.Now()
			existing.mu.Unlock()
			return id, nil
		}
		// Existing is dead, overwrite it.
		if existing.Client != nil {
			existing.Client.Close()
		}
	}

	p.conns[id] = conn
	return id, nil
}

// GetConnection retrieves a connection by ID, attempting auto-reconnect if dead.
func (p *Pool) GetConnection(ctx context.Context, id SessionID) (*Connection, error) {
	p.mu.RLock()
	conn, exists := p.conns[id]
	p.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("session %s not found", id)
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
func (p *Pool) Disconnect(id SessionID) error {
	p.mu.Lock()
	conn, exists := p.conns[id]
	if !exists {
		p.mu.Unlock()
		return fmt.Errorf("session %s not found", id)
	}
	delete(p.conns, id)
	p.mu.Unlock()

	conn.mu.Lock()
	defer conn.mu.Unlock()

	conn.Connected = false
	if conn.Client != nil {
		return conn.Client.Close()
	}
	return nil
}

// ListConnections returns info about all connections.
func (p *Pool) ListConnections() []ConnectionInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	infos := make([]ConnectionInfo, 0, len(p.conns))
	for _, conn := range p.conns {
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
	}
	return infos
}

// CloseAll closes all connections (for graceful shutdown).
func (p *Pool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, conn := range p.conns {
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
