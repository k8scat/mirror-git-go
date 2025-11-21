package types

type Git interface {
	// Name returns the name of the Git service
	Name() string

	// GetRepoAddr returns the repository address
	GetRepoAddr(pathWithNamespace string) string
}

type TargetGit interface {
	Git

	// IsRepoExist checks if a repository exists
	IsRepoExist(repoName string) (bool, error)

	// CreateRepo creates a new repository
	CreateRepo(name string, desc string, private bool) error
}

type SourceGit interface {
	Git

	// ListRepos lists all repositories
	ListRepos() ([]Repo, error)
}
