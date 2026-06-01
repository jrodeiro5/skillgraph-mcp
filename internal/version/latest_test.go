package version

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompare(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "v1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.1.0", "1.0.9", 1},
		{"2.0.0", "1.99.99", 1},
		{"1.0", "1.0.0", 0},
		{"1.0.0.5", "1.0.0", 1},
	}
	for _, tc := range tests {
		got, err := Compare(tc.a, tc.b)
		if err != nil {
			t.Errorf("Compare(%q,%q) err=%v", tc.a, tc.b, err)
			continue
		}
		if got != tc.want {
			t.Errorf("Compare(%q,%q)=%d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCompareInvalid(t *testing.T) {
	t.Parallel()
	cases := []string{"", "1.0-rc1", "abc", "1.x.0"}
	for _, s := range cases {
		if _, err := Compare(s, "1.0.0"); err == nil {
			t.Errorf("Compare(%q,\"1.0.0\") expected error", s)
		}
	}
}

func TestFetchLatestTagOK(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3","name":"v1.2.3"}`))
	}))
	defer srv.Close()

	orig := LatestReleaseURL
	LatestReleaseURL = srv.URL
	defer func() { LatestReleaseURL = orig }()

	got, err := FetchLatestTag(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.2.3" {
		t.Errorf("got %q, want 1.2.3", got)
	}
}

func TestFetchLatestTagNon200(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	defer srv.Close()

	orig := LatestReleaseURL
	LatestReleaseURL = srv.URL
	defer func() { LatestReleaseURL = orig }()

	_, err := FetchLatestTag(context.Background())
	if err == nil {
		t.Fatal("expected error on non-200")
	}
}

func TestFetchLatestTagEmpty(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	orig := LatestReleaseURL
	LatestReleaseURL = srv.URL
	defer func() { LatestReleaseURL = orig }()

	_, err := FetchLatestTag(context.Background())
	if err == nil {
		t.Fatal("expected error on empty tag_name")
	}
}
