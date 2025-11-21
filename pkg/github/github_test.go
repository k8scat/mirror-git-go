package github

import (
	"fmt"
	"testing"
)

func TestIsRepoExist(t *testing.T) {
	g := NewGitHubFromEnv()
	exists, err := g.IsRepoExist("test")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		fmt.Println("Repository exists")
	} else {
		fmt.Println("Repository does not exist")
	}
}

func TestCreateRepo(t *testing.T) {
	g := NewGitHubFromEnv()
	err := g.CreateRepo("test", "This is a test repository", true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestListRepos(t *testing.T) {
	g := NewGitHubFromEnv()
	repos, err := g.ListRepos()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", repos[0])
	for _, repo := range repos {
		fmt.Printf("%+v\n", repo)
	}
	fmt.Printf("total repos: %d\n", len(repos))
}
