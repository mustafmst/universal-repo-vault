package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

type AesGcmCipher struct {
	key []byte
}

func (agc *AesGcmCipher) Encrypt(data []byte) ([]byte, error) {
	res := []byte{}
	b, err := aes.NewCipher(agc.key)
	if err != nil {
		return res, err
	}

	gcm, err := cipher.NewGCM(b)
	if err != nil {
		return res, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return res, err
	}

	res = gcm.Seal(nonce, nonce, data, nil)

	return res, nil
}

func (agc *AesGcmCipher) Decrypt(data []byte) ([]byte, error) {
	res := []byte{}
	b, err := aes.NewCipher(agc.key)
	if err != nil {
		return res, err
	}

	gcm, err := cipher.NewGCM(b)
	if err != nil {
		return res, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return res, fmt.Errorf("cipher too small")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func NewAesGcmFromHexKey(hexKey string) (*AesGcmCipher, error) {
	byteKey, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("decoding AES GCM key from hex to bytes: %w", err)
	}
	return NewAesGcm(byteKey)
}

func NewAesGcm(key []byte) (*AesGcmCipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("expected key length is 32 bytes, got: %d", len(key))
	}
	return &AesGcmCipher{
		key,
	}, nil
}

func AesGcmEncrypt(key string, data []byte) ([]byte, error) {
	c, err := NewAesGcmFromHexKey(key)
	if err != nil {
		return nil, err
	}
	return c.Encrypt(data)
}

func AesGcmDecrypt(key string, data []byte) ([]byte, error) {
	c, err := NewAesGcmFromHexKey(key)
	if err != nil {
		return nil, err
	}
	return c.Decrypt(data)
}
