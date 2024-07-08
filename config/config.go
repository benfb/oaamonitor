package config

import (
	"log"
	"os"
	"strconv"
)

// Config is a struct that holds the configuration for the application.
type Config struct {
	DatabasePath     string
	DownloadDatabase bool
	RefreshRate      int
	UploadDatabase   bool
}

// NewConfig returns a new Config struct.
func NewConfig() *Config {
	return &Config{
		DatabasePath:     GetEnvStringOrDefault("DATABASE_PATH", "./data/oaamonitor.db"),
		DownloadDatabase: GetEnvBoolOrDefault("DOWNLOAD_DATABASE", false),
		RefreshRate:      GetEnvIntOrDefault("REFRESH_RATE", 3600),
		UploadDatabase:   GetEnvBoolOrDefault("UPLOAD_DATABASE", false),
	}
}

// GetEnvStringOrDefault takes an environment variable name and a default value,
// and returns the value of the environment variable if it is present, or the default value otherwise.
func GetEnvStringOrDefault(envVarName, defaultValue string) string {
	value := os.Getenv(envVarName)
	if value == "" {
		return defaultValue
	}
	return value
}

// GetEnvIntOrDefault takes an environment variable name and a default value,
// and returns the value of the environment variable as an integer if it is present, or the default value otherwise.
func GetEnvIntOrDefault(envVarName string, defaultValue int) int {
	valueStr := os.Getenv(envVarName)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("problem parsing env var %s: %v\n", envVarName, err)
		return defaultValue
	}
	return value
}

// GetEnvBoolOrDefault takes an environment variable name and a default value,
// and returns the value of the environment variable as a boolean if it is present, or the default value otherwise.
func GetEnvBoolOrDefault(envVarName string, defaultValue bool) bool {
	valueStr := os.Getenv(envVarName)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		log.Printf("problem parsing env var %s: %v\n", envVarName, err)
		return defaultValue
	}
	return value
}
