package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewVaultFromDataStoresVersionAlgoHashesAndHexData(t *testing.T) {
	hashes := map[string]string{".env": "abc123"}

	v := NewVaultFromData([]byte("secret"), hashes)
	hashes[".env"] = "changed"

	if v.Version != VaultVersion {
		t.Fatalf("expected version %q, got %q", VaultVersion, v.Version)
	}
	if v.Algo != VaultAlgo {
		t.Fatalf("expected algo %q, got %q", VaultAlgo, v.Algo)
	}
	if v.Hashes[".env"] != "abc123" {
		t.Fatalf("expected vault hash copy, got %#v", v.Hashes)
	}

	data, err := v.GetByteData()
	if err != nil {
		t.Fatalf("expected no decode error, got %v", err)
	}
	if string(data) != "secret" {
		t.Fatalf("expected decoded data %q, got %q", "secret", string(data))
	}
}

func TestNewVaultFromFilePathLoadsV1Vault(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, VaultFileName)
	contents := []byte("version: v1\nalgo: aes-gcm\nhashes:\n  .env: abc123\ndata: 736563726574\n")
	if err := os.WriteFile(vaultPath, contents, 0o644); err != nil {
		t.Fatal(err)
	}

	v, err := NewVaultFromFilePath(vaultPath)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if v.Version != VaultVersion || v.Algo != VaultAlgo || v.Hashes[".env"] != "abc123" {
		t.Fatalf("unexpected vault: %#v", v)
	}
}

func TestSaveToFileWritesCompatibleVaultYAML(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, VaultFileName)
	v := NewVaultFromData([]byte("secret"), map[string]string{".env": "abc123"})

	if err := v.SaveToFile(vaultPath); err != nil {
		t.Fatalf("expected save to succeed, got %v", err)
	}

	loaded, err := NewVaultFromFilePath(vaultPath)
	if err != nil {
		t.Fatalf("expected load to succeed, got %v", err)
	}
	if loaded.Version != VaultVersion {
		t.Fatalf("expected version %q, got %q", VaultVersion, loaded.Version)
	}
	if loaded.Algo != VaultAlgo {
		t.Fatalf("expected algo %q, got %q", VaultAlgo, loaded.Algo)
	}
	if loaded.Hashes[".env"] != "abc123" {
		t.Fatalf("unexpected hashes: %#v", loaded.Hashes)
	}
	data, err := loaded.GetByteData()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "secret" {
		t.Fatalf("expected decoded data, got %q", string(data))
	}
}

func TestValidateForDecryptRejectsUnsupportedVersionAndAlgo(t *testing.T) {
	tests := []struct {
		name string
		v    Vault
		want string
	}{
		{
			name: "unsupported version",
			v:    Vault{Version: "v2", Algo: VaultAlgo},
			want: "unsupported vault version",
		},
		{
			name: "unsupported algo",
			v:    Vault{Version: VaultVersion, Algo: "other"},
			want: "unsupported vault algo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.v.ValidateForDecrypt()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error to contain %q, got %v", tt.want, err)
			}
		})
	}
}

func TestGetByteDataRejectsInvalidHex(t *testing.T) {
	v := Vault{Data: "not-hex"}

	data, err := v.GetByteData()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if data != nil {
		t.Fatalf("expected nil data, got %v", data)
	}
}
