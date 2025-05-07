package vault_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athenaservices/vault"
	"gotest.tools/v3/assert"
)

func TestVault(t *testing.T) {
	v := vault.New([]byte("secret_key_secret_key_secret_key"))

	originalPlainText := uuid.NewString()

	cypherBytes, err := v.Encrypt([]byte(originalPlainText))
	if err != nil {
		t.Fatal(err)
	}

	assert.Assert(t, string(cypherBytes) != originalPlainText)

	plainTextBytes, err := v.Decrypt(cypherBytes)
	if err != nil {
		t.Fatal(err)
	}

	assert.Assert(t, string(plainTextBytes) == originalPlainText)

}
