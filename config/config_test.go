package config

import (
	"os"
	"testing"
)

func TestGetEnvValue(t *testing.T) {
	tests := []struct {
		envVarName    string
		envVarValue   string
		defaultValue  interface{}
		expectedValue interface{}
	}{
		{"TEST_STRING", "test_value", "default_value", "test_value"},
		{"TEST_STRING", "", "default_value", "default_value"},
		{"TEST_INT", "123", 456, 123},
		{"TEST_INT", "", 456, 456},
		{"TEST_BOOL", "true", false, true},
		{"TEST_BOOL", "", true, true},
	}

	for _, tt := range tests {
		if tt.envVarValue != "" {
			os.Setenv(tt.envVarName, tt.envVarValue)
		} else {
			os.Unsetenv(tt.envVarName)
		}

		switch tt.defaultValue.(type) {
		case string:
			result := GetEnvValue(tt.envVarName, tt.defaultValue.(string))
			if result != tt.expectedValue {
				t.Errorf("GetEnvValue(%s, %v) = %v; want %v", tt.envVarName, tt.defaultValue, result, tt.expectedValue)
			}
		case int:
			result := GetEnvValue(tt.envVarName, tt.defaultValue.(int))
			if result != tt.expectedValue {
				t.Errorf("GetEnvValue(%s, %v) = %v; want %v", tt.envVarName, tt.defaultValue, result, tt.expectedValue)
			}
		case bool:
			result := GetEnvValue(tt.envVarName, tt.defaultValue.(bool))
			if result != tt.expectedValue {
				t.Errorf("GetEnvValue(%s, %v) = %v; want %v", tt.envVarName, tt.defaultValue, result, tt.expectedValue)
			}
		}
	}
}
