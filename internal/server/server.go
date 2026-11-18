package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"go.uber.org/fx"

	"movie-booking/database"
	"movie-booking/modules"
	bookinghttp "movie-booking/modules/booking/presentation/http_api"
	cataloghttp "movie-booking/modules/catalog/presentation/http_api"
	identityapplication "movie-booking/modules/identity/application"
	identityhttp "movie-booking/modules/identity/presentation/http_api"
	systemhttp "movie-booking/modules/system/presentation/http_api"
	appconfig "movie-booking/shared/config"
)

type runtime struct {
	server *http.Server
	errors chan error
}

// Run builds the server dependency graph and owns its complete lifecycle.
func Run(ctx context.Context, config appconfig.Config, logger *slog.Logger) error {
	var running *runtime
	app := fx.New(
		fx.Supply(config, logger),
		database.Module,
		modules.FeatureModules,
		fx.Provide(newRuntime),
		fx.Invoke(registerLifecycle),
		fx.Populate(&running),
		fx.WithLogger(newFxEventLogger),
	)

	startCtx, cancelStart := context.WithTimeout(ctx, config.HTTP.ShutdownTimeout)
	defer cancelStart()
	if err := app.Start(startCtx); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-running.errors:
	}

	stopCtx, cancelStop := context.WithTimeout(context.Background(), config.HTTP.ShutdownTimeout)
	defer cancelStop()
	if err := app.Stop(stopCtx); err != nil {
		return errors.Join(serveErr, fmt.Errorf("stop server: %w", err))
	}
	return serveErr
}

func newRuntime(
	config appconfig.Config,
	logger *slog.Logger,
	authentication *identityapplication.AuthenticateUserQuery,
	systemHandler *systemhttp.Handler,
	catalogHandler *cataloghttp.Handler,
	bookingHandler *bookinghttp.Handler,
) *runtime {
	handler := newRouter(
		config,
		logger,
		identityhttp.Auth(authentication),
		systemHandler,
		catalogHandler,
		bookingHandler,
	)
	return &runtime{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", config.HTTP.Port),
			Handler:      handler,
			IdleTimeout:  config.HTTP.IdleTimeout,
			ReadTimeout:  config.HTTP.ReadTimeout,
			WriteTimeout: config.HTTP.WriteTimeout,
		},
		errors: make(chan error, 1),
	}
}

func registerLifecycle(lifecycle fx.Lifecycle, running *runtime, logger *slog.Logger) {
	lifecycle.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				err := running.server.ListenAndServe()
				if errors.Is(err, http.ErrServerClosed) {
					err = nil
				}
				running.errors <- err
			}()
			logger.Info("http_server_started", "address", running.server.Addr)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("server_shutdown_started")
			if err := running.server.Shutdown(ctx); err != nil {
				return fmt.Errorf("shut down HTTP server: %w", err)
			}
			logger.Info("server_stopped")
			return nil
		},
	})
}
