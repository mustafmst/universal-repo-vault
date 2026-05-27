package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

const (
	configFileName string = ".urv.yaml"
	configFileType string = "yaml"
)

var ErrConfigNotExist error = errors.New("config not exist")

type Config struct {
	SecretFiles []string `mapstructure:"secretfiles" yaml:"secretfiles"`
	Patterns    []string `mapstructure:"patterns" yaml:"patterns"`
	Archiver    string   `mapstructure:"archiver" yaml:"archiver"`
	Cypher      string   `mapstructure:"cypher" yaml:"cypher"`
}

type ConfigProvider struct {
	repoPath string
	config   *Config
}

func NewProvider(repoPath string) *ConfigProvider {
	return &ConfigProvider{repoPath, nil}
}

func (cp *ConfigProvider) GetConfigPath() string {
	return filepath.Join(cp.repoPath, configFileName)
}

func (cp *ConfigProvider) Get() (*Config, error) {
	if cp.config != nil {
		return cp.config, nil
	}

	rawData, err := os.ReadFile(cp.GetConfigPath())
	if err != nil && errors.Is(err, os.ErrNotExist) {
		cp.config = defaultConfig()
		return cp.config, fmt.Errorf("getting config for repo in %s: %w", cp.repoPath, err)
	}
	var c Config

	err = yaml.Unmarshal(rawData, c)
	if err != nil {
		cp.config = defaultConfig()
		return cp.config, fmt.Errorf("deserializing config for repo in %s: %w", cp.repoPath, err)
	}
	cp.config = &c
	return cp.config, nil
}

func defaultConfig() *Config {
	return &Config{
		SecretFiles: []string{".env"},
		Patterns:    []string{"*.secret.*"},
		Archiver:    "zip",
		Cypher:      "aes-gcm",
	}
}

// Load reads configuration from given path and return deserialized object
func Load(repoPath string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(filepath.Join(repoPath, configFileName))
	v.SetConfigType(configFileType)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config to struct: %w", err)
	}

	return &cfg, nil
}

// Initialize creates empty configuration file with example values inside repository
func Initialize(repoPath string) error {
	v := viper.New()

	fullConfigPath := filepath.Join(repoPath, configFileName)
	v.SetConfigFile(fullConfigPath)
	v.SetConfigType(configFileType)

	v.Set("secretfiles", []string{".env"})
	v.Set("patterns", []string{"*.secret.*"})

	if err := v.SafeWriteConfigAs(fullConfigPath); err != nil {
		if errors.Is(err, viper.ConfigFileAlreadyExistsError(fullConfigPath)) {
			return nil
		}
		return fmt.Errorf("initializing config file: %w", err)
	}
	return nil
}
