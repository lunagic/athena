package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/lunagic/athena/athenaservices/vault"
)

func NewDriverLocal(
	directory string,
	vault vault.Vault,
) (*DriverLocal, error) {
	return &DriverLocal{
		Directory: directory,
		Vault:     vault,
	}, nil
}

type DriverLocal struct {
	Directory    string
	BaseEndpoint string
	Vault        vault.Vault
}

func (driver *DriverLocal) httpPath(filePath string) string {
	return fmt.Sprintf("%s/%s", driver.BaseEndpoint, filePath)
}

func (driver *DriverLocal) absolutePath(filePath string) string {
	return fmt.Sprintf("%s/%s", driver.Directory, filePath)
}

func (driver *DriverLocal) Get(ctx context.Context, filePath string) (io.Reader, error) {
	return os.Open(driver.absolutePath(filePath))
}

func (driver *DriverLocal) Put(ctx context.Context, filePath string, payload io.Reader) error {
	file, err := os.Create(driver.absolutePath(filePath))
	if err != nil {
		return err
	}

	if _, err := io.Copy(file, payload); err != nil {
		return err
	}

	return nil
}

func (driver *DriverLocal) Delete(ctx context.Context, filePath string) error {
	if err := os.Remove(driver.absolutePath(filePath)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	return nil
}

func (driver *DriverLocal) Exists(ctx context.Context, filePath string) (bool, error) {
	if _, err := os.Stat(driver.absolutePath(filePath)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (driver *DriverLocal) IsReady(ctx context.Context) error {
	return nil
}

type urlPayload struct {
	ExpiresAt time.Time
	Path      string
}

func (driver *DriverLocal) PreSignedURL(ctx context.Context, filePath string, expiration time.Duration) (string, error) {
	payload := urlPayload{
		ExpiresAt: time.Now().Add(expiration),
		Path:      filePath,
	}

	message, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	signature, err := driver.Vault.Encrypt(message)
	if err != nil {
		return "", err
	}

	return driver.BaseEndpoint + "/_presigned?" + url.Values{
		"signature": []string{string(signature)},
	}.Encode(), nil
}

func (driver *DriverLocal) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	signature, err := driver.Vault.Decrypt([]byte(r.URL.Query().Get("signature")))
	if err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	payload := urlPayload{}
	if err := json.Unmarshal(signature, &payload); err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if time.Since(payload.ExpiresAt) > 0 {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	reader, err := driver.Get(r.Context(), payload.Path)
	if err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if _, err := io.Copy(w, reader); err != nil {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
}

func (driver *DriverLocal) PublicLink(ctx context.Context, filePath string) (string, error) {
	return driver.httpPath(filePath), nil
}
