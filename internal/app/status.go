package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mustafmst/universal-repo-vault/internal/config"
	"github.com/mustafmst/universal-repo-vault/internal/files"
	"github.com/mustafmst/universal-repo-vault/internal/vault"
)

type OverallStatus string

const (
	OverallSafe            OverallStatus = "safe"
	OverallNeedsEncryption OverallStatus = "needs encryption"
	OverallBrokenSetup     OverallStatus = "broken setup"
)

type FileStatus string

const (
	FileUnchanged FileStatus = "unchanged"
	FileChanged   FileStatus = "changed"
	FileNew       FileStatus = "new"
	FileMissing   FileStatus = "missing"
	FileVaultOnly FileStatus = "vault-only"
)

type StatusFile struct {
	Path   string
	Status FileStatus
}

type StatusReport struct {
	Overall        OverallStatus
	ConfigOK       bool
	VaultOK        bool
	VaultExists    bool
	KeyMapped      bool
	KeyName        string
	KeyFileExists  bool
	KeyLengthValid bool
	Files          []StatusFile
	Warnings       []string
	Errors         []string
}

func StatusRepo(repoPath string) (*StatusReport, error) {
	return StatusRepoWithServices(repoPath, DefaultServices())
}

func StatusRepoWithServices(repoPath string, services Services) (*StatusReport, error) {
	services = services.withDefaults()
	report := &StatusReport{Overall: OverallSafe}

	cfg, err := config.Load(repoPath)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("config .urv.yaml is missing or invalid: %v", err))
		report.finish()
		return report, nil
	}
	report.ConfigOK = true

	keyHealth := services.KeyStore.HealthForRepo(repoPath)
	report.KeyMapped = keyHealth.Mapped
	report.KeyName = keyHealth.KeyName
	report.KeyFileExists = keyHealth.KeyFileExists
	report.KeyLengthValid = keyHealth.KeyLengthValid
	if keyHealth.Err != nil {
		report.Errors = append(report.Errors, keyHealth.Err.Error())
	}

	v, err := vault.NewVaultFromFilePath(filepath.Join(repoPath, vault.VaultFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			report.Errors = append(report.Errors, "vault .urv.vault.yaml is missing")
		} else {
			report.Errors = append(report.Errors, fmt.Sprintf("vault .urv.vault.yaml is invalid: %v", err))
		}
	} else {
		report.VaultExists = true
		if err := v.ValidateForDecrypt(); err != nil {
			report.Errors = append(report.Errors, err.Error())
		} else if _, err := v.GetByteData(); err != nil {
			report.Errors = append(report.Errors, err.Error())
		} else {
			report.VaultOK = true
		}
	}

	foundFiles, err := files.ListAllConfiguredFiles(repoPath, cfg.SecretFiles, cfg.Patterns)
	if err != nil {
		report.Errors = append(report.Errors, err.Error())
		report.finish()
		return report, nil
	}

	hashes := map[string]string{}
	if len(foundFiles) > 0 {
		hashCollection, err := files.NewFileHashCollection(repoPath, foundFiles)
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
		} else {
			hashes = hashCollection.Hashes
		}
	}

	report.Files = classifyStatusFiles(repoPath, cfg.SecretFiles, foundFiles, hashes, vaultHashes(v))
	report.finish()
	return report, nil
}

func vaultHashes(v *vault.Vault) map[string]string {
	if v == nil || v.Hashes == nil {
		return map[string]string{}
	}
	return v.Hashes
}

func classifyStatusFiles(repoPath string, explicit []string, discovered []string, currentHashes map[string]string, oldHashes map[string]string) []StatusFile {
	discoveredSet := map[string]struct{}{}
	for _, path := range discovered {
		discoveredSet[path] = struct{}{}
	}

	resultByPath := map[string]FileStatus{}
	for _, path := range discovered {
		oldHash, ok := oldHashes[path]
		switch {
		case !ok:
			resultByPath[path] = FileNew
		case currentHashes[path] != oldHash:
			resultByPath[path] = FileChanged
		default:
			resultByPath[path] = FileUnchanged
		}
	}

	for _, path := range explicit {
		cleanPath := filepath.ToSlash(filepath.Clean(path))
		if _, ok := discoveredSet[cleanPath]; ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(cleanPath))); errors.Is(err, os.ErrNotExist) {
			resultByPath[cleanPath] = FileMissing
		}
	}

	for path := range oldHashes {
		if _, ok := discoveredSet[path]; !ok {
			if _, already := resultByPath[path]; !already {
				resultByPath[path] = FileVaultOnly
			}
		}
	}

	paths := make([]string, 0, len(resultByPath))
	for path := range resultByPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	result := make([]StatusFile, 0, len(paths))
	for _, path := range paths {
		result = append(result, StatusFile{Path: path, Status: resultByPath[path]})
	}
	return result
}

func (r *StatusReport) finish() {
	if len(r.Errors) > 0 {
		r.Overall = OverallBrokenSetup
		return
	}
	for _, file := range r.Files {
		if file.Status == FileChanged || file.Status == FileNew || file.Status == FileMissing || file.Status == FileVaultOnly {
			r.Overall = OverallNeedsEncryption
			return
		}
	}
	r.Overall = OverallSafe
}
