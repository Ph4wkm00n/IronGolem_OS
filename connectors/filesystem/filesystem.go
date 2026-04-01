// Package filesystem implements a connector for local filesystem operations.
// This is one of the initial Phase 1 connectors.
package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Ph4wkm00n/IronGolem_OS/connectors"
)

// Connector provides filesystem read/write capabilities within a sandboxed
// root directory.
type Connector struct {
	mu          sync.RWMutex
	id          string
	rootDir     string
	health      connectors.HealthState
	connectedAt *time.Time
}

// New creates a new filesystem connector with the given ID.
func New(id string) *Connector {
	return &Connector{
		id:     id,
		health: connectors.HealthDisconnected,
	}
}

func (c *Connector) Type() connectors.ConnectorType {
	return connectors.TypeFilesystem
}

func (c *Connector) Connect(ctx context.Context, config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	rootDir, ok := config["root_dir"]
	if !ok {
		return fmt.Errorf("filesystem connector requires 'root_dir' config")
	}

	// Ensure root directory exists
	absPath, err := filepath.Abs(rootDir)
	if err != nil {
		return fmt.Errorf("invalid root_dir path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		if err := os.MkdirAll(absPath, 0o750); err != nil {
			return fmt.Errorf("failed to create root_dir: %w", err)
		}
	}

	c.rootDir = absPath
	c.health = connectors.HealthHealthy
	now := time.Now()
	c.connectedAt = &now

	return nil
}

func (c *Connector) Disconnect(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.health = connectors.HealthDisconnected
	c.connectedAt = nil
	return nil
}

func (c *Connector) Health(_ context.Context) connectors.HealthState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.health
}

func (c *Connector) Send(_ context.Context, msg *connectors.Message) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.health != connectors.HealthHealthy {
		return fmt.Errorf("connector is not healthy: %s", c.health)
	}

	// Extract file path from metadata
	filePath, ok := msg.Metadata["path"]
	if !ok {
		return fmt.Errorf("message metadata must include 'path'")
	}

	// Ensure path is within root directory (prevent path traversal)
	absPath, err := filepath.Abs(filepath.Join(c.rootDir, filePath))
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if !isSubPath(c.rootDir, absPath) {
		return fmt.Errorf("path traversal denied: path escapes root directory")
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Write the file
	if err := os.WriteFile(absPath, []byte(msg.Content), 0o640); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func (c *Connector) Receive(_ context.Context) (<-chan *connectors.Message, error) {
	// Filesystem connector doesn't have a continuous receive stream.
	// File watch could be implemented here in the future.
	ch := make(chan *connectors.Message)
	return ch, nil
}

func (c *Connector) Capabilities() []string {
	return []string{"file_read", "file_write", "file_list"}
}

// ReadFile reads a file from the sandboxed root directory.
func (c *Connector) ReadFile(path string) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	absPath, err := filepath.Abs(filepath.Join(c.rootDir, path))
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if !isSubPath(c.rootDir, absPath) {
		return nil, fmt.Errorf("path traversal denied: path escapes root directory")
	}

	return os.ReadFile(absPath)
}

// ListFiles lists files in a directory within the sandboxed root.
func (c *Connector) ListFiles(dir string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	absPath, err := filepath.Abs(filepath.Join(c.rootDir, dir))
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if !isSubPath(c.rootDir, absPath) {
		return nil, fmt.Errorf("path traversal denied: path escapes root directory")
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []string
	for _, entry := range entries {
		files = append(files, entry.Name())
	}
	return files, nil
}

// isSubPath checks if child is under parent directory (prevents path traversal).
func isSubPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	// Reject if relative path escapes parent via ".." components.
	return !strings.HasPrefix(rel, "..") && rel != "."
}
