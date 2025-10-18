package refresher

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/benfb/oaamonitor/config"
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

func TestRunPeriodically(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := &config.Config{}

	// Set the interval for the ticker
	interval := 100 * time.Millisecond

	// Create a channel to receive the number of times the function is called
	calls := make(chan int)

	// Define the function to be called by RunPeriodically
	fn := func(ctx context.Context, cfg *config.Config) error {
		// Increment the counter
		calls <- 1
		return nil
	}

	// Start the RunPeriodically function in a separate goroutine
	go RunPeriodically(ctx, cfg, interval, fn)

	// Wait for the function to be called 3 times
	for range 3 {
		select {
		case <-calls:
			// Function called, continue
		case <-time.After(2 * interval):
			t.Errorf("RunPeriodically did not call the function within the expected time")
			return
		}
	}

	// Cancel the context to stop the RunPeriodically function
	cancel()

	// Wait for the RunPeriodically function to exit
	time.Sleep(2 * interval)

	// Check if the function was called more than 3 times
	select {
	case <-calls:
		t.Errorf("RunPeriodically called the function more times than expected")
	default:
		// Function called expected number of times, continue
	}
}
