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
	RequestTimeout   int
}

// NewConfig returns a new Config struct.
func NewConfig() *Config {
	return &Config{
		DatabasePath:     GetEnvValue("DATABASE_PATH", "./data/oaamonitor.db"),
		DownloadDatabase: GetEnvValue("DOWNLOAD_DATABASE", false),
		RefreshRate:      GetEnvValue("REFRESH_RATE", 3600),
		UploadDatabase:   GetEnvValue("UPLOAD_DATABASE", false),
		RequestTimeout:   GetEnvValue("REQUEST_TIMEOUT", 30),
	}
}

// GetEnvValue is a generic function that takes an environment variable name and a default value,
// and returns the value of the environment variable if it is present, or the default value otherwise.
func GetEnvValue[T string | int | bool](envVarName string, defaultValue T) T {
	valueStr := os.Getenv(envVarName)
	if valueStr == "" {
		return defaultValue
	}

	var result T
	var err error

	switch any(defaultValue).(type) {
	case string:
		result = any(valueStr).(T)
	case int:
		var intVal int
		intVal, err = strconv.Atoi(valueStr)
		result = any(intVal).(T)
	case bool:
		var boolVal bool
		boolVal, err = strconv.ParseBool(valueStr)
		result = any(boolVal).(T)
	}

	if err != nil {
		log.Printf("problem parsing env var %s: %v\n", envVarName, err)
		return defaultValue
	}
	return result
}
