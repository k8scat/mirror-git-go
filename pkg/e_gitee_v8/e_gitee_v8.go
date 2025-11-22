package e_gitee_v8

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/k8scat/mirror-git-go/pkg/types"
	"github.com/tidwall/gjson"
)

var _ types.SourceGit = &EnterpriseGiteeV8{}

type EnterpriseGiteeV8 struct {
	EnterpriseId string
	Username     string
	AccessToken  string
}

func (g *EnterpriseGiteeV8) Name() string {
	return "e_gitee_v8"
}

func NewEnterpriseGiteeV8(enterpriseId, accessToken, username string) *EnterpriseGiteeV8 {
	return &EnterpriseGiteeV8{
		EnterpriseId: enterpriseId,
		Username:     username,
		AccessToken:  accessToken,
	}
}

func NewEnterpriseGiteeV8FromEnv() *EnterpriseGiteeV8 {
	return &EnterpriseGiteeV8{
		EnterpriseId: os.Getenv("E_GITEE_V8_ENTERPRISE_ID"),
		Username:     os.Getenv("E_GITEE_V8_USERNAME"),
		AccessToken:  os.Getenv("E_GITEE_V8_ACCESS_TOKEN"),
	}
}

type Namespace struct {
	ID           int    `json:"id"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	CompleteName string `json:"complete_name"`
	CompletePath string `json:"complete_path"`
	Description  string `json:"description"`
}

type Creator struct {
	ID                 int    `json:"id"`
	Username           string `json:"username"`
	Name               string `json:"name"`
	Remark             string `json:"remark"`
	Pinyin             string `json:"pinyin"`
	AvatarURL          string `json:"avatar_url"`
	IsAIBot            bool   `json:"is_ai_bot"`
	IsEnterpriseMember bool   `json:"is_enterprise_member"`
	IsHistoryMember    bool   `json:"is_history_member"`
	Outsourced         bool   `json:"outsourced"`
}

type Repo struct {
	ID                      int         `json:"id"`
	Name                    string      `json:"name"`
	Path                    string      `json:"path"`
	PathWithNamespace       string      `json:"path_with_namespace"`
	Public                  int         `json:"public"`
	EnterpriseID            int         `json:"enterprise_id"`
	CreatedAt               time.Time   `json:"created_at"`
	UpdatedAt               time.Time   `json:"updated_at"`
	SecurityHoleEnabled     bool        `json:"security_hole_enabled"`
	Namespace               Namespace   `json:"namespace"`
	Creator                 Creator     `json:"creator"`
	NameWithNamespace       string      `json:"name_with_namespace"`
	ScanCheckRun            bool        `json:"scan_check_run"`
	IsFork                  bool        `json:"is_fork"`
	ParentProject           interface{} `json:"parent_project"`
	Status                  int         `json:"status"`
	StatusName              string      `json:"status_name"`
	Outsourced              bool        `json:"outsourced"`
	RepoSize                int         `json:"repo_size"`
	CanAdminProject         bool        `json:"can_admin_project"`
	MembersCount            int         `json:"members_count"`
	LastPushAt              time.Time   `json:"last_push_at"`
	WatchesCount            int         `json:"watches_count"`
	StarsCount              int         `json:"stars_count"`
	ForkedCount             int         `json:"forked_count"`
	EnableBackup            bool        `json:"enable_backup"`
	HasBackups              bool        `json:"has_backups"`
	VIP                     bool        `json:"vip"`
	Recomm                  bool        `json:"recomm"`
	Template                interface{} `json:"template"`
	TemplateEnabled         bool        `json:"template_enabled"`
	Description             string      `json:"description"`
	GetDefaultBranch        string      `json:"get_default_branch"`
	ReleasesCount           int         `json:"releases_count"`
	TotalPRCount            int         `json:"total_pr_count"`
	OpenPRCount             int         `json:"open_pr_count"`
	IsStar                  bool        `json:"is_star"`
	UsedTemplateCount       int         `json:"used_template_count"`
	ForkEnabled             bool        `json:"fork_enabled"`
	PullRequestsEnabled     bool        `json:"pull_requests_enabled"`
	NameWithoutTopNamespace string      `json:"name_without_top_namespace"`
	WikiEnabledWithContent  bool        `json:"wiki_enabled_with_content"`
}

func (g *EnterpriseGiteeV8) listRepos(page, perPage int) ([]types.Repo, error) {
	api := "https://api.gitee.com/enterprises/" + g.EnterpriseId + "/projects"

	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36")

	queries := url.Values{}
	queries.Set("access_token", g.AccessToken)
	queries.Set("per_page", fmt.Sprintf("%d", perPage))
	queries.Set("page", fmt.Sprintf("%d", page))
	req.URL.RawQuery = queries.Encode()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("read response body failed", "error", err)
		return nil, err
	}

	data := gjson.Get(string(body), "data")
	if !data.Exists() {
		slog.Error("response data not exists", "body", string(body))
		return nil, fmt.Errorf("response data not exists")
	}

	repos := make([]*Repo, 0)
	err = json.Unmarshal([]byte(data.Raw), &repos)
	if err != nil {
		slog.Error("unmarshal failed", "error", err, "body", string(body))
		return nil, err
	}

	result := make([]types.Repo, len(repos))
	for i, r := range repos {
		result[i] = types.NewRepo(r.Path, r.PathWithNamespace, r.Description, true)
	}

	return result, nil
}

func (g *EnterpriseGiteeV8) ListRepos() ([]types.Repo, error) {
	allRepos := make([]types.Repo, 0)
	page := 1
	perPage := 100
	for {
		repos, err := g.listRepos(page, perPage)
		if err != nil {
			return nil, err
		}
		if len(repos) == 0 {
			break
		}
		allRepos = append(allRepos, repos...)
		if len(repos) < perPage {
			break
		}
		page++
	}

	return allRepos, nil
}

// GetSourceRepoAddr implements types.SourceGit.
func (g *EnterpriseGiteeV8) GetSourceRepoAddr(pathWithNamespace string) string {
	return fmt.Sprintf("https://%s:%s@gitee.com/%s.git", g.Username, g.AccessToken, pathWithNamespace)
}
