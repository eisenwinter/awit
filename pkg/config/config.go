package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const FileName = "config.yaml"

type Config struct {
	Prefix        string   `yaml:"prefix"`
	DefaultLabels []string `yaml:"default_labels,omitempty"`
	StaleClaim    Duration `yaml:"stale_claim"` // default 2h
	AgentID       string   `yaml:"agent_id,omitempty"`
}

type Duration time.Duration

func (d Duration) MarshalYAML() (any, error) {
	s := time.Duration(d).String()
	s = strings.TrimSuffix(s, "0s")
	s = strings.TrimSuffix(s, "0m")
	if s == "" {
		s = "0s"
	}
	return s, nil
}

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

func Default(prefix string) Config {
	return Config{
		Prefix:     prefix,
		StaleClaim: Duration(2 * time.Hour),
	}
}

func Load(awitDir string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(awitDir, FileName))
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	if c.Prefix == "" {
		return Config{}, errors.New("config: prefix is required")
	}
	if c.StaleClaim == 0 {
		c.StaleClaim = Duration(2 * time.Hour)
	}
	return c, nil
}

func (c Config) Write(awitDir string) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return WriteAtomic(filepath.Join(awitDir, FileName), buf.Bytes())
}

func (c Config) Agent(flag string) string {
	if flag != "" {
		return flag
	}
	if v := os.Getenv("AWIT_AGENT"); v != "" {
		return v
	}
	return c.AgentID
}
