package athena_test

import (
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/lunagic/athena/athena"
	"github.com/lunagic/athena/athenaservices/database"
	"github.com/lunagic/poseidon/poseidon"
)

type UserModel struct {
	Name string `db:"name" json:"name"`
}

func (user UserModel) TableStructure() database.Table {
	return database.Table{
		Name: "user",
	}
}

func testingAuthMiddleware(next http.Handler) http.Handler {
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

func (router Router) MyInformation(user UserModel, userRequest UserRequest) (UserModel, error) {
	if user.Name != userRequest.Name {
		return UserModel{}, UserFacingError{
			StatusCode: http.StatusBadRequest,
			Err:        errors.New("wrong user requested"),
		}
	}

	return user, nil
}

func BuildTestApp(t *testing.T) (*athena.App, error) {
	config := athena.NewTestConfig(t)

	databaseService, err := config.Database()
	if err != nil {
		return nil, err
	}

	router := Router{}

	return athena.NewApp(
		t.Context(),
		config,
		athena.WithHandler("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(mockIndexResponse))
		})),
		athena.WithTypeScriptOutput("TestingNamespace", io.Discard, map[string]reflect.Type{}),
		athena.WithRouter(
			mockRouterPrefix,
			router,
			func(w http.ResponseWriter, r *http.Request, err error) {
				x, ok := err.(UserFacingError)
				if ok {
					poseidon.RespondJSON(w, x.StatusCode, x.Err.Error())
					return
				}
				poseidon.RespondJSON(w, http.StatusInternalServerError, "something went wrong")
			},
			testingAuthMiddleware,
		),
		athena.WithRouterArgumentProvider(func(w http.ResponseWriter, r *http.Request) (UserModel, error) {
			return UserModel{
				Name: mockUsername,
			}, nil
		}),
		athena.WithDatabaseAutoMigration(databaseService, []database.Entity{
			UserModel{},
		}),
	)
}
