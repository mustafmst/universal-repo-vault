package app

import (
	"fmt"

	"github.com/mustafmst/universal-repo-vault/internal/repo"
)

type CheckResult struct {
	Report   *StatusReport
	Safe     bool
	Messages []string
}

func CheckRepo(repoPath string) (*CheckResult, error) {
	return CheckRepoWithServices(repoPath, DefaultServices())
}

func CheckRepoWithServices(repoPath string, services Services) (*CheckResult, error) {
	report, err := StatusRepoWithServices(repoPath, services)
	if err != nil {
		return nil, err
	}

	result := &CheckResult{Report: report, Safe: report.Overall == OverallSafe}
	result.Messages = append(result.Messages, report.Errors...)
	for _, file := range report.Files {
		if file.Status != FileUnchanged {
			result.Messages = append(result.Messages, fmt.Sprintf("%s %s", file.Path, file.Status))
		}
	}
	paths := make([]string, 0, len(report.Files))
	for _, file := range report.Files {
		if file.Status != FileVaultOnly {
			paths = append(paths, file.Path)
		}
	}
	indexed, err := repo.IndexedFiles(repoPath, paths)
	if err != nil {
		return nil, err
	}
	staged, err := repo.StagedFiles(repoPath, paths)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		if indexed[path] {
			result.Safe = false
			result.Messages = append(result.Messages, fmt.Sprintf("configured plaintext file is tracked in the Git index: %s", path))
			continue
		}
		if staged[path] {
			result.Safe = false
			result.Messages = append(result.Messages, fmt.Sprintf("configured plaintext file has a staged Git change: %s", path))
		}
	}
	return result, nil
}
