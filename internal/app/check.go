package app

import "fmt"

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
	return result, nil
}
