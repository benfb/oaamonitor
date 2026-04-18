package main

import "testing"

func TestValidateOutputDir(t *testing.T) {
	tests := []struct {
		path    string
		wantErr bool
	}{
		{"public", false},
		{"out/site", false},
		{"", true},
		{".", true},
		{"/", true},
		{"/etc", true},
		{"/Users/ben/public", true},
		{"../escape", true},
	}
	for _, tt := range tests {
		err := validateOutputDir(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateOutputDir(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
		}
	}
}
