package athenatest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/lunagic/athena/athena"
	"gotest.tools/v3/assert"
)

type HTTPTestCase struct {
	Request  HTTPTestCaseRequest
	Expected HTTPTestCaseResponse
}

type HTTPTestCaseRequest struct {
	Method   string
	Path     string
	Query    url.Values
	Body     any
	Headers  http.Header
	Modifier func(request *http.Request)
}

func (testCase HTTPTestCaseRequest) BuildRequest(t *testing.T) *http.Request {
	var body io.Reader
	if testCase.Body != nil {
		bodyBytes, err := json.Marshal(testCase.Body)
		assert.NilError(t, err)
		body = bytes.NewBuffer(bodyBytes)
	}

	requestURL := testCase.Path
	if len(testCase.Query) > 0 {
		requestURL += "?" + testCase.Query.Encode()
	}

	request := httptest.NewRequest(
		testCase.Method,
		requestURL,
		body,
	)

	if testCase.Headers != nil {
		request.Header = testCase.Headers
	}

	if testCase.Modifier != nil {
		testCase.Modifier(request)
	}

	return request
}

type HTTPTestCaseResponse struct {
	Status  int
	Headers http.Header
	Body    any
}

func TestRequest(t *testing.T, app *athena.App, testCase HTTPTestCase) {
	t.Helper()

	recorder := httptest.NewRecorder()

	// Execute the request
	{
		app.Handler().ServeHTTP(
			recorder,
			testCase.Request.BuildRequest(t),
		)
	}

	// Assert status code
	{
		assert.Equal(t, testCase.Expected.Status, recorder.Code)
	}

	// Assert body
	{
		responseBody := strings.TrimSpace(recorder.Body.String())
		expectedBody := ""
		switch typedBody := testCase.Expected.Body.(type) {
		case string:
			expectedBody = typedBody
		default:
			jsonBytes, err := json.Marshal(typedBody)
			assert.NilError(t, err)
			expectedBody = string(jsonBytes)
		}

		assert.Equal(t, expectedBody, responseBody)
	}
}
