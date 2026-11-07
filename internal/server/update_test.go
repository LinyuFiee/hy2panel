package server

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left, right string
		expected    int
	}{
		{"0.4.0", "0.3.1", 1},
		{"v0.4.0", "0.4.0", 0},
		{"0.3.1", "0.4.0", -1},
		{"1.0.0", "0.9.9", 1},
		{"0.4", "0.4.0", 0},
	}
	for _, test := range tests {
		if result := compareVersions(test.left, test.right); result != test.expected {
			t.Fatalf("compareVersions(%q, %q) = %d, expected %d", test.left, test.right, result, test.expected)
		}
	}
}
