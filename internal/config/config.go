// Package config stores user-level settings: the connected AI and recent
// projects. Location: $FOUNDRY_HOME or ~/.foundry, file foundry.json.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/handrawanw/foundry/internal/safefile"
)

type ConnectionMode string

const (
	ModeAgent ConnectionMode = "agent"
	ModeMCP   ConnectionMode = "mcp"
)

type Connection struct {
	Mode    ConnectionMode `json:"mode"`
	AgentID string         `json:"agent_id,omitempty"`
	Command []string       `json:"command,omitempty"` // custom override of the preset command
	Model   string         `json:"model,omitempty"`   // empty: the agent's default
	Effort  string         `json:"effort,omitempty"`
	Agents  int            `json:"agents,omitempty"` // coding agents building in parallel; 0: default
	Since   time.Time      `json:"since"`
}

type MCPHeartbeat struct {
	Client string    `json:"client"`
	Dir    string    `json:"dir"`
	At     time.Time `json:"at"`
}

type Config struct {
	Connection *Connection   `json:"connection,omitempty"`
	MCP        *MCPHeartbeat `json:"mcp,omitempty"`
	Recent     []string      `json:"recent"`
}

var mu sync.Mutex

// ErrLocked means another process held the config lock past lockWait.
var ErrLocked = errors.New("config sedang dikunci proses lain")

func Home() string {
	if h := os.Getenv("FOUNDRY_HOME"); h != "" {
		return h
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".foundry"
	}
	return filepath.Join(home, ".foundry")
}

func path() string { return filepath.Join(Home(), "foundry.json") }

func Load() (*Config, error) {
	b, err := os.ReadFile(path())
	if errors.Is(err, os.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// lockWait bounds the wait for another Foundry process (UI, MCP) that is
// updating the config.
var lockWait = 10 * time.Second

// Update re-reads the config, applies fn and saves it atomically. The file
// lock serializes every process; mu only spares goroutines of this one the
// lock polling.
func Update(fn func(c *Config)) (*Config, error) {
	mu.Lock()
	defer mu.Unlock()
	if err := os.MkdirAll(Home(), 0o755); err != nil {
		return nil, err
	}
	l, err := safefile.Acquire(path()+".lock", lockWait)
	if errors.Is(err, safefile.ErrTimeout) {
		return nil, ErrLocked
	}
	if err != nil {
		return nil, err
	}
	defer l.Unlock()
	c, err := Load()
	if err != nil {
		return nil, err
	}
	fn(c)
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := safefile.WriteFile(path(), b, 0o600); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) AddRecent(dir string) {
	out := []string{dir}
	for _, d := range c.Recent {
		if d != dir && len(out) < 10 {
			out = append(out, d)
		}
	}
	c.Recent = out
}

// RemoveRecent drops dir from the recent list; an empty dir clears it.
func (c *Config) RemoveRecent(dir string) {
	if dir == "" {
		c.Recent = nil
		return
	}
	out := c.Recent[:0]
	for _, d := range c.Recent {
		if d != dir {
			out = append(out, d)
		}
	}
	c.Recent = out
}

// MCPAlive reports whether an AI host talked to `foundry mcp` recently.
func (c *Config) MCPAlive() bool {
	return c.MCP != nil && time.Since(c.MCP.At) < 10*time.Minute
}
