package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Repository represents a Git repository manager
type Repository struct {
	baseDir string
}

// NewRepository creates a new Git repository manager
func NewRepository(baseDir string) *Repository {
	return &Repository{
		baseDir: baseDir,
	}
}

// CloneRepository clones a Git repository to the specified directory
func (r *Repository) CloneRepository(repoURL, branch, projectName string) error {
	targetDir := filepath.Join(r.baseDir, projectName)

	// Ensure the parent directory exists
	if err := os.MkdirAll(r.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	// Remove existing directory if it exists
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	// Clone the repository
	args := []string{"clone"}

	// Add branch specification if provided
	if branch != "" && branch != "main" && branch != "master" {
		args = append(args, "-b", branch)
	}

	args = append(args, repoURL, targetDir)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %s", string(output))
	}

	return nil
}

// PullRepository pulls the latest changes from the remote repository
func (r *Repository) PullRepository(projectName string) error {
	projectDir := filepath.Join(r.baseDir, projectName)

	// Check if directory exists
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return fmt.Errorf("project directory does not exist: %s", projectDir)
	}

	// Get current branch first
	currentBranch, err := r.GetCurrentBranch(projectName)
	if err != nil {
		// Fallback to main if we can't detect current branch
		currentBranch = "main"
	}

	cmd := exec.Command("git", "pull", "origin", currentBranch)
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try with master if main fails
		if currentBranch == "main" {
			cmd = exec.Command("git", "pull", "origin", "master")
			cmd.Dir = projectDir
			output, err = cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("git pull failed: %s", string(output))
			}
		} else {
			return fmt.Errorf("git pull failed: %s", string(output))
		}
	}

	return nil
}

// CheckoutBranch switches to the specified branch
func (r *Repository) CheckoutBranch(projectName, branch string) error {
	projectDir := filepath.Join(r.baseDir, projectName)

	// Check if directory exists
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return fmt.Errorf("project directory does not exist: %s", projectDir)
	}

	// First, fetch all branches
	fetchCmd := exec.Command("git", "fetch", "origin")
	fetchCmd.Dir = projectDir
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %s", string(output))
	}

	// Try to checkout the branch
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutCmd.Dir = projectDir
	output, err := checkoutCmd.CombinedOutput()
	if err != nil {
		// If branch doesn't exist locally, try to create and track it
		createCmd := exec.Command("git", "checkout", "-b", branch, fmt.Sprintf("origin/%s", branch))
		createCmd.Dir = projectDir
		_, createErr := createCmd.CombinedOutput()
		if createErr != nil {
			return fmt.Errorf("git checkout failed: %s", string(output))
		}
		return nil
	}

	return nil
}

// GetCurrentBranch returns the current branch name
func (r *Repository) GetCurrentBranch(projectName string) (string, error) {
	projectDir := filepath.Join(r.baseDir, projectName)

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetRemoteURL returns the remote URL of the repository
func (r *Repository) GetRemoteURL(projectName string) (string, error) {
	projectDir := filepath.Join(r.baseDir, projectName)

	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// GetLastCommit returns information about the last commit
func (r *Repository) GetLastCommit(projectName string) (*CommitInfo, error) {
	projectDir := filepath.Join(r.baseDir, projectName)

	// Get commit hash, author, and message
	cmd := exec.Command("git", "log", "-1", "--pretty=format:%H|%an|%ae|%s|%ad", "--date=iso")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get last commit: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) < 5 {
		return nil, fmt.Errorf("unexpected git log output format")
	}

	return &CommitInfo{
		Hash:      parts[0],
		Author:    parts[1],
		Email:     parts[2],
		Message:   parts[3],
		Timestamp: parts[4],
	}, nil
}

// ListBranches returns all available branches (local and remote)
func (r *Repository) ListBranches(projectName string) ([]string, error) {
	projectDir := filepath.Join(r.baseDir, projectName)

	// Get all branches (local and remote)
	cmd := exec.Command("git", "branch", "-a")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var branches []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Remove current branch indicator
		if strings.HasPrefix(line, "* ") {
			line = strings.TrimPrefix(line, "* ")
		}

		// Skip HEAD references
		if strings.Contains(line, "HEAD ->") {
			continue
		}

		// Clean up remote branch names
		if strings.HasPrefix(line, "remotes/origin/") {
			line = strings.TrimPrefix(line, "remotes/origin/")
		}

		// Avoid duplicates
		exists := false
		for _, existing := range branches {
			if existing == line {
				exists = true
				break
			}
		}

		if !exists && line != "" {
			branches = append(branches, line)
		}
	}

	return branches, nil
}

// ValidateRepository checks if the directory is a valid Git repository
func (r *Repository) ValidateRepository(projectName string) error {
	projectDir := filepath.Join(r.baseDir, projectName)

	// Check if .git directory exists
	gitDir := filepath.Join(projectDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s", projectDir)
	}

	// Try to run a simple git command to verify
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = projectDir
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("invalid git repository: %w", err)
	}

	return nil
}

// GetRepositoryStatus returns the current status of the repository
func (r *Repository) GetRepositoryStatus(projectName string) (*RepositoryStatus, error) {
	projectDir := filepath.Join(r.baseDir, projectName)

	status := &RepositoryStatus{
		ProjectName: projectName,
	}

	// Get current branch
	if branch, err := r.GetCurrentBranch(projectName); err == nil {
		status.CurrentBranch = branch
	}

	// Get remote URL
	if remoteURL, err := r.GetRemoteURL(projectName); err == nil {
		status.RemoteURL = remoteURL
	}

	// Get last commit
	if commit, err := r.GetLastCommit(projectName); err == nil {
		status.LastCommit = commit
	}

	// Check for uncommitted changes
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	if err == nil {
		status.HasUncommittedChanges = strings.TrimSpace(string(output)) != ""
	}

	// Check if we're ahead/behind remote
	cmd = exec.Command("git", "status", "-b", "--porcelain")
	cmd.Dir = projectDir
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) > 0 && strings.Contains(lines[0], "[") {
			if strings.Contains(lines[0], "ahead") {
				status.IsAheadOfRemote = true
			}
			if strings.Contains(lines[0], "behind") {
				status.IsBehindRemote = true
			}
		}
	}

	return status, nil
}

// CommitInfo represents information about a Git commit
type CommitInfo struct {
	Hash      string `json:"hash"`
	Author    string `json:"author"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// RepositoryStatus represents the current status of a Git repository
type RepositoryStatus struct {
	ProjectName           string      `json:"project_name"`
	CurrentBranch         string      `json:"current_branch"`
	RemoteURL             string      `json:"remote_url"`
	LastCommit            *CommitInfo `json:"last_commit"`
	HasUncommittedChanges bool        `json:"has_uncommitted_changes"`
	IsAheadOfRemote       bool        `json:"is_ahead_of_remote"`
	IsBehindRemote        bool        `json:"is_behind_remote"`
}
