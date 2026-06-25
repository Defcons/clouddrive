package handlers

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPathWithinHome(t *testing.T) {
	tests := []struct {
		name string
		path string
		home string
		want bool
	}{
		{"empty home is unrestricted", "/anything/here", "", true},
		{"root home is unrestricted", "/anything/here", "/", true},
		{"exact home match", "/Alice", "/Alice", true},
		{"descendant of home", "/Alice/docs/file.txt", "/Alice", true},
		{"deep descendant", "/Alice/a/b/c", "/Alice", true},
		{"sibling outside home", "/Bob/secret.txt", "/Alice", false},
		{"sibling-prefix attack is denied", "/Alicebackup/file", "/Alice", false},
		{"parent of home is denied", "/", "/Alice", false},
		{"traversal escaping home is denied", "/Alice/../Bob", "/Alice", false},
		{"home with trailing slash normalizes", "/Alice/file", "/Alice/", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathWithinHome(tc.path, tc.home); got != tc.want {
				t.Errorf("pathWithinHome(%q, %q) = %v, want %v", tc.path, tc.home, got, tc.want)
			}
		})
	}
}

func TestSafePath(t *testing.T) {
	root := t.TempDir()
	h := &FileHandler{root: root}

	allowed := []struct {
		name string
		req  string
	}{
		{"empty path resolves to root", ""},
		{"root", "/"},
		{"normal subpath", "/docs/file.txt"},
		{"redundant separators cleaned", "/docs//sub/./file.txt"},
		{"contained dotdot stays inside", "/docs/../docs/file.txt"},
	}
	for _, tc := range allowed {
		t.Run("allow/"+tc.name, func(t *testing.T) {
			abs, err := h.safePath(tc.req)
			if err != nil {
				t.Fatalf("safePath(%q) unexpected error: %v", tc.req, err)
			}
			rootAbs, _ := filepath.Abs(root)
			if abs != rootAbs && !strings.HasPrefix(abs, rootAbs+string(filepath.Separator)) {
				t.Errorf("safePath(%q) = %q escaped root %q", tc.req, abs, rootAbs)
			}
		})
	}

	// Absolute "/.." inputs are harmless: filepath.Clean drops ".." above the
	// absolute root, so they stay inside. The real escape vector is a *relative*
	// path that climbs out of the join base — those must be rejected.
	denied := []struct {
		name string
		req  string
	}{
		{"relative parent escape", "../escape"},
		{"relative deep escape", "../../escape"},
		{"relative escape with suffix", "../../etc/passwd"},
	}
	for _, tc := range denied {
		t.Run("deny/"+tc.name, func(t *testing.T) {
			if _, err := h.safePath(tc.req); err == nil {
				t.Errorf("safePath(%q) = nil error, want traversal denied", tc.req)
			}
		})
	}
}
