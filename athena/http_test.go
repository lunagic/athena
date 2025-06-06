package athena_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athena"
	"github.com/lunagic/athena/athenatest"
	"gotest.tools/v3/assert"
)

var (
	mockRouterPrefix         = "/" + uuid.NewString()
	mockUnauthorizedResponse = uuid.NewString()
	mockUsername             = uuid.NewString()
	mockPassword             = uuid.NewString()
	mockIndexResponse        = uuid.NewString()
)

func TestAppEmpty(t *testing.T) {
	ctx := t.Context()

	app, err := athena.NewApp(
		ctx,
		athena.NewDefaultConfig(),
	)
	assert.NilError(t, err)

	athenatest.TestRequest(t, app, athenatest.HTTPTestCase{
		Request: athenatest.HTTPTestCaseRequest{
			Method: http.MethodGet,
			Path:   "/",
		},
		Expected: athenatest.HTTPTestCaseResponse{
			Status: http.StatusNotFound,
			Body:   "404 page not found",
		},
	})
}

func TestAppStandard(t *testing.T) {
	app, err := BuildTestApp(t)
	assert.NilError(t, err)

	// Test the root url loads the index
	athenatest.TestRequest(t, app, athenatest.HTTPTestCase{
		Request: athenatest.HTTPTestCaseRequest{
			Method: http.MethodGet,
			Path:   "/",
		},
		Expected: athenatest.HTTPTestCaseResponse{
			Status: http.StatusOK,
			Body:   mockIndexResponse,
		},
	})

	// Test that non-root paths are also registered
	athenatest.TestRequest(t, app, athenatest.HTTPTestCase{
		Request: athenatest.HTTPTestCaseRequest{
			Method: http.MethodGet,
			Path:   "/random/page/that/is/not/directly/registered",
		},
		Expected: athenatest.HTTPTestCaseResponse{
			Status: http.StatusOK,
			Body:   mockIndexResponse,
		},
	})

	// Test that api requests fail without auth
	athenatest.TestRequest(t, app, athenatest.HTTPTestCase{
		Request: athenatest.HTTPTestCaseRequest{
			Method: http.MethodGet,
			Path:   mockRouterPrefix,
			Query: url.Values{
				"method": []string{
					"MyInformation",
				},
			},
		},
		Expected: athenatest.HTTPTestCaseResponse{
			Status: http.StatusUnauthorized,
			Body:   mockUnauthorizedResponse,
		},
	})

	// Test that api requests succeed with auth
	athenatest.TestRequest(t, app, athenatest.HTTPTestCase{
		Request: athenatest.HTTPTestCaseRequest{
			Method: http.MethodPost,
			Path:   mockRouterPrefix,
			Query: url.Values{
				"method": []string{
					"MyInformation",
				},
			},
			Modifier: func(request *http.Request) {
				request.SetBasicAuth(mockUsername, mockPassword)
			},
			Body: UserRequest{
				Name: mockUsername,
			},
		},
		Expected: athenatest.HTTPTestCaseResponse{
			Status: http.StatusOK,
			Body: UserModel{
				Name: mockUsername,
			},
		},
	})

	// Test Payload Decoding Error
	athenatest.TestRequest(t, app, athenatest.HTTPTestCase{
		Request: athenatest.HTTPTestCaseRequest{
			Method: http.MethodPost,
			Path:   mockRouterPrefix,
			Query: url.Values{
				"method": []string{
					"MyInformation",
				},
			},
			Modifier: func(request *http.Request) {
				request.SetBasicAuth(mockUsername, mockPassword)
			},
			Body: nil,
		},
		Expected: athenatest.HTTPTestCaseResponse{
			Status: http.StatusInternalServerError,
			Body:   "\"something went wrong\"",
		},
	})

	// Test Payload Validation Error
	athenatest.TestRequest(t, app, athenatest.HTTPTestCase{
		Request: athenatest.HTTPTestCaseRequest{
			Method: http.MethodPost,
			Path:   mockRouterPrefix,
			Query: url.Values{
				"method": []string{
					"MyInformation",
				},
			},
			Modifier: func(request *http.Request) {
				request.SetBasicAuth(mockUsername, mockPassword)
			},
			Body: UserRequest{
				Name: "", // Blank on purpose
			},
		},
		Expected: athenatest.HTTPTestCaseResponse{
			Status: http.StatusBadRequest,
			Body:   "\"name can not be blank\"",
		},
	})

	// Test Router Error
	athenatest.TestRequest(t, app, athenatest.HTTPTestCase{
		Request: athenatest.HTTPTestCaseRequest{
			Method: http.MethodPost,
			Path:   mockRouterPrefix,
			Query: url.Values{
				"method": []string{
					"MyInformation",
				},
			},
			Modifier: func(request *http.Request) {
				request.SetBasicAuth(mockUsername, mockPassword)
			},
			Body: UserRequest{
				Name: "unknown-user",
			},
		},
		Expected: athenatest.HTTPTestCaseResponse{
			Status: http.StatusBadRequest,
			Body:   "\"wrong user requested\"",
		},
	})
}
