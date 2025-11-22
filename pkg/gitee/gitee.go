package gitee

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/k8scat/mirror-git-go/pkg/types"
)

var _ types.TargetGit = &Gitee{}

type Gitee struct {
	AccessToken string
	Username    string
	BaseAPI     string
	client      *http.Client
	Version     string
}

func NewGiteeFromEnv() *Gitee {
	g := &Gitee{
		Username:    os.Getenv("GITEE_USERNAME"),
		AccessToken: os.Getenv("GITEE_ACCESS_TOKEN"),
		client:      &http.Client{Timeout: 60 * time.Second},
		Version:     "v5",
	}
	g.BaseAPI = "https://gitee.com/api/" + g.Version
	return g
}

type CreateRepoRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
	AccessToken string `json:"access_token"`
}

// CreateRepo implements types.TargetGit.
func (g *Gitee) CreateRepo(name string, desc string, private bool) error {
	payload := CreateRepoRequest{
		Name:        name,
		Description: desc,
		Private:     private,
		AccessToken: g.AccessToken,
	}

	url := g.BaseAPI + "/user/repos"
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}
	req, err := http.NewRequest("POST", url, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.AccessToken)

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create repo failed, status: %s, body: %s", resp.Status, string(respBody))
	}
	return nil
}

// GetTargetRepoAddr implements types.TargetGit.
func (g *Gitee) GetTargetRepoAddr(path string) string {
	return fmt.Sprintf("https://%s:%s@gitee.com/%s/%s.git", g.Username, g.AccessToken, g.Username, path)
}

// IsRepoExist implements types.TargetGit.
func (g *Gitee) IsRepoExist(repoName string) (bool, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", g.BaseAPI, g.Username, repoName)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+g.AccessToken)

	resp, err := g.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		return true, nil
	case 404:
		return false, nil
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("error checking repo: status=%d, body=%s", resp.StatusCode, string(respBody))
	}
}

// Name implements types.TargetGit.
func (g *Gitee) Name() string {
	return "gitee"
}
