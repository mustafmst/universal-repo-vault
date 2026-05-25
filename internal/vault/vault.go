package vault

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func CreateZipVaultData(basePath string, filePaths []string) ([]byte, error) {
	var buff bytes.Buffer

	w := zip.NewWriter(&buff)

	errs := []error{}
	for _, f := range filePaths {
		err := writeFileToZip(w, basePath, f)
		if err != nil {
			errs = append(errs, err)
		}
	}

	_ = w.Close()

	if len(errs) > 0 {
		return []byte{}, errors.Join(errs...)
	}

	return buff.Bytes(), nil
}

func writeFileToZip(zw *zip.Writer, basePath string, filePath string) error {
	f, err := os.Open(filepath.Join(basePath, filePath))
	if err != nil {
		return err
	}

	defer f.Close()

	entry, err := zw.Create(filePath)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, f)
	if err != nil {
		return err
	}
	return nil
}

func UnpackZipVaultData(basePath string, data []string) error {
	// TODO: create logic to unpack zip data to proper files in the repo
	panic("unimplemented")
}

func SaveVaultFile(repoPath string, encryptedData []byte, algorythm string, lockfileHash string) error {
	panic("unimplemented")
}
