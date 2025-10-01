package local

import "github.com/k8scat/mirror-git-go/pkg/types"

var _ types.TargetGit = &Local{}

type Local struct{}

func (l *Local) CreateRepo(name string, desc string, private bool) error {
	return nil
}

func (l *Local) GetRepoAddr(repoName string) string {
	return ""
}

func (l *Local) IsRepoExist(repoName string) (bool, error) {
	return true, nil
}

func (l *Local) Name() string {
	return "local"
}
