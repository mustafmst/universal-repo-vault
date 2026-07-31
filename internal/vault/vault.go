package vault

import (
	"encoding/hex"
	"fmt"
	"os"

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
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshalling vault to yaml: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0o664); err != nil {
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
