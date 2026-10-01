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

func Update(fn func(c *Config)) (*Config, error) {
	mu.Lock()
	defer mu.Unlock()
	c, err := Load()
	if err != nil {
		return nil, err
	}
	fn(c)
	if err := os.MkdirAll(Home(), 0o755); err != nil {
		return nil, err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	tmp := path() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return nil, err
	}
	return c, os.Rename(tmp, path())
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

// MCPAlive reports whether an AI host talked to `foundry mcp` recently.
func (c *Config) MCPAlive() bool {
	return c.MCP != nil && time.Since(c.MCP.At) < 10*time.Minute
}
