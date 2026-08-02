package archive

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Archiver interface {
	Pack(basePath string, relPaths []string) ([]byte, error)
	PlanUnpack(basePath string, data []byte) ([]EntryPlan, error)
	Unpack(basePath string, data []byte, overwrite bool) error
}

type EntryAction string

const (
	EntryCreate    EntryAction = "create"
	EntryOverwrite EntryAction = "overwrite"
)

type EntryPlan struct {
	Path   string
	Action EntryAction
	Mode   os.FileMode
}

type ZipArchiver struct{}

func NewZipArchiver() *ZipArchiver {
	return &ZipArchiver{}
}

func (za *ZipArchiver) Pack(basePath string, relPaths []string) ([]byte, error) {
	var buff bytes.Buffer
	w := zip.NewWriter(&buff)

	errs := []error{}
	for _, relPath := range relPaths {
		if err := writeFileToZip(w, basePath, relPath); err != nil {
			errs = append(errs, err)
		}
	}

	if err := w.Close(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return buff.Bytes(), nil
}

func writeFileToZip(zw *zip.Writer, basePath string, relPath string) error {
	cleanPath := filepath.ToSlash(filepath.Clean(relPath))
	if !filepath.IsLocal(cleanPath) {
		return fmt.Errorf("unsafe archive path: %s", relPath)
	}
	f, err := os.Open(filepath.Join(basePath, filepath.FromSlash(cleanPath)))
	if err != nil {
		return err
	}
	defer f.Close()

	entry, err := zw.Create(cleanPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(entry, f); err != nil {
		return err
	}
	return nil
}

func (za *ZipArchiver) PlanUnpack(basePath string, data []byte) ([]EntryPlan, error) {
	dataReader := bytes.NewReader(data)
	zr, err := zip.NewReader(dataReader, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("creating zip reader from bytes data: %w", err)
	}

	plans := make([]EntryPlan, 0, len(zr.File))
	for _, zf := range zr.File {
		cleanName := filepath.ToSlash(filepath.Clean(zf.Name))
		if !filepath.IsLocal(cleanName) {
			return nil, fmt.Errorf("unsafe zip path: %s", zf.Name)
		}

		action := EntryCreate
		if info, err := os.Stat(filepath.Join(basePath, filepath.FromSlash(cleanName))); err == nil && !info.IsDir() {
			action = EntryOverwrite
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspecting unpack target %s: %w", cleanName, err)
		}

		plans = append(plans, EntryPlan{Path: cleanName, Action: action, Mode: zf.Mode()})
	}
	return plans, nil
}

func (za *ZipArchiver) Unpack(basePath string, data []byte, overwrite bool) error {
	dataReader := bytes.NewReader(data)
	zr, err := zip.NewReader(dataReader, int64(len(data)))
	if err != nil {
		return fmt.Errorf("creating zip reader from bytes data: %w", err)
	}

	errs := []error{}
	for _, zf := range zr.File {
		if err := extractFileFromZip(zf, basePath, overwrite); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func extractFileFromZip(zf *zip.File, basePath string, overwrite bool) error {
	cleanName := filepath.Clean(zf.Name)
	if !filepath.IsLocal(cleanName) {
		return fmt.Errorf("unsafe zip path: %s", zf.Name)
	}
	fullPath := filepath.Join(basePath, cleanName)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}

	flags := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	f, err := os.OpenFile(fullPath, flags, zf.Mode())
	if err != nil {
		return fmt.Errorf("opening file %s for unpack: %w", fullPath, err)
	}
	defer f.Close()

	zfr, err := zf.Open()
	if err != nil {
		return fmt.Errorf("opening zip file read: %w", err)
	}
	defer zfr.Close()

	if _, err := io.Copy(f, zfr); err != nil {
		return fmt.Errorf("copying zipped file data to a file: %w", err)
	}
	return nil
}
