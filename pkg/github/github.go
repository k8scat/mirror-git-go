package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/k8scat/mirror-git-go/pkg/types"
)

var _ types.TargetGit = &GitHub{}
var _ types.SourceGit = &GitHub{}

type GitHub struct {
	AccessToken string
	Username    string
	BaseAPI     string
	IsOrg       bool
}

// ListRepos implements types.SourceGit.
func (g *GitHub) ListRepos() ([]types.Repo, error) {
	client := &http.Client{
		Timeout: 60 * time.Second,
	}
	apiBaseURL := "https://api.github.com/user/repos"
	perPage := 100
	page := 1

	var repos []types.Repo

	for {
		queryValues := url.Values{}
		queryValues.Set("per_page", fmt.Sprintf("%d", perPage))
		queryValues.Set("page", fmt.Sprintf("%d", page))
		apiURL := apiBaseURL + "?" + queryValues.Encode()

		req, err := http.NewRequest(http.MethodGet, apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("Authorization", "Bearer "+g.AccessToken)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to call GitHub API: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("GitHub API error: %v", resp.Status)
		}

		var rawRepos []struct {
			Name        string `json:"name"`
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			Private     bool   `json:"private"`
		}
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&rawRepos); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("failed to decode GitHub repos: %w", err)
		}
		resp.Body.Close() // Close before next request

		for _, r := range rawRepos {
			repos = append(repos, types.NewRepo(
				r.Name,
				r.FullName,
				r.Description,
				r.Private,
			))
		}

		if len(rawRepos) < perPage {
			break
		}
		page++
	}

	return repos, nil
}

func (g *GitHub) Name() string {
	return "github"
}

// GraphQL request structure
type GraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// Repository query response
type RepositoryQueryResponse struct {
	Data struct {
		Repository *struct {
			ID string `json:"id"`
		} `json:"repository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// Create repository mutation response
type CreateRepoMutationResponse struct {
	Data struct {
		CreateRepository struct {
			ClientMutationID string `json:"clientMutationId"`
			Repository       struct {
				ID string `json:"id"`
			} `json:"repository"`
		} `json:"createRepository"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

// NewGitHub creates a new GitHub client
func NewGitHub(username, accessToken string, isOrg bool) *GitHub {
	return &GitHub{
		Username:    username,
		AccessToken: accessToken,
		BaseAPI:     "https://api.github.com/graphql",
		IsOrg:       isOrg,
	}
}

// NewGitHubFromEnv creates a new GitHub client from environment variables
func NewGitHubFromEnv() *GitHub {
	return &GitHub{
		Username:    os.Getenv("GITHUB_USERNAME"),
		AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
		BaseAPI:     "https://api.github.com/graphql",
		IsOrg:       os.Getenv("GITHUB_IS_ORG") == "true",
	}
}

func (g *GitHub) graphql(query string, variables map[string]any, response any) error {
	request := GraphQLRequest{
		Query:     query,
		Variables: variables,
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, g.BaseAPI, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+g.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

func (g *GitHub) IsRepoExist(name string) (bool, error) {
	query := `
		query ($repo_owner: String!, $repo_name: String!) {
			repository(owner: $repo_owner, name: $repo_name) {
				id
			}
		}
	`

	variables := map[string]any{
		"repo_owner": g.Username,
		"repo_name":  name,
	}

	var response RepositoryQueryResponse
	err := g.graphql(query, variables, &response)
	if err != nil {
		return false, fmt.Errorf("failed to execute GraphQL query: %w", err)
	}

	if len(response.Errors) > 0 {
		err := response.Errors[0]
		if strings.HasPrefix(err.Message, "Could not resolve to a Repository with the name") {
			return false, nil
		}

		return false, fmt.Errorf("GraphQL errors: %v", response.Errors)
	}

	return response.Data.Repository != nil && response.Data.Repository.ID != "", nil
}

func (g *GitHub) CreateRepo(name string, desc string, private bool) error {
	// Check if repository already exists
	exists, err := g.IsRepoExist(name)
	if err != nil {
		return fmt.Errorf("failed to check if repository exists: %w", err)
	}
	if exists {
		slog.Info("Repository already exists", "name", name)
		return nil
	}

	if g.IsOrg {
		return g.createOrgRepo(name, desc, private)
	}
	return g.createUserRepo(name, desc, private)
}

func (g *GitHub) createOrgRepo(name string, desc string, private bool) error {
	apiURL := fmt.Sprintf("https://api.github.com/orgs/%s/repos", g.Username)

	payload := map[string]any{
		"name":        name,
		"description": desc,
		"private":     private,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.AccessToken))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create org repo: %s", string(body))
	}
	return nil
}

func (g *GitHub) createUserRepo(name string, desc string, private bool) error {
	apiURL := "https://api.github.com/user/repos"

	payload := map[string]any{
		"name":        name,
		"description": desc,
		"private":     private,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.AccessToken))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create user repo: %s", string(body))
	}
	return nil
}

func (g *GitHub) GetTargetRepoAddr(path string) string {
	return fmt.Sprintf("https://%s:%s@github.com/%s/%s.git", g.Username, g.AccessToken, g.Username, path)
}

func (g *GitHub) GetSourceRepoAddr(pathWithNamespace string) string {
	return fmt.Sprintf("https://%s:%s@github.com/%s.git", g.Username, g.AccessToken, pathWithNamespace)
}
