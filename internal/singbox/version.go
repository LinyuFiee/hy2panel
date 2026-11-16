package singbox

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const maxVersionComponent = 1_000_000_000

var (
	versionPattern       = regexp.MustCompile(`^(?:v)?([0-9]+)\.([0-9]+)\.([0-9]+)$`)
	versionOutputPattern = regexp.MustCompile(`(?m)^sing-box version[ \t]+([^\r\n]+)[ \t]*\r?$`)
)

// ParseVersionOutput extracts and normalizes the version from the exact
// "sing-box version X.Y.Z" line produced by `sing-box version`. Additional
// environment lines are allowed; duplicate version lines are rejected.
func ParseVersionOutput(output string) (string, error) {
	if strings.IndexByte(output, 0) >= 0 {
		return "", errors.New("sing-box version output contains a NUL byte")
	}
	if len(output) > 64<<10 {
		return "", errors.New("sing-box version output is too large")
	}

	matches := versionOutputPattern.FindAllStringSubmatch(output, -1)
	if len(matches) != 1 {
		return "", fmt.Errorf("sing-box version output must contain exactly one version line, found %d", len(matches))
	}
	version, err := NormalizeVersion(strings.TrimSpace(matches[0][1]))
	if err != nil {
		return "", fmt.Errorf("invalid sing-box version output: %w", err)
	}
	return version, nil
}

// NormalizeVersion validates a strict three-component semantic version and
// returns it without an optional leading v. Pre-release/build metadata, signs,
// whitespace, missing components, and leading zeroes are rejected.
func NormalizeVersion(value string) (string, error) {
	matches := versionPattern.FindStringSubmatch(value)
	if len(matches) != 4 {
		return "", fmt.Errorf("version %q must be X.Y.Z with an optional leading v", value)
	}

	parts := [3]uint64{}
	for index := range parts {
		part := matches[index+1]
		if len(part) > 1 && part[0] == '0' {
			return "", fmt.Errorf("version %q has a leading zero", value)
		}
		parsed, err := strconv.ParseUint(part, 10, 32)
		if err != nil || parsed > maxVersionComponent {
			return "", fmt.Errorf("version %q has an out-of-range component", value)
		}
		parts[index] = parsed
	}
	return fmt.Sprintf("%d.%d.%d", parts[0], parts[1], parts[2]), nil
}

// CompareVersions securely compares two strict semantic versions. It returns
// -1, 0, or 1. Invalid input is never treated as version zero.
func CompareVersions(left, right string) (int, error) {
	leftParts, err := parseVersionParts(left)
	if err != nil {
		return 0, fmt.Errorf("invalid left version: %w", err)
	}
	rightParts, err := parseVersionParts(right)
	if err != nil {
		return 0, fmt.Errorf("invalid right version: %w", err)
	}
	for index := range leftParts {
		if leftParts[index] < rightParts[index] {
			return -1, nil
		}
		if leftParts[index] > rightParts[index] {
			return 1, nil
		}
	}
	return 0, nil
}

func parseVersionParts(value string) ([3]uint64, error) {
	var result [3]uint64
	canonical, err := NormalizeVersion(value)
	if err != nil {
		return result, err
	}
	for index, part := range strings.Split(canonical, ".") {
		result[index], err = strconv.ParseUint(part, 10, 64)
		if err != nil {
			return [3]uint64{}, err
		}
	}
	return result, nil
}
