package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	} else {
		report.ConfigOK = true
	}

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
			report.VaultExists = true
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

	if !report.ConfigOK {
		report.finish()
		return report, nil
	}

	warnings, validationErrors, err := validateStatusConfig(repoPath, cfg)
	if err != nil {
		return nil, err
	}
	report.Warnings = append(report.Warnings, warnings...)
	report.Errors = append(report.Errors, validationErrors...)
	if len(validationErrors) > 0 {
		report.finish()
		return report, nil
	}

	foundFiles, err := files.ListAllConfiguredFiles(repoPath, cfg.SecretFiles, cfg.Patterns)
	if err != nil {
		return nil, fmt.Errorf("listing configured files for status: %w", err)
	}
	for _, pattern := range cfg.Patterns {
		matched := false
		for _, path := range foundFiles {
			if ok, _ := filepath.Match(pattern, filepath.Base(path)); ok {
				matched = true
				break
			}
		}
		if !matched {
			report.Warnings = append(report.Warnings, fmt.Sprintf("pattern matched no files: %s", pattern))
		}
	}

	foundFiles, validationErrors = validateStatusDiscoveredFiles(foundFiles)
	report.Errors = append(report.Errors, validationErrors...)

	hashes := map[string]string{}
	if len(foundFiles) > 0 {
		hashCollection, err := files.NewFileHashCollection(repoPath, foundFiles)
		if err != nil {
			return nil, fmt.Errorf("hashing configured files for status: %w", err)
		} else {
			hashes = hashCollection.Hashes
		}
	}

	report.Files, err = classifyStatusFiles(repoPath, cfg.SecretFiles, foundFiles, hashes, vaultHashes(v))
	if err != nil {
		return nil, err
	}
	report.finish()
	return report, nil
}

func validateStatusConfig(repoPath string, cfg *config.Config) (warnings []string, validationErrors []string, inspectionErr error) {
	for _, pattern := range cfg.Patterns {
		if _, err := filepath.Match(pattern, ""); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("invalid file pattern %q: %v", pattern, err))
		}
	}

	if cfg.ArchiverName() != "zip" {
		validationErrors = append(validationErrors, fmt.Sprintf("unsupported archiver %q", cfg.ArchiverName()))
	}
	if cfg.CipherName() != "aes-gcm" {
		validationErrors = append(validationErrors, fmt.Sprintf("unsupported cypher %q", cfg.CipherName()))
	}

	for _, path := range cfg.SecretFiles {
		cleanPath := filepath.ToSlash(filepath.Clean(path))
		if !filepath.IsLocal(cleanPath) {
			validationErrors = append(validationErrors, fmt.Sprintf("unsafe explicit file path %q", path))
			continue
		}
		if isReservedStatusPath(cleanPath) {
			validationErrors = append(validationErrors, fmt.Sprintf("reserved path configured as secret file %q", path))
		}

		info, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(cleanPath)))
		if err == nil && info.IsDir() {
			validationErrors = append(validationErrors, fmt.Sprintf("configured secret file is a directory %q", path))
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("inspecting configured secret file %q: %w", path, err)
		}
	}

	return warnings, validationErrors, nil
}

func validateStatusDiscoveredFiles(foundFiles []string) (safeFiles []string, errors []string) {
	for _, path := range foundFiles {
		if isReservedStatusPath(path) {
			errors = append(errors, fmt.Sprintf("reserved file selected by configuration %q", path))
			continue
		}
		safeFiles = append(safeFiles, path)
	}
	return safeFiles, errors
}

func isReservedStatusPath(path string) bool {
	for _, reservedPath := range []string{".git", ".urv.yaml", vault.VaultFileName, files.LockFileName} {
		if path == reservedPath || strings.HasPrefix(path, reservedPath+"/") {
			return true
		}
	}
	return false
}

func vaultHashes(v *vault.Vault) map[string]string {
	if v == nil || v.Hashes == nil {
		return map[string]string{}
	}
	return v.Hashes
}

func classifyStatusFiles(repoPath string, explicit []string, discovered []string, currentHashes map[string]string, oldHashes map[string]string) ([]StatusFile, error) {
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
		_, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(cleanPath)))
		if errors.Is(err, os.ErrNotExist) {
			resultByPath[cleanPath] = FileMissing
		} else if err != nil {
			return nil, fmt.Errorf("inspecting configured secret file %q: %w", cleanPath, err)
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
	return result, nil
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
