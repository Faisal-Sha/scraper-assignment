package config

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Load initializes configuration from environment variables and .env file.
func Load() error {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	// Set default values
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_NAME", "scraper")

	if err := viper.ReadInConfig(); err != nil {
		logrus.WithError(err).Warn("Failed to read .env file, using environment variables")
	}

	logrus.Info("Configuration loaded successfully")
	return nil
}