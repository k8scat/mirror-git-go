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

func TestListProtectedBranches(t *testing.T) {
	g := NewGitLabFromEnv()
	// Use your actual project ID or namespace/project-name format
	projectID := "user/repo" // Replace with actual project
	branches, err := g.ListProtectedBranches(projectID)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Found %d protected branches\n", len(branches))
	for _, branch := range branches {
		fmt.Printf("Protected branch: %s (ID: %d)\n", branch.Name, branch.ID)
	}
}

func TestUnprotectBranch(t *testing.T) {
	g := NewGitLabFromEnv()
	// Use your actual project ID and branch name
	projectID := "user/repo" // Replace with actual project
	branchName := "main"     // Replace with actual branch name

	err := g.UnprotectBranch(projectID, branchName)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Successfully unprotected branch: %s\n", branchName)
}
