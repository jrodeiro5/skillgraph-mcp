package version

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// LatestReleaseURL is the GitHub Releases API endpoint queried by
// FetchLatestTag. Exposed (not const) so tests can point it at httptest.
var LatestReleaseURL = "https://api.github.com/repos/jrodeiro5/skillgraph-mcp/releases/latest"

// FetchLatestTag returns the latest release tag (without any leading "v") from
// the GitHub Releases API. Callers should treat the version check as
// best-effort — any network/parsing failure returns ("", error) so the caller
// can degrade gracefully rather than block.
func FetchLatestTag(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, LatestReleaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github API status %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.TagName == "" {
		return "", errors.New("github API returned empty tag_name")
	}
	return strings.TrimPrefix(body.TagName, "v"), nil
}

// Compare returns -1 if a < b, 0 if equal, +1 if a > b. Inputs must be
// dot-separated numeric versions (e.g. "1.0.0"); leading "v" is tolerated.
// Pre-release suffixes like "-rc1" are not interpreted — segments containing
// non-numeric characters cause an error.
func Compare(a, b string) (int, error) {
	pa, err := parseSegments(strings.TrimPrefix(a, "v"))
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", a, err)
	}
	pb, err := parseSegments(strings.TrimPrefix(b, "v"))
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", b, err)
	}
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var ai, bi int
		if i < len(pa) {
			ai = pa[i]
		}
		if i < len(pb) {
			bi = pb[i]
		}
		if ai < bi {
			return -1, nil
		}
		if ai > bi {
			return 1, nil
		}
	}
	return 0, nil
}

func parseSegments(s string) ([]int, error) {
	if s == "" {
		return nil, errors.New("empty version")
	}
	parts := strings.Split(s, ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("segment %q: %w", p, err)
		}
		out[i] = n
	}
	return out, nil
}
