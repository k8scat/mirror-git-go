package gitee

import (
	"fmt"
	"testing"
	"time"
)

func TestCreateRepo(t *testing.T) {
	g := NewGiteeFromEnv()
	fmt.Println(g.AccessToken)
	err := g.CreateRepo("test"+time.Now().Format("20060102150405"), "This is a test repository", true)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("Repository created successfully")
}

func TestIsRepoExist(t *testing.T) {
	g := NewGiteeFromEnv()
	exists, err := g.IsRepoExist("goworker")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(exists)
	if exists {
		fmt.Println("Repository exists")
	} else {
		fmt.Println("Repository does not exist")
	}
}
