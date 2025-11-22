package types

type Git interface {
	// Name returns the name of the Git service
	Name() string
}

type TargetGit interface {
	Git

	// IsRepoExist checks if a repository exists
	IsRepoExist(repoName string) (bool, error)

	// CreateRepo creates a new repository
	CreateRepo(name string, desc string, private bool) error

	// GetTargetRepoAddr returns the target repository address
	GetTargetRepoAddr(path string) string
}

type SourceGit interface {
	Git

	// GetSourceRepoAddr returns the source repository address
	GetSourceRepoAddr(pathWithNamespace string) string

	// ListRepos lists all repositories
	ListRepos() ([]Repo, error)
}
