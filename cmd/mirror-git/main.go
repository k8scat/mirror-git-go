package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/k8scat/mirror-git-go/pkg/e_gitee_v8"
	"github.com/k8scat/mirror-git-go/pkg/git"
	"github.com/k8scat/mirror-git-go/pkg/gitee"
	"github.com/k8scat/mirror-git-go/pkg/github"
	"github.com/k8scat/mirror-git-go/pkg/gitlab"
	"github.com/k8scat/mirror-git-go/pkg/local"
	"github.com/k8scat/mirror-git-go/pkg/types"
)

var (
	sourceType string
	targetType string
	timeout    int
)

func main() {
	flag.IntVar(&timeout, "timeout", 3600, "timeout in seconds")
	flag.StringVar(&sourceType, "source", git.EGiteeV8, "source git service")
	flag.StringVar(&targetType, "target", git.GitHub, "target git service")
	flag.Parse()

	var sourceGit types.SourceGit
	switch sourceType {
	case git.EGiteeV8:
		sourceGit = e_gitee_v8.NewEnterpriseGiteeV8FromEnv()
	case git.GitHub:
		sourceGit = github.NewGitHubFromEnv()
	default:
		slog.Error("invalid source type", "type", sourceType)
		os.Exit(1)
	}

	var targetGit types.TargetGit
	switch targetType {
	case git.GitLab:
		targetGit = gitlab.NewGitLabFromEnv()
	case git.GitHub:
		targetGit = github.NewGitHubFromEnv()
	case git.Local:
		targetGit = &local.Local{}
	case git.Gitee:
		targetGit = gitee.NewGiteeFromEnv()
	default:
		slog.Error("invalid target type", "type", targetType)
		os.Exit(1)
	}

	workDir := filepath.Join(os.TempDir(), "/repos_"+time.Now().Format("20060102150405"))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		slog.Error("create work dir failed", "error", err, "work_dir", workDir)
		os.Exit(1)
	}
	slog.Info("work dir created", "dir", workDir)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	err := runMirror(ctx, workDir, sourceGit, targetGit)
	if err != nil {
		slog.Error("mirror failed", "error", err)
		os.Exit(1)
	}

	if targetType != "local" {
		slog.Info("cleaning up clone directory", "dir", workDir)
		if err := os.RemoveAll(workDir); err != nil {
			slog.Error("remove clone dir failed", "error", err, "clone_dir", workDir)
		}
	}
}

func runMirror(ctx context.Context, workDir string, sourceGit types.SourceGit, targetGit types.TargetGit) (err error) {
	allRepos, err := sourceGit.ListRepos()
	if err != nil {
		slog.Error("list repos failed", "error", err, "source", sourceType)
		return fmt.Errorf("list repos failed: %w", err)
	}
	if len(allRepos) == 0 {
		slog.Info("no repos found", "source", sourceType)
		return nil
	}

	slog.Info("total repos", "count", len(allRepos), "source", sourceType)

	failedRepos := make([][]string, 0)
	failedReposLock := sync.Mutex{}

	maxWorkers := 5
	sem := make(chan struct{}, maxWorkers)
	defer close(sem)

	// Process allRepos as needed
	for _, repo := range allRepos {
		// Check if context is already cancelled
		select {
		case <-ctx.Done():
			slog.Warn("context cancelled, stopping repo processing", "error", ctx.Err())
			goto waitForCompletion
		default:
		}

		sem <- struct{}{} // Acquire a token

		go func(r types.Repo) {
			defer func() { <-sem }() // Release the token

			err := mirrorRepo(ctx, workDir, r, sourceGit, targetGit)
			if err != nil {
				failedReposLock.Lock()
				failedRepos = append(failedRepos, []string{r.GetPathWithNamespace(), err.Error()})
				failedReposLock.Unlock()
			}
		}(repo)
	}

waitForCompletion:

	// Wait for all goroutines to finish
	for range maxWorkers {
		sem <- struct{}{}
	}

	if len(failedRepos) > 0 {
		slog.Info("some repos mirror failed", "count", len(failedRepos))
		for _, r := range failedRepos {
			slog.Info("failed repo", "repo", r[0], "reason", r[1])
		}
	}
	return nil
}

func mirrorRepo(ctx context.Context, workDir string, repo types.Repo, source types.SourceGit, target types.TargetGit) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("mirror panic: %v", r)
		}
	}()

	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context cancelled before starting: %w", err)
	}

	slog.Info("mirror repo", "repo", repo.GetPathWithNamespace())

	repoDir := workDir + "/" + repo.GetPath() + "_" + time.Now().Format("20060102150405")

	gitUrl := source.GetRepoAddr(repo.GetPathWithNamespace())

	var cloneCmd []string
	if target.Name() == "local" {
		cloneCmd = []string{"git", "clone", gitUrl, repoDir}
	} else {
		cloneCmd = []string{"git", "clone", "--bare", gitUrl, repoDir}
	}

	slog.Info("clone repo", "cmd", cloneCmd)
	cmd := exec.CommandContext(ctx, cloneCmd[0], cloneCmd[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		slog.Error("clone repo failed", "error", err, "cmd", cloneCmd)
		return fmt.Errorf("clone failed: %w", err)
	}

	exists, err := target.IsRepoExist(repo.GetPath())
	if err != nil {
		slog.Error("check repo exist failed", "error", err, "repo", repo)
		return fmt.Errorf("check exist failed: %w", err)
	}
	if !exists {
		slog.Info("repo not exists, create it", "repo", repo.GetPath())
		err := target.CreateRepo(repo.GetPath(), repo.GetDesc(), repo.GetPrivate())
		if err != nil {
			slog.Error("create repo failed", "error", err, "repo", repo)
			return fmt.Errorf("create failed: %w", err)
		}
	}

	pushAddr := target.GetRepoAddr(repo.GetPath())
	if pushAddr != "" {
		pushCmd := []string{
			"git", "push", "--mirror", pushAddr,
		}
		slog.Info("push repo", "cmd", pushCmd)
		cmd = exec.CommandContext(ctx, pushCmd[0], pushCmd[1:]...)
		cmd.Dir = repoDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			slog.Error("push repo failed", "error", err, "cmd", pushCmd)
			return fmt.Errorf("push failed: %w", err)
		}
	}

	slog.Info("mirror repo success", "repo", repo.GetPathWithNamespace())

	return nil
}
