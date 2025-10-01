package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
)

type GitHub struct {
	AccessToken string
	Username    string
	BaseAPI     string
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
func NewGitHub(username, accessToken string) *GitHub {
	return &GitHub{
		Username:    username,
		AccessToken: accessToken,
		BaseAPI:     "https://api.github.com/graphql",
	}
}

func NewGitHubFromEnv() *GitHub {
	return &GitHub{
		Username:    os.Getenv("GITHUB_USERNAME"),
		AccessToken: os.Getenv("GITHUB_ACCESS_TOKEN"),
		BaseAPI:     "https://api.github.com/graphql",
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

	mutation := `
		mutation ($name: String!, $desc: String!, $isPrivate: RepositoryVisibility!) {
			createRepository(input: {name: $name, description: $desc, visibility: $isPrivate}) {
				clientMutationId
				repository {
					id
				}
			}
		}
	`

	visibility := "PUBLIC"
	if private {
		visibility = "PRIVATE"
	}

	variables := map[string]any{
		"name":      name,
		"desc":      desc,
		"isPrivate": visibility,
	}

	var response CreateRepoMutationResponse
	err = g.graphql(mutation, variables, &response)
	if err != nil {
		return fmt.Errorf("failed to execute GraphQL mutation: %w", err)
	}

	if len(response.Errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", response.Errors)
	}

	return nil
}

func (g *GitHub) GetRepoAddr(repoName string) string {
	return fmt.Sprintf("https://%s:%s@github.com/%s/%s.git", g.Username, g.AccessToken, g.Username, repoName)
}
