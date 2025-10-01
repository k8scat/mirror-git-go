package gitlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/k8scat/mirror-git-go/pkg/types"
)

var _ types.TargetGit = &GitLab{}

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

// ProtectedBranch represents a protected branch in GitLab
type ProtectedBranch struct {
	ID                        int           `json:"id"`
	Name                      string        `json:"name"`
	PushAccessLevels          []AccessLevel `json:"push_access_levels"`
	MergeAccessLevels         []AccessLevel `json:"merge_access_levels"`
	UnprotectAccessLevels     []AccessLevel `json:"unprotect_access_levels"`
	CodeOwnerApprovalRequired bool          `json:"code_owner_approval_required"`
}

// AccessLevel represents access level information
type AccessLevel struct {
	ID          int    `json:"id"`
	AccessLevel int    `json:"access_level"`
	Description string `json:"access_level_description"`
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
		project := g.Username + "/" + repoName
		branches, err := g.ListProtectedBranches(project)
		if err != nil {
			slog.Error("list protected branches failed", "error", err, "repo", repoName)
		} else {
			for _, branch := range branches {
				slog.Info("unprotected branch", "repo", repoName, "branch", branch.Name)
				if err := g.UnprotectBranch(project, branch.Name); err != nil {
					slog.Error("unprotect branch failed", "error", err, "repo", repoName, "branch", branch.Name)
				}
			}
		}

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

// ListProtectedBranches lists all protected branches for a project
// https://docs.gitlab.com/ee/api/protected_branches.html#list-protected-branches
func (g *GitLab) ListProtectedBranches(projectID string) ([]ProtectedBranch, error) {
	// Use URL encoding for the project ID
	encodedProjectID := url.QueryEscape(projectID)
	apiURL := fmt.Sprintf("%s/projects/%s/protected_branches", g.BaseAPI, encodedProjectID)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Private-Token", g.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("list protected branches failed: %s", string(body))
		return nil, fmt.Errorf("list protected branches failed, status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var branches []ProtectedBranch
	if err := json.Unmarshal(body, &branches); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return branches, nil
}

// UnprotectBranch unprotects the given protected branch or wildcard protected branch
// https://docs.gitlab.com/ee/api/protected_branches.html#unprotect-repository-branches
func (g *GitLab) UnprotectBranch(projectID, branchName string) error {
	// Use URL encoding for both project ID and branch name
	encodedProjectID := url.QueryEscape(projectID)
	encodedBranchName := url.QueryEscape(branchName)
	apiURL := fmt.Sprintf("%s/projects/%s/protected_branches/%s", g.BaseAPI, encodedProjectID, encodedBranchName)

	req, err := http.NewRequest(http.MethodDelete, apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Private-Token", g.AccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	return fmt.Errorf("unprotect repository branch failed, status code: %d, body: %s", resp.StatusCode, string(body))
}
