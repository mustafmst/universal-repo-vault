package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

func Encrypt(key string, data []byte) ([]byte, error) {
	res := []byte{}
	byteKey, err := hex.DecodeString(key)
	if err != nil {
		return res, err
	}
	b, err := aes.NewCipher(byteKey)
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

func Decrypt(key string, data []byte) ([]byte, error) {
	res := []byte{}
	byteKey, err := hex.DecodeString(key)
	if err != nil {
		return res, err
	}
	b, err := aes.NewCipher(byteKey)
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
