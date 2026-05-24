package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var ErrNoRepoFound error = errors.New("no repo found")

const (
	gitCheckErrFormat      string = "git repo check failed: %w"
	currentRepoErrorFormat string = "error getting current repo path: %w"
)

func checkDirGitRepo(dirPath string) (bool, error) {
	gitDirStat, err := os.Stat(filepath.Join(dirPath, ".git"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf(gitCheckErrFormat, err)
	} else if err != nil {
		return false, nil
	}

	if gitDirStat.IsDir() {
		return true, nil
	}

	return false, nil
}

func getRepoPathForPath(dirPath string) (string, error) {
	if dirPath == "/" || dirPath == filepath.Dir(dirPath) {
		return "", ErrNoRepoFound
	}
	isGitRepo, err := checkDirGitRepo(dirPath)
	if err != nil {
		return "", err
	}
	if isGitRepo {
		return dirPath, nil
	}
	return getRepoPathForPath(filepath.Dir(dirPath))
}

func GetCurrentRepoPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf(currentRepoErrorFormat, err)
	}
	return getRepoPathForPath(cwd)
}
