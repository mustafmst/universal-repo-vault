package repo

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var ErrNoRepoFound error = errors.New("no repo found")

const (
	gitCheckErrFormat      string = "git repo check failed: %w"
	currentRepoErrorFormat string = "error getting current repo path: %w"

	tempDir string = ".urvtemp"
)

func checkDirGitRepo(dirPath string) (bool, error) {
	_, err := os.Stat(filepath.Join(dirPath, ".git"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf(gitCheckErrFormat, err)
	} else if err != nil {
		return false, nil
	}

	return true, nil
}

func getRepoPathForPath(dirPath string) (string, error) {
	currentPath, err := filepath.Abs(dirPath)
	if err != nil {
		return "", fmt.Errorf("getting abs path: %w", err)
	}

	for {
		isGitRepo, err := checkDirGitRepo(currentPath)
		if err != nil {
			return "", fmt.Errorf("checking if path is a git repo: %w", err)
		}
		if isGitRepo {
			return currentPath, nil
		}

		parentPath := filepath.Dir(currentPath)
		if currentPath == parentPath {
			return "", ErrNoRepoFound
		}
		currentPath = parentPath
	}
}

func GetCurrentRepoPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf(currentRepoErrorFormat, err)
	}
	return getRepoPathForPath(cwd)
}

func CheckGitignore(repoPath string) error {
	fullPath := filepath.Join(repoPath, ".gitignore")
	data, err := os.ReadFile(fullPath)

	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if err != nil {
		return os.WriteFile(fullPath, []byte(tempDir+"\n"), 0o664)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == tempDir {
			return nil
		}
	}

	contents := string(data)
	if contents != "" && !strings.HasSuffix(contents, "\n") {
		contents += "\n"
	}
	contents += tempDir + "\n"

	return os.WriteFile(fullPath, []byte(contents), 0o664)
}

func IgnoredFiles(repoPath string, relPaths []string) (map[string]bool, error) {
	result := map[string]bool{}
	for _, path := range relPaths {
		result[path] = false
		cmd := exec.Command("git", "check-ignore", "--quiet", "--", path)
		cmd.Dir = repoPath
		err := cmd.Run()
		if err == nil {
			result[path] = true
			continue
		}
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			continue
		}
		return nil, fmt.Errorf("checking ignored file %s: %w", path, err)
	}
	return result, nil
}

func StagedFiles(repoPath string, relPaths []string) (map[string]bool, error) {
	result := map[string]bool{}
	for _, path := range relPaths {
		result[path] = false
	}

	cmd := exec.Command("git", "diff", "--cached", "--name-only", "--")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing staged files: %w", err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := result[line]; ok {
			result[line] = true
		}
	}
	return result, nil
}
