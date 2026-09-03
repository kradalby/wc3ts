package config_test

import (
	"slices"
	"testing"

	"github.com/kradalby/wc3ts/config"
)

func TestParseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    uint32
		wantErr bool
	}{
		{"dotted 1.20", "1.20", 20, false},
		{"bare 20", "20", 20, false},
		{"dotted 1.26", "1.26", 26, false},
		{"dotted 1.29", "1.29", 29, false},
		{"whitespace", "  1.28 ", 28, false},
		{"empty", "", 0, true},
		{"garbage", "abc", 0, true},
		{"dotted garbage", "1.x", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := config.ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}

			if got != tt.want {
				t.Errorf("ParseVersion(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSupportedVersions(t *testing.T) {
	t.Parallel()

	versions := config.SupportedVersions()

	if len(versions) == 0 {
		t.Fatal("SupportedVersions() is empty")
	}

	// Sorted, contiguous, no duplicates.
	for i := 1; i < len(versions); i++ {
		if versions[i] != versions[i-1]+1 {
			t.Errorf("versions not contiguous/sorted at index %d: %v", i, versions)
		}
	}

	if got := versions[0]; got != 26 {
		t.Errorf("lowest supported version = %d, want 26", got)
	}

	if got := versions[len(versions)-1]; got != 28 {
		t.Errorf("highest supported version = %d, want 28", got)
	}

	if !slices.Contains(versions, config.DefaultGameVersion) {
		t.Errorf("DefaultGameVersion %d not in SupportedVersions %v", config.DefaultGameVersion, versions)
	}
}

func TestFormatVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   uint32
		want string
	}{
		{20, "1.20"},
		{26, "1.26"},
		{29, "1.29"},
	}

	for _, tt := range tests {
		if got := config.FormatVersion(tt.in); got != tt.want {
			t.Errorf("FormatVersion(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
