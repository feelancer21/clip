package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/feelancer21/clip"
	"github.com/go-playground/validator/v10"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

var (
	appName = "clip"
)

// Config is loaded from the YAML file (global).
type Config struct {
	KeyStorePath string               `yaml:"key_store_path"`
	Lnclient     string               `yaml:"lnclient" validate:"required,oneof=lnd interactive"`
	LNDConfig    *LNDConfig           `yaml:"lnd" validate:"required_if=Lnclient lnd"`
	LnInter      *LnInteractiveConfig `yaml:"interactive" validate:"required_if=Lnclient interactive"`
	LogLevel     string               `yaml:"log_level"`
	RelayURLs    []string             `yaml:"relay_urls" validate:"required,min=1,dive,url"`
	NodeInfo     clip.NodeInfo        `yaml:"node_info"`
}

// LNDConfig holds the LND node connection settings
type LNDConfig struct {
	Host         string `yaml:"host" validate:"required"`
	Port         int    `yaml:"port" validate:"required"`
	TLSCertPath  string `yaml:"tls_cert_path" validate:"required"`
	MacaroonPath string `yaml:"macaroon_path" validate:"required"`
}

type LnInteractiveConfig struct {
	Network string `yaml:"network" validate:"required"`
	PubKey  string `yaml:"pub_key" validate:"required"`
}

// Validate performs basic validation of the config values and sets defaults.
func (c *Config) validate() error {
	// Validate all fields with required tag
	validator := validator.New()
	if err := validator.Struct(c); err != nil {
		return err
	}

	if err := c.NodeInfo.Validate(); err != nil {
		return fmt.Errorf("validating node info: %w", err)
	}

	if c.LnInter != nil {
		if !clip.IsValidNetwork(c.LnInter.Network) {
			return fmt.Errorf("invalid interactive network: %s", c.LnInter.Network)
		}
	}

	return nil
}

func (c *Config) setDefaults() error {
	if c.KeyStorePath == "" {
		path, err := defaultKeyPath()
		if err != nil {
			return err
		}
		c.KeyStorePath = path
	}

	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	return nil
}

func loadConfig(c *cli.Context) (*Config, error) {
	var (
		configFile string
		err        error
	)

	if c.IsSet("config") {
		configFile = c.String("config")
	} else if configFile, err = defaultConfigPath(); err != nil {
		return nil, err
	}

	b, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	// new YAML decoder that errors on unknown fields,
	decoder := yaml.NewDecoder(bytes.NewReader(b))
	decoder.KnownFields(true)

	cfg := &Config{}
	if err = decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config file: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if err := cfg.setDefaults(); err != nil {
		return nil, fmt.Errorf("setting config defaults: %w", err)
	}

	return cfg, nil
}

// DefaultConfigPath returns a reasonable per-user path like
//
//	Linux/macOS: $XDG_CONFIG_HOME/.<app>/config.yaml
func defaultConfigPath() (string, error) {
	return configDirFilePath("config.yaml")
}

func configDirFilePath(filename string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appName, filename), nil
}
