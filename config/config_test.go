package config

import (
	"os"
	"testing"
)

func TestGetEnvStringOrDefault(t *testing.T) {
	// Test case 1: Environment variable is not set, should return the default value
	defaultValue := "./data/oaamonitor.db"
	result := GetEnvStringOrDefault("DATABASE_PATH", defaultValue)
	if result != defaultValue {
		t.Errorf("Expected %s, but got %s", defaultValue, result)
	}

	// Test case 2: Environment variable is set to a non-empty value
	os.Setenv("DATABASE_PATH", "/path/to/database.db")
	expectedValue := "/path/to/database.db"
	result = GetEnvStringOrDefault("DATABASE_PATH", defaultValue)
	if result != expectedValue {
		t.Errorf("Expected %s, but got %s", expectedValue, result)
	}

	// Test case 3: Environment variable is set to an empty value, should return the default value
	os.Setenv("DATABASE_PATH", "")
	result = GetEnvStringOrDefault("DATABASE_PATH", defaultValue)
	if result != defaultValue {
		t.Errorf("Expected %s, but got %s", defaultValue, result)
	}
}

func TestGetEnvIntOrDefault(t *testing.T) {
	// Test case 1: Environment variable is not set, should return the default value
	defaultValue := 3600
	result := GetEnvIntOrDefault("REFRESH_RATE", defaultValue)
	if result != defaultValue {
		t.Errorf("Expected %d, but got %d", defaultValue, result)
	}

	// Test case 2: Environment variable is set to a valid integer value
	os.Setenv("REFRESH_RATE", "1800")
	expectedValue := 1800
	result = GetEnvIntOrDefault("REFRESH_RATE", defaultValue)
	if result != expectedValue {
		t.Errorf("Expected %d, but got %d", expectedValue, result)
	}

	// Test case 3: Environment variable is set to an invalid integer value, should return the default value
	os.Setenv("REFRESH_RATE", "invalid")
	result = GetEnvIntOrDefault("REFRESH_RATE", defaultValue)
	if result != defaultValue {
		t.Errorf("Expected %d, but got %d", defaultValue, result)
	}
}
func TestGetEnvBoolOrDefault(t *testing.T) {
	// Test case 1: Environment variable is not set, should return the default value
	defaultValue := false
	result := GetEnvBoolOrDefault("DOWNLOAD_DATABASE", defaultValue)
	if result != defaultValue {
		t.Errorf("Expected %t, but got %t", defaultValue, result)
	}

	// Test case 2: Environment variable is set to a valid boolean value
	os.Setenv("DOWNLOAD_DATABASE", "true")
	expectedValue := true
	result = GetEnvBoolOrDefault("DOWNLOAD_DATABASE", defaultValue)
	if result != expectedValue {
		t.Errorf("Expected %t, but got %t", expectedValue, result)
	}

	// Test case 3: Environment variable is set to an invalid boolean value, should return the default value
	os.Setenv("DOWNLOAD_DATABASE", "invalid")
	result = GetEnvBoolOrDefault("DOWNLOAD_DATABASE", defaultValue)
	if result != defaultValue {
		t.Errorf("Expected %t, but got %t", defaultValue, result)
	}
}
