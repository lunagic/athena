package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

func New(key []byte) Vault {
	return Vault{
		key: key,
	}
}

type Vault struct {
	key []byte
}

func (v Vault) Encrypt(text []byte) ([]byte, error) {
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return nil, err
	}

	b := base64.StdEncoding.EncodeToString(text)
	cipherText := make([]byte, aes.BlockSize+len(b))
	iv := cipherText[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	ctr := cipher.NewCTR(block, iv)
	ctr.XORKeyStream(cipherText[aes.BlockSize:], []byte(b))

	return []byte(base64.URLEncoding.EncodeToString(cipherText)), nil
}

func (v Vault) Decrypt(raw []byte) ([]byte, error) {
	s, err := base64.URLEncoding.DecodeString(string(raw))
	if err != nil {
		return nil, err
	}

	text := []byte(s)
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return nil, err
	}

	if len(text) < aes.BlockSize {
		return nil, errors.New("cipher text too short")
	}

	iv := text[:aes.BlockSize]
	text = text[aes.BlockSize:]
	ctr := cipher.NewCTR(block, iv)
	ctr.XORKeyStream(text, text)
	data, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		return nil, err
	}

	return data, nil
}
