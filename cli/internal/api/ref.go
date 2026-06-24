package api

import (
	"fmt"
	"strings"
)

// ParseRef parses an "org_slug/frame_name" or "org_slug/frame_name@version"
// reference. version is empty when omitted.
func ParseRef(s string) (orgSlug, name, version string, err error) {
	ref := s
	if at := strings.LastIndex(ref, "@"); at >= 0 {
		version = ref[at+1:]
		ref = ref[:at]
	}
	slash := strings.IndexByte(ref, '/')
	if slash <= 0 || slash == len(ref)-1 {
		return "", "", "", fmt.Errorf("invalid frame reference %q: expected org_slug/frame_name[@version]", s)
	}
	orgSlug = ref[:slash]
	name = ref[slash+1:]
	if strings.ContainsAny(name, "/") {
		return "", "", "", fmt.Errorf("invalid frame reference %q: expected org_slug/frame_name[@version]", s)
	}
	return orgSlug, name, version, nil
}
