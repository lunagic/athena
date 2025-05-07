package storage_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athenaservices/storage"
	"gotest.tools/v3/assert"
)

func testSuite(t *testing.T, driver storage.Driver) {
	fileName := uuid.NewString()
	fileContents := uuid.NewString()

	{ // Confirm file does not already exist
		found, err := driver.Exists(t.Context(), fileName)
		if err != nil {
			t.Fatal(err)
		}

		if found {
			t.Fatalf("file found before putting the file, bad test: %s", fileName)
		}
	}

	{ // Put the file in storage
		if err := driver.Put(
			t.Context(),
			fileName,
			strings.NewReader(fileContents),
		); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_ = driver.Delete(context.Background(), fileName)
		})
	}

	{ // Confirm the file is now in storage
		found, err := driver.Exists(t.Context(), fileName)
		if err != nil {
			t.Fatal(err)
		}

		if !found {
			t.Fatalf("File not found after putting it")
		}
	}

	{ // Confirm the file contents
		reader, err := driver.Get(t.Context(), fileName)
		if err != nil {
			t.Fatal(err)
		}
		actualContents, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, string(actualContents), fileContents)
	}

	{ // Get the presigned url
		url, err := driver.PreSignedURL(t.Context(), fileName, time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		response, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = response.Body.Close()
		}()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, string(body), fileContents)
	}

	{ // Get the public url
		url, err := driver.PublicLink(t.Context(), fileName)
		if err != nil {
			t.Fatal(err)
		}

		response, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}

		assert.Assert(t, response.StatusCode >= 400)
	}

	{ // Delete the file
		if err := driver.Delete(t.Context(), fileName); err != nil {
			t.Fatal(err)
		}
	}

	{ // Confirm it no longer exists
		found, err := driver.Exists(t.Context(), fileName)
		if err != nil {
			t.Fatal(err)
		}

		if found {
			t.Fatalf("file found after deleting: %s", fileName)
		}
	}

	{ // Confirm deleting a file that does not exist does not error out
		if err := driver.Delete(t.Context(), fileName); err != nil {
			t.Fatal(err)
		}
	}
}
