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
	_, err := os.Stat(filepath.Join(dirPath, ".git"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf(gitCheckErrFormat, err)
	} else if err != nil {
		return false, nil
	}

	return true, nil
}

func getRepoPathForPath(dirPath string) (string, error) {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return "", err
	}
	if absPath == "/" || absPath == filepath.Dir(absPath) {
		return "", ErrNoRepoFound
	}
	isGitRepo, err := checkDirGitRepo(absPath)
	if err != nil {
		return "", err
	}
	if isGitRepo {
		return absPath, nil
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
