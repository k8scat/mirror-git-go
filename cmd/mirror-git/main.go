package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/k8scat/mirror-git-go/pkg/e_gitee_v8"
	"github.com/k8scat/mirror-git-go/pkg/github"
	"github.com/k8scat/mirror-git-go/pkg/gitlab"
	"github.com/k8scat/mirror-git-go/pkg/types"
)

const cloneDir = "repos"

var (
	sourceType string
	targetType string
)

func main() {
	flag.StringVar(&sourceType, "source", "gitee", "source git service (gitee)")
	flag.StringVar(&targetType, "target", "github", "target git service (gitlab|github)")
	flag.Parse()

	var sourceGit types.SourceGit
	switch sourceType {
	case "gitee":
		sourceGit = e_gitee_v8.NewEnterpriseGiteeV8FromEnv()
	default:
		slog.Error("invalid source type", "type", sourceType)
		os.Exit(1)
	}

	var targetGit types.TargetGit
	switch targetType {
	case "gitlab":
		targetGit = gitlab.NewGitLabFromEnv()
	case "github":
		targetGit = github.NewGitHubFromEnv()
	default:
		slog.Error("invalid target type", "type", targetType)
		os.Exit(1)
	}

	go runMirror(sourceGit, targetGit)

	ch := make(chan os.Signal, 1)
	defer close(ch)

	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	<-ch
	slog.Info("received interrupt signal, exiting...")
}

func runMirror(sourceGit types.SourceGit, targetGit types.TargetGit) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("mirror panic", "error", r)
		}
	}()
	defer os.RemoveAll(cloneDir)

	allRepos, err := sourceGit.ListRepos()
	if err != nil {
		slog.Error("list repos failed", "error", err, "source", sourceType)
		os.Exit(1)
	}
	if len(allRepos) == 0 {
		slog.Info("no repos found", "source", sourceType)
		return
	}

	slog.Info("total repos", "count", len(allRepos), "source", sourceType)

	failedRepos := make([][]string, 0)
	failedReposLock := sync.Mutex{}

	maxWorkers := 5
	sem := make(chan struct{}, maxWorkers)
	defer close(sem)

	// Process allRepos as needed
	for _, repo := range allRepos {
		sem <- struct{}{} // Acquire a token

		go func(r types.Repo) {
			defer func() {
				if e := recover(); e != nil {
					slog.Error("mirror repo panic", "error", e, "repo", r.GetPathWithNamespace())
					failedReposLock.Lock()
					defer failedReposLock.Unlock()
					failedRepos = append(failedRepos, []string{r.GetPathWithNamespace(), fmt.Sprintf("panic: %v", e)})
				}
			}()
			defer func() { <-sem }() // Release the token

			err := mirrorGiteeRepo(repo, sourceGit, targetGit)
			if err != nil {
				failedRepos = append(failedRepos, []string{repo.GetPathWithNamespace(), err.Error()})
			}
		}(repo)
	}

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
}

func mirrorGiteeRepo(repo types.Repo, source types.SourceGit, target types.TargetGit) error {
	slog.Info("mirror repo", "repo", repo.GetPathWithNamespace())

	repo_dir := cloneDir + "/" + repo.GetPath() + "_" + time.Now().Format("20060102150405")
	defer os.RemoveAll(repo_dir)

	gitUrl := source.GetRepoAddr(repo.GetPathWithNamespace())
	cloneCmd := []string{
		"git", "clone", "--bare", gitUrl, repo_dir,
	}
	slog.Info("clone repo", "cmd", cloneCmd)
	cmd := exec.Command(cloneCmd[0], cloneCmd[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
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

	pushCmd := []string{
		"git", "push", "--mirror", target.GetRepoAddr(repo.GetPath()),
	}
	slog.Info("push repo", "cmd", pushCmd)
	cmd = exec.Command(pushCmd[0], pushCmd[1:]...)
	cmd.Dir = repo_dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		slog.Error("push repo failed", "error", err, "cmd", pushCmd)
		return fmt.Errorf("push failed: %w", err)
	}
	slog.Info("mirror repo success", "repo", repo.GetPathWithNamespace())

	return nil
}
