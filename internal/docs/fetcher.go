package docs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v60/github"
	"golang.org/x/oauth2"
)

// ParseGitHubURL parses a standard GitHub URL into (owner, repo, subpath, branch, error).
func ParseGitHubURL(rawURL string) (owner, repo, subpath, branch string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", "", err
	}
	if !strings.Contains(u.Host, "github.com") {
		return "", "", "", "", fmt.Errorf("host %s is not github.com", u.Host)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", "", "", fmt.Errorf("invalid github URL path: %s", u.Path)
	}

	owner = parts[0]
	repo = parts[1]

	if len(parts) > 3 && parts[2] == "tree" {
		branch = parts[3]
		subpath = strings.Join(parts[4:], "/")
	}

	return owner, repo, subpath, branch, nil
}

// FetchReadme downloads the README content (from subpath or root) and saves it to outputPath.
func FetchReadme(ctx context.Context, rawURL, outputPath string) error {
	owner, repo, subpath, branch, err := ParseGitHubURL(rawURL)
	if err != nil {
		return err
	}

	var tc *github.Client
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc = github.NewClient(oauth2.NewClient(ctx, ts))
	} else {
		tc = github.NewClient(nil)
	}

	opts := &github.RepositoryContentGetOptions{}
	if branch != "" {
		opts.Ref = branch
	}

	// Ensure the parent directory of outputPath exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	// 1. If there is a subpath, try to get README.md or README inside that subpath
	if subpath != "" {
		readmePath := filepath.Join(subpath, "README.md")
		fileContent, _, _, err := tc.Repositories.GetContents(ctx, owner, repo, readmePath, opts)
		if err == nil && fileContent != nil {
			content, err := fileContent.GetContent()
			if err == nil {
				return os.WriteFile(outputPath, []byte(content), 0644)
			}
		}

		readmePathNoExt := filepath.Join(subpath, "README")
		fileContent, _, _, err = tc.Repositories.GetContents(ctx, owner, repo, readmePathNoExt, opts)
		if err == nil && fileContent != nil {
			content, err := fileContent.GetContent()
			if err == nil {
				return os.WriteFile(outputPath, []byte(content), 0644)
			}
		}
	}

	// 2. Fallback to the root README of the repository
	readme, _, err := tc.Repositories.GetReadme(ctx, owner, repo, opts)
	if err != nil {
		return fmt.Errorf("failed to fetch README from %s/%s: %w", owner, repo, err)
	}

	content, err := readme.GetContent()
	if err != nil {
		return err
	}

	return os.WriteFile(outputPath, []byte(content), 0644)
}

// FetchNPMReadme fetches the README of an NPM package and saves it to outputPath.
func FetchNPMReadme(ctx context.Context, packageName, outputPath string) error {
	reqURL := fmt.Sprintf("https://registry.npmjs.org/%s/latest", packageName)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("NPM registry returned status %s", resp.Status)
	}

	var data struct {
		Readme string `json:"readme"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	if data.Readme == "" {
		return fmt.Errorf("no README found in NPM registry for %s", packageName)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(outputPath, []byte(data.Readme), 0644)
}
