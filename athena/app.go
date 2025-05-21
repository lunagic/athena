package athena

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"reflect"

	"github.com/lunagic/athena/athena/internal/agenda"
	"github.com/lunagic/athena/athenaservices/cache"
	"github.com/lunagic/athena/athenaservices/database"
	"github.com/lunagic/poseidon/poseidon"
)

func NewApp(
	ctx context.Context,
	config AppConfig,
	configFuncs ...AppConfigFunc,
) (
	*App,
	error,
) {
	// Build the service with the defaults
	service := &App{
		config:   config,
		handlers: map[string]http.Handler{},
		logger:   slog.Default(),
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
		if err := configFunc(service); err != nil {
			return nil, err
		}
	}

	if err := service.calculateTypeScript(); err != nil {
		return nil, err
	}

	if service.database != nil {
		if _, err := service.database.AutoMigrate(ctx, service.databaseAutoMigrationEntities); err != nil {
			return nil, err
		}
	}

	if err := service.buildHandler(); err != nil {
		return nil, err
	}

	return service, nil
}

type App struct {
	config                        AppConfig
	httpHandler                   http.Handler
	logger                        *slog.Logger
	jobs                          []agenda.Job
	jobCacheDriver                cache.Driver
	typeScript                    typeScriptConfig
	autoRouter                    autoRouterConfig
	databaseAutoMigrationEntities []database.Entity
	database                      *database.Service
	handlers                      map[string]http.Handler
	middlewares                   poseidon.Middlewares
}

func (app *App) Start(ctx context.Context) error {
	go func(ctx context.Context) {
		_ = agenda.EverySecond(ctx, app.backgroundTask)
	}(ctx)

	listener, err := net.Listen("tcp", app.config.ListenAddr())
	if err != nil {
		return err
	}

	app.logger.Info(
		"Server Listen on HTTP",
		"addr", fmt.Sprintf("http://%s", listener.Addr().String()),
	)

	return (&http.Server{
		Handler: app.httpHandler,
		Addr:    app.config.ListenAddr(),
	}).Serve(listener)
}

func (app App) Handler() http.Handler {
	return app.httpHandler
}

func (app App) backgroundTask(ctx context.Context) error {
	if app.jobCacheDriver == nil {
		return nil
	}

	if len(app.jobs) == 0 {
		return nil
	}

	return background(ctx, app.jobCacheDriver, app.jobs)
}
