package types

type Repo interface {
	GetPath() string
	GetPathWithNamespace() string
	GetDesc() string
	GetPrivate() bool
}

type RepoImpl struct {
	Path              string
	PathWithNamespace string
	Desc              string
	Private           bool
}

func NewRepo(path, pathWithNamespace, desc string, private bool) Repo {
	return &RepoImpl{
		Path:              path,
		PathWithNamespace: pathWithNamespace,
		Desc:              desc,
		Private:           private,
	}
}

func (r *RepoImpl) GetPath() string {
	return r.Path
}

func (r *RepoImpl) GetPathWithNamespace() string {
	return r.PathWithNamespace
}

func (r *RepoImpl) GetDesc() string {
	return r.Desc
}

func (r *RepoImpl) GetPrivate() bool {
	return r.Private
}
