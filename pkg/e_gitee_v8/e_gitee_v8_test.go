package e_gitee_v8

import (
	"fmt"
	"testing"
)

func TestListRepos(t *testing.T) {
	g := NewEnterpriseGiteeV8FromEnv()

	repos, err := g.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	for _, repo := range repos {
		fmt.Printf("%+v\n", repo)
	}
	fmt.Printf("%+v\n", repos[0])
}
