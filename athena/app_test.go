package athena_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athena"
	"github.com/lunagic/athena/athenaservices/queue"
	"gotest.tools/v3/assert"
)

// type RouteTestInterface interface {
// 	GetThing(w http.ResponseWriter, r *http.Request, userID string, shouldBeIgnored TestUser) TestUser
// }

// type RouteTest struct{}

// func (RouteTest) GetThing(w http.ResponseWriter, r *http.Request, userID string, shouldBeIgnored TestUser) TestUser {
// 	return TestUser{}
// }

// func TestRouteInterface(t *testing.T) {
// 	type TestStruct struct {
// 		Timestamp string `json:"@timestamp"`
// 		UpdatedAt time.Time
// 		DeletedAt *time.Time
// 		Timeout   time.Duration
// 		Data      any
// 		MoreData  interface{}
// 	}

// 	service := typescript.New(
// 		typescript.WithRegistry(map[string]reflect.Type{
// 			"TestStruct": reflect.TypeFor[TestStruct](),
// 			"TestUser":   reflect.TypeFor[TestUser](),
// 		}),
// 		typescript.WithAutoRouter("/_backend", reflect.TypeFor[RouteTestInterface](), map[reflect.Type]bool{reflect.TypeFor[TestUser](): true}),
// 	)

// 	testThePackage(t, service)
// }

// func TestRouteNonInterface(t *testing.T) {
// 	type TestStruct struct {
// 		Timestamp string `json:"@timestamp"`
// 		UpdatedAt time.Time
// 		DeletedAt *time.Time
// 		Timeout   time.Duration
// 		Data      any
// 		MoreData  interface{}
// 	}

// 	service := typescript.New(
// 		typescript.WithRegistry(map[string]reflect.Type{
// 			"TestStruct": reflect.TypeFor[TestStruct](),
// 			"TestUser":   reflect.TypeFor[TestUser](),
// 		}),
// 		typescript.WithAutoRouter("/_backend", reflect.TypeFor[RouteTest](), map[reflect.Type]bool{reflect.TypeFor[TestUser](): true}),
// 	)

// 	testThePackage(t, service)
// }

func TestApp(t *testing.T) {
	type User struct {
		Name string
	}

	ctx := t.Context()

	config := athena.NewConfig()

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
