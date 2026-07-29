package version

import "testing"

func TestString(t *testing.T) {
	origVersion, origCommit := Version, Commit
	t.Cleanup(func() { Version, Commit = origVersion, origCommit })

	tests := []struct {
		name    string
		version string
		commit  string
		want    string
	}{
		{"without commit metadata", "1.2.3", "unknown", "tit 1.2.3"},
		{"with commit metadata", "1.2.3", "abc1234", "tit 1.2.3 (abc1234)"},
		{"development default", "dev", "unknown", "tit dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Version, Commit = tt.version, tt.commit
			if got := String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
