package athena_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athena"
	"github.com/lunagic/athena/athenaservices/database"
	"github.com/lunagic/poseidon/poseidon"
	"gotest.tools/v3/assert"
)

var (
	mockRouterPrefix         = "/" + uuid.NewString()
	mockUnauthorizedResponse = uuid.NewString()
	mockUsername             = uuid.NewString()
	mockPassword             = uuid.NewString()
	mockIndexResponse        = uuid.NewString()
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

func testRequest(t *testing.T, app *athena.App, testCase HTTPTestCase) {
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

func TestAppEmpty(t *testing.T) {
	ctx := t.Context()

	app, err := athena.NewApp(
		ctx,
		athena.NewDefaultConfig(),
	)
	assert.NilError(t, err)

	testRequest(t, app, HTTPTestCase{
		Request: HTTPTestCaseRequest{
			Method: http.MethodGet,
			Path:   "/",
		},
		Expected: HTTPTestCaseResponse{
			Status: http.StatusNotFound,
			Body:   "404 page not found",
		},
	})
}

type User struct {
	Name string `db:"name" json:"name"`
}

func (user User) TableStructure() database.Table {
	return database.Table{
		Name: "user",
	}
}

func RouterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualUsername, actualPassword, ok := r.BasicAuth()
		if !ok || actualUsername != mockUsername || actualPassword != mockPassword {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(mockUnauthorizedResponse))
			return
		}

		next.ServeHTTP(w, r)
	})
}

type UserRequest struct {
	Name string
}

func (userRequest UserRequest) Validate(r *http.Request) error {
	if userRequest.Name == "" {
		return UserFacingError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("name can not be blank"),
		}
	}

	return nil
}

type UserFacingError struct {
	StatusCode int
	Err        error
}

func (err UserFacingError) Error() string {
	return err.Err.Error()
}

type Router struct{}

func (router Router) MyInformation(user User, userRequest UserRequest) (User, error) {
	if user.Name != userRequest.Name {
		return User{}, UserFacingError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("wrong user requested"),
		}
	}

	return user, nil
}

func TestAppStandard(t *testing.T) {
	ctx := t.Context()

	config := athena.NewDefaultConfig()
	config.SQLitePath = fmt.Sprintf("%s/database.sqlite", t.TempDir())

	databaseService, err := config.Database()
	assert.NilError(t, err)

	app, err := athena.NewApp(
		ctx,
		config,
		athena.WithHandler("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(mockIndexResponse))
		})),
		athena.WithTypeScriptOutput("TestingNamespace", io.Discard, map[string]reflect.Type{}),
		athena.WithRouter(
			mockRouterPrefix,
			Router{},
			func(w http.ResponseWriter, r *http.Request, err error) {
				x, ok := err.(UserFacingError)
				if ok {
					poseidon.RespondJSON(w, x.StatusCode, x.Err.Error())
					return
				}
				poseidon.RespondJSON(w, http.StatusInternalServerError, "something went wrong")
			},
			RouterMiddleware,
		),
		athena.WithRouterArgumentProvider(func(w http.ResponseWriter, r *http.Request) (User, error) {
			return User{
				Name: mockUsername,
			}, nil
		}),
		athena.WithDatabaseAutoMigration(databaseService, []database.Entity{
			User{},
		}),
	)
	assert.NilError(t, err)

	// Test the root url loads the index
	testRequest(t, app, HTTPTestCase{
		Request: HTTPTestCaseRequest{
			Method: http.MethodGet,
			Path:   "/",
		},
		Expected: HTTPTestCaseResponse{
			Status: http.StatusOK,
			Body:   mockIndexResponse,
		},
	})

	// Test that non-root paths are also registered
	testRequest(t, app, HTTPTestCase{
		Request: HTTPTestCaseRequest{
			Method: http.MethodGet,
			Path:   "/random/page/that/is/not/directly/registered",
		},
		Expected: HTTPTestCaseResponse{
			Status: http.StatusOK,
			Body:   mockIndexResponse,
		},
	})

	// Test that api requests fail without auth
	testRequest(t, app, HTTPTestCase{
		Request: HTTPTestCaseRequest{
			Method: http.MethodGet,
			Path:   mockRouterPrefix,
			Query: url.Values{
				"method": []string{
					"MyInformation",
				},
			},
		},
		Expected: HTTPTestCaseResponse{
			Status: http.StatusUnauthorized,
			Body:   mockUnauthorizedResponse,
		},
	})

	// Test that api requests succeed with auth
	testRequest(t, app, HTTPTestCase{
		Request: HTTPTestCaseRequest{
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
		Expected: HTTPTestCaseResponse{
			Status: http.StatusOK,
			Body: User{
				Name: mockUsername,
			},
		},
	})

	// Test Payload Decoding Error
	testRequest(t, app, HTTPTestCase{
		Request: HTTPTestCaseRequest{
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
		Expected: HTTPTestCaseResponse{
			Status: http.StatusInternalServerError,
			Body:   "\"something went wrong\"",
		},
	})

	// Test Payload Validation Error
	testRequest(t, app, HTTPTestCase{
		Request: HTTPTestCaseRequest{
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
		Expected: HTTPTestCaseResponse{
			Status: http.StatusBadRequest,
			Body:   "\"name can not be blank\"",
		},
	})

	// Test Router Error
	testRequest(t, app, HTTPTestCase{
		Request: HTTPTestCaseRequest{
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
		Expected: HTTPTestCaseResponse{
			Status: http.StatusBadRequest,
			Body:   "\"wrong user requested\"",
		},
	})
}
