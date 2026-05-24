package config

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	SecretFiles []string `mapstructure:"secretfiles"`
}

func Load(repoPath string) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(filepath.Join(repoPath, ".urv"))
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config to struct: %w", err)
	}

	return &cfg, nil
}
