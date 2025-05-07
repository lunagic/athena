package athena_test

import (
	"bytes"
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athena"
	"github.com/lunagic/athena/athenaservices/queue"
	"gotest.tools/v3/assert"
)

type TestUser struct {
	Username string
}

type RouteTestInterface interface {
	GetThing(w http.ResponseWriter, r *http.Request, userID string, shouldBeIgnored TestUser) TestUser
}

type RouteTestStruct struct{}

func (RouteTestStruct) GetThing(w http.ResponseWriter, r *http.Request, userID string, shouldBeIgnored TestUser) TestUser {
	return TestUser{}
}

func TestRouteInterface(t *testing.T) {
	_, err := athena.NewApp(
		t.Context(),
		athena.NewConfig(),
		athena.WithTypeScriptOutput(
			bytes.NewBufferString(""),
			map[string]reflect.Type{
				"user": reflect.TypeFor[TestUser](),
			},
		),
		athena.WithRouter[RouteTestInterface]("/_api",
			RouteTestStruct{},
		),
	)

	assert.NilError(t, err)
}

func TestRouteNonInterface(t *testing.T) {
	_, err := athena.NewApp(
		t.Context(),
		athena.NewConfig(),
		athena.WithTypeScriptOutput(
			bytes.NewBufferString(""),
			map[string]reflect.Type{
				"user": reflect.TypeFor[TestUser](),
			},
		),
		athena.WithRouter("/_api",
			RouteTestStruct{},
		),
	)

	assert.NilError(t, err)
}

func TestApp(t *testing.T) {
	type User struct {
		Name string
	}

	ctx := t.Context()

	config := athena.NewConfig()
	config.AppHTTPPort = 0 // Make sure a random port is selected for each instance

	queueDriver, err := queue.NewDriverMemory()
	assert.NilError(t, err)

	userQueue, err := queue.NewQueue[User](ctx, queueDriver, uuid.NewString())
	assert.NilError(t, err)

	app, err := athena.NewApp(
		t.Context(),
		config,
		athena.WithQueue(ctx, userQueue, func(ctx context.Context, payload User) error {
			return nil
		}),
	)

	assert.NilError(t, err)

	startApp(t, app)
	startApp(t, app)
	startApp(t, app)
	startApp(t, app)
	startApp(t, app)

	time.Sleep(time.Second * 2)
	assert.NilError(t, nil)
}
