package athena

import (
	"context"
	"log/slog"
	"net/http"
	"reflect"

	"github.com/google/uuid"
	"github.com/lunagic/athena/athenaservices/cache"
	"github.com/lunagic/athena/athenaservices/database"
	"github.com/lunagic/poseidon/poseidon"
)

type AppBuilder interface {
	BuildApp(ctx context.Context) (*App, error)
}

func NewApp(
	ctx context.Context,
	config Config,
	configFuncs ...ConfigurationFunc,
) (
	*App,
	error,
) {
	// Build the app with the defaults
	app := &App{
		instanceUUID: uuid.NewString(),
		config:       config,
		handlers:     map[string]http.Handler{},
		logger:       slog.Default(),
		typeScript: typeScriptConfig{
			typesMap:              map[string]reflect.Type{},
			argumentTypesToIgnore: map[reflect.Type]bool{},
		},
		autoRouter: autoRouterConfig{
			argumentMapping: map[reflect.Type]func(w http.ResponseWriter, r *http.Request) (reflect.Value, error){},
			returnMapping:   map[reflect.Type]func(w http.ResponseWriter, r *http.Request, value reflect.Value){},
		},
	}

	// Process all config functions provided by the user
	for _, configFunc := range configFuncs {
		if err := configFunc(app); err != nil {
			return nil, err
		}
	}

	if err := app.calculateTypeScript(); err != nil {
		return nil, err
	}

	if app.database != nil {
		if _, err := app.database.AutoMigrate(ctx, app.databaseAutoMigrationEntities); err != nil {
			return nil, err
		}
	}

	return app, nil
}

type App struct {
	instanceUUID                  string
	config                        Config
	logger                        *slog.Logger
	jobsCacheService              cache.Driver
	jobs                          []BackgroundJob
	typeScript                    typeScriptConfig
	autoRouter                    autoRouterConfig
	databaseAutoMigrationEntities []database.Entity
	database                      *database.Service
	handlers                      map[string]http.Handler
	middlewares                   poseidon.Middlewares
}

func (app *App) Start(ctx context.Context) error {
	if err := app.Background(ctx); err != nil {
		return err
	}

	return app.Serve(ctx)
}

// Start background tasks and serve the application over HTTP
func Run(ctx context.Context, appBuilder AppBuilder) error {
	app, err := appBuilder.BuildApp(ctx)
	if err != nil {
		return err
	}

	return app.Start(ctx)
}
