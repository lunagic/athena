package storage_test

import (
	"net/http/httptest"
	"testing"

	"github.com/lunagic/athena/athenaservices/storage"
	"github.com/lunagic/athena/athenaservices/vault"
	"gotest.tools/v3/assert"
)

func Test_Driver_Local(t *testing.T) {
	t.Parallel()

	driver, err := storage.NewDriverLocal(t.TempDir(), vault.New([]byte("secret_key_secret_key_secret_key")))
	assert.NilError(t, err)

	server := httptest.NewServer(driver)

	driver.BaseEndpoint = server.URL

	testSuite(t, driver)
}
