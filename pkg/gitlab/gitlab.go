package gitlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

type GitLab struct {
	AccessToken string
	Username    string
	BaseAPI     string
}

// NewGitLab creates a new GitLab client
func NewGitLab(username, accessToken string) *GitLab {
	return &GitLab{
		Username:    username,
		AccessToken: accessToken,
		BaseAPI:     "https://gitlab.com/api/v4",
	}
}

func NewGitLabFromEnv() *GitLab {
	return &GitLab{
		Username:    os.Getenv("GITLAB_USERNAME"),
		AccessToken: os.Getenv("GITLAB_ACCESS_TOKEN"),
		BaseAPI:     "https://gitlab.com/api/v4",
	}
}

// CreateRepoRequest represents the request payload for creating a repository
type CreateRepoRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
}

// IsRepoExist checks if a repository exists
func (g *GitLab) IsRepoExist(repoName string) (bool, error) {
	// Get single project: GET /projects/:id
	// Use URL encoding for the project path
	path := url.QueryEscape(fmt.Sprintf("%s/%s", g.Username, repoName))
	apiURL := fmt.Sprintf("%s/projects/%s", g.BaseAPI, path)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Private-Token", g.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %w", err)
	}
	return false, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(b))
}

// CreateRepo creates a new repository
func (g *GitLab) CreateRepo(name, desc string, isPrivate bool) error {
	visibility := "public"
	if isPrivate {
		visibility = "private"
	}

	data := CreateRepoRequest{
		Name:        name,
		Description: desc,
		Visibility:  visibility,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal request data: %w", err)
	}

	apiURL := fmt.Sprintf("%s/projects", g.BaseAPI)
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Private-Token", g.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create repository, status code: %d", resp.StatusCode)
	}

	return nil
}

func (g *GitLab) GetRepoAddr(repoName string) string {
	return fmt.Sprintf("https://%s:%s@gitlab.com/%s/%s.git", g.Username, g.AccessToken, g.Username, repoName)
}
