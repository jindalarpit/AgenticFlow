package release

import (
	"fmt"
	"regexp"
	"strings"
)

// tagPattern matches valid semver tags: vX.Y.Z or vX.Y.Z-prerelease
var tagPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)

// ValidateTag validates a Git tag for release eligibility.
// It returns the version string (without the "v" prefix), whether the release
// is stable (no pre-release suffix), and an error if the tag is invalid.
//
// A tag is invalid if:
//   - It does not match the pattern vX.Y.Z or vX.Y.Z-suffix
//   - It contains the substring "dirty"
//
// A tag is considered unstable (isStable=false) if it contains a hyphen after
// the version numbers (i.e., has a pre-release suffix).
func ValidateTag(tag string) (version string, isStable bool, err error) {
	if !tagPattern.MatchString(tag) {
		return "", false, fmt.Errorf("release tags must match vX.Y.Z or vX.Y.Z-suffix; got %q", tag)
	}

	if strings.Contains(tag, "dirty") {
		return "", false, fmt.Errorf("refusing to release from dirty tag %q", tag)
	}

	// Strip the "v" prefix to get the version string.
	version = strings.TrimPrefix(tag, "v")

	// A tag with a hyphen after the version (pre-release suffix) is unstable.
	isStable = !strings.Contains(tag[1:], "-")

	return version, isStable, nil
}
