package config

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	configFileName string = ".urv.yaml"
	configFileType string = "yaml"
)

type Config struct {
	SecretFiles []string `mapstructure:"secretfiles"`
	Patterns    []string `mapstructure:"patterns"`
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
