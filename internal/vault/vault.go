package vault

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const (
	VaultAlgo     string = "aes-gcm"
	VaultVersion  string = "v1"
	VaultFileName string = ".urv.vault.yaml"
)

type Vault struct {
	Version string            `yaml:"version"`
	Algo    string            `yaml:"algo"`
	Hashes  map[string]string `yaml:"hashes"`
	Data    string            `yaml:"data"`
}

func (v *Vault) GetByteData() ([]byte, error) {
	res, err := hex.DecodeString(v.Data)
	if err != nil {
		return nil, fmt.Errorf("decoding vault data from hex: %w", err)
	}
	return res, nil
}

func (v *Vault) ValidateForDecrypt() error {
	if v.Version != "" && v.Version != VaultVersion {
		return fmt.Errorf("unsupported vault version: %s", v.Version)
	}
	if v.Algo != VaultAlgo {
		return fmt.Errorf("unsupported vault algo: %s", v.Algo)
	}
	return nil
}

func (v *Vault) SaveToFile(filePath string) error {
	_, err := os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		f, _ := os.Create(filePath)
		f.Close()
	}
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshalling vault to yaml: %w", err)
	}
	err = os.WriteFile(filePath, data, 0o664)
	if err != nil {
		return fmt.Errorf("writing vault data to file %s: %w", filePath, err)
	}
	return nil
}

func NewVaultFromData(data []byte, hashes map[string]string) *Vault {
	strData := hex.EncodeToString(data)
	vaultHashes := map[string]string{}
	for k, v := range hashes {
		vaultHashes[k] = v
	}
	return &Vault{
		Version: VaultVersion,
		Algo:    VaultAlgo,
		Hashes:  vaultHashes,
		Data:    strData,
	}
}

func NewVaultFromFilePath(filePath string) (*Vault, error) {
	var res Vault
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	if res.Hashes == nil {
		res.Hashes = map[string]string{}
	}
	return &res, nil
}

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

func UnpackZipVaultData(basePath string, data []byte) error {
	dataReader := bytes.NewReader(data)
	zr, err := zip.NewReader(dataReader, int64(len(data)))
	if err != nil {
		return fmt.Errorf("creating zip readed from bytes data: %w", err)
	}

	errs := []error{}

	for _, zf := range zr.File {
		// FIXME: Use flag here
		err := extractFileFromZip(zf, basePath, true)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func extractFileFromZip(zf *zip.File, basePath string, forceReplace bool) error {
	cleanName := filepath.Clean(zf.Name)
	if !filepath.IsLocal(cleanName) {
		return fmt.Errorf("unsafe zip path: %s", zf.Name)
	}
	log.Printf("file in decrypted archive: %s", zf.Name)
	fullPath := filepath.Join(basePath, cleanName)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	flags := os.O_WRONLY | os.O_CREATE
	if forceReplace {
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
	_, err = io.Copy(f, zfr)
	if err != nil {
		return fmt.Errorf("coping zipped file data to a file: %w", err)
	}
	return nil
}
