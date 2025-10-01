package types

type TargetGit interface {
	IsRepoExist(repoName string) (bool, error)
	CreateRepo(name string, desc string, private bool) error
	GetRepoAddr(repoName string) string
}

type SourceGit interface {
	ListRepos() ([]Repo, error)
	GetRepoAddr(repoName string) string
}
