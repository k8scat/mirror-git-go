package gitlab

import (
	"fmt"
	"testing"
)

func TestIsRepoExist(t *testing.T) {
	g := NewGitLabFromEnv()
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
	g := NewGitLabFromEnv()
	err := g.CreateRepo("test", "This is a test repository", true)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Repository created successfully")
}
