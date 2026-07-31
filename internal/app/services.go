package app

import (
	"github.com/mustafmst/universal-repo-vault/internal/archive"
	"github.com/mustafmst/universal-repo-vault/internal/keystore"
)

type Services struct {
	Archiver archive.Archiver
	KeyStore *keystore.FileStore
}

func DefaultServices() Services {
	return Services{
		Archiver: archive.NewZipArchiver(),
		KeyStore: keystore.NewDefaultFileStore(),
	}
}

func (s Services) withDefaults() Services {
	if s.Archiver == nil {
		s.Archiver = archive.NewZipArchiver()
	}
	if s.KeyStore == nil {
		s.KeyStore = keystore.NewDefaultFileStore()
	}
	return s
}
