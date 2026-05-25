package vault

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

const (
	VaultAlgo     string = "aes-gcm"
	VaultFileName string = ".urv.vault.yaml"
)

type Vault struct {
	Algo string `yaml:"algo"`
	Data string `yaml:"data"`
}

func (v *Vault) GetByteData() []byte {
	res, _ := hex.DecodeString(v.Data)
	return res
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

func NewVaultFromData(data []byte) *Vault {
	strData := hex.EncodeToString(data)
	return &Vault{
		Algo: VaultAlgo,
		Data: strData,
	}
}

func NewVaultFromFilePath(filePath string) (*Vault, error) {
	var res *Vault
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(data, res)
	if err != nil {
		return nil, err
	}
	return res, nil
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
	// TODO: create logic to unpack zip data to proper files in the repo
	panic("unimplemented")
}
