package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Nexus Docker Registry v2 API: GET /v2/<name>/tags/list
// Returns {"name":"...","tags":["v1","v2"]}

type nexusTagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func (s *Service) listTagsNexus(ctx context.Context, m map[string]string, appName string) ([]string, error) {
	baseURL := strings.TrimSuffix(strings.TrimSpace(m["nexus_url"]), "/")
	repo := strings.TrimSpace(m["nexus_repository"]) // Docker repository name in Nexus (e.g. docker-hosted)
	username := strings.TrimSpace(m["nexus_username"])
	password := strings.TrimSpace(m["nexus_password"])
	if baseURL == "" {
		return nil, fmt.Errorf("nexus_url is required in registry settings")
	}
	// Image name: if nexus_repository is set, image might be repo/appName; else appName
	imageName := appName
	if repo != "" {
		imageName = strings.TrimPrefix(repo+"/"+appName, "/")
	}
	path := fmt.Sprintf("%s/v2/%s/tags/list", baseURL, url.PathEscape(imageName))
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
		return nil, fmt.Errorf("nexus API returned %d", resp.StatusCode)
	}
	var data nexusTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	return data.Tags, nil
}
