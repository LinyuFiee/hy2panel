package singbox

import (
	"strings"
	"testing"
)

func TestParseVersionOutput(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "standard",
			output: "sing-box version 1.14.4\n\nEnvironment: go1.26.0 linux/amd64\nTags: with_quic\nCGO: disabled\n",
			want:   "1.14.4",
		},
		{
			name:   "leading v",
			output: "sing-box version v1.14.3\n",
			want:   "1.14.3",
		},
		{
			name:   "windows line ending",
			output: "sing-box version 1.14.2\r\n\r\nEnvironment: go1.26.0 windows/amd64\r\n",
			want:   "1.14.2",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseVersionOutput(test.output)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseVersionOutputRejectsInvalidOutput(t *testing.T) {
	tests := map[string]string{
		"empty":              "",
		"missing line":       "Environment: go1.26 linux/amd64\n",
		"wrong product":      "other version 1.14.4\n",
		"unknown":            "sing-box version unknown\n",
		"missing patch":      "sing-box version 1.14\n",
		"prerelease":         "sing-box version 1.14.4-rc.1\n",
		"build metadata":     "sing-box version 1.14.4+build\n",
		"extra token":        "sing-box version 1.14.4 malicious\n",
		"duplicate":          "sing-box version 1.14.4\nsing-box version 1.14.3\n",
		"prefixed line":      "INFO sing-box version 1.14.4\n",
		"leading whitespace": " sing-box version 1.14.4\n",
		"NUL":                "sing-box version 1.14.4\x00\n",
		"too large":          "sing-box version 1.14.4\n" + strings.Repeat("x", 64<<10),
	}
	for name, output := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseVersionOutput(output); err == nil {
				t.Fatalf("expected output %q to fail", output)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	valid := map[string]string{
		"0.0.0":          "0.0.0",
		"1.14.4":         "1.14.4",
		"v1.14.4":        "1.14.4",
		"1000000000.2.3": "1000000000.2.3",
	}
	for input, expected := range valid {
		got, err := NormalizeVersion(input)
		if err != nil {
			t.Fatalf("NormalizeVersion(%q): %v", input, err)
		}
		if got != expected {
			t.Fatalf("NormalizeVersion(%q) = %q, want %q", input, got, expected)
		}
	}

	invalid := []string{
		"", "v", "1", "1.2", "1.2.3.4", "1.2.x", "V1.2.3",
		"01.2.3", "1.02.3", "1.2.03", "+1.2.3", "-1.2.3",
		"1.2.3-rc.1", "1.2.3+build", " 1.2.3", "1.2.3 ",
		"1000000001.2.3", "999999999999999999999.2.3",
	}
	for _, input := range invalid {
		if _, err := NormalizeVersion(input); err == nil {
			t.Fatalf("NormalizeVersion(%q) unexpectedly succeeded", input)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left  string
		right string
		want  int
	}{
		{left: "1.14.4", right: "1.14.3", want: 1},
		{left: "v1.14.4", right: "1.14.4", want: 0},
		{left: "1.14.2", right: "1.14.3", want: -1},
		{left: "2.0.0", right: "1.999.999", want: 1},
		{left: "1.10.0", right: "1.9.99", want: 1},
		{left: "0.0.0", right: "0.0.0", want: 0},
	}
	for _, test := range tests {
		got, err := CompareVersions(test.left, test.right)
		if err != nil {
			t.Fatalf("CompareVersions(%q, %q): %v", test.left, test.right, err)
		}
		if got != test.want {
			t.Fatalf("CompareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestCompareVersionsRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct{ left, right string }{
		{left: "garbage", right: "1.2.3"},
		{left: "1.2.3", right: "garbage"},
		{left: "1.2", right: "1.2.0"},
		{left: "1.2.3", right: "1.2.3-rc.1"},
	} {
		if _, err := CompareVersions(test.left, test.right); err == nil {
			t.Fatalf("CompareVersions(%q, %q) unexpectedly succeeded", test.left, test.right)
		}
	}
}
