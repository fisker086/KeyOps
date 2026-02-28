package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Harbor v2 API: list artifacts and collect tags
// GET /api/v2.0/projects/{project_name}/repositories/{repository_name}/artifacts

type harborArtifact struct {
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

func (s *Service) listTagsHarbor(ctx context.Context, m map[string]string, appName string) ([]string, error) {
	baseURL := strings.TrimSuffix(strings.TrimSpace(m["harbor_url"]), "/")
	project := strings.TrimSpace(m["harbor_project"])
	if project == "" {
		project = "library"
	}
	username := strings.TrimSpace(m["harbor_username"])
	password := strings.TrimSpace(m["harbor_password"])
	if baseURL == "" {
		return nil, fmt.Errorf("harbor_url is required in registry settings")
	}
	repoName := appName
	// Support app name as "project/repo" to override project
	if idx := strings.Index(appName, "/"); idx > 0 {
		project = appName[:idx]
		repoName = appName[idx+1:]
	}
	// Escape path segments
	projectEnc := url.PathEscape(project)
	repoEnc := url.PathEscape(repoName)
	path := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s/artifacts?page_size=100&with_tag=true", baseURL, projectEnc, repoEnc)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("harbor API returned %d", resp.StatusCode)
	}
	var artifacts []harborArtifact
	if err := json.NewDecoder(resp.Body).Decode(&artifacts); err != nil {
		return nil, err
	}
	var tags []string
	seen := make(map[string]bool)
	for _, a := range artifacts {
		for _, t := range a.Tags {
			if t.Name != "" && !seen[t.Name] {
				seen[t.Name] = true
				tags = append(tags, t.Name)
			}
		}
	}
	return tags, nil
}
