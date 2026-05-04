package models

import "testing"

func TestNormalizeRef(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"abcdef", "abcdef", false},
		{"GHST-abcdef", "abcdef", false},
		{"ghst-abcdef", "abcdef", false},
		{"  GHST-deadbeef  ", "deadbeef", false},
		{"f47ac10b-58cc-4372-a567-0e02b2c3d479", "f47ac10b-58cc-4372-a567-0e02b2c3d479", false},
		{"", "", true},
		{"   ", "", true},
	}

	for _, tt := range tests {
		got, err := NormalizeRef(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("NormalizeRef(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("NormalizeRef(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRefIDFor(t *testing.T) {
	tests := []struct {
		uuid string
		want string
	}{
		{"f47ac10b-58cc-4372-a567-0e02b2c3d479", "GHST-f47ac10b"},
		{"abcd1234", "GHST-abcd1234"},
		{"abc", "GHST-abc"},
	}
	for _, tt := range tests {
		if got := RefIDFor(tt.uuid); got != tt.want {
			t.Errorf("RefIDFor(%q) = %q, want %q", tt.uuid, got, tt.want)
		}
	}
}
