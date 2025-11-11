package refresher

import (
	"io"
	"strings"
	"testing"
)

func TestParsePercentage(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"50%", 0.5},
		{"25%", 0.25},
		{"100%", 1},
		{"0%", 0},
		{"-50%", -0.5},
		{"-25%", -0.25},
		{"-100%", -1},
		{"-0%", 0},
	}

	for _, test := range tests {
		result, err := parsePercentage(test.input)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if result != test.expected {
			t.Errorf("Unexpected result. Got: %f, want: %f", result, test.expected)
		}
	}
}
func TestRemoveBOM(t *testing.T) {
	input := strings.NewReader("\xEF\xBB\xBFHello, World!")
	expected := "Hello, World!"

	result := removeBOM(input)
	output, err := io.ReadAll(result)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if string(output) != expected {
		t.Errorf("Unexpected result. Got: %s, want: %s", string(output), expected)
	}
}
