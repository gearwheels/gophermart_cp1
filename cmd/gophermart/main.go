package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"
	"syscall"

	openapischema "github.com/gearwheels/gophermart_cp1/api"
	settings "github.com/gearwheels/gophermart_cp1/internal/config"
	"github.com/gearwheels/gophermart_cp1/internal/accrual"
	"github.com/gearwheels/gophermart_cp1/internal/db"
	authmid "github.com/gearwheels/gophermart_cp1/internal/middleware/auth"
	loggermid "github.com/gearwheels/gophermart_cp1/internal/middleware/logger"
	api "github.com/gearwheels/gophermart_cp1/internal/pkg/api"
	accrualworker "github.com/gearwheels/gophermart_cp1/internal/service/accrual_worker"

	"github.com/labstack/echo/v4"
	echomid "github.com/labstack/echo/v4/middleware"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   "timestamp",
					Value: slog.StringValue(a.Value.Time().Format(time.RFC3339)),
				}
			}
			return a
		},
	}))
	slog.SetDefault(logger)

	settings.AppConfig = settings.Init()

	e := echo.New()

	gdb, err := db.InitDB(settings.AppConfig.DatabaseURI)
	if err != nil {
		slog.Error("Failed to connect to database", slog.String("err", err.Error()))
		panic(err)
	}

	e.Use(loggermid.RequestLogger(logger))
	e.Use(echomid.Decompress())
	e.Use(echomid.Gzip())
	e.Use(authmid.Middleware(gdb))

	server := api.NewServer(gdb)
	api.RegisterHandlers(e, server)

	// Swagger UI (опционально)
	e.GET("/swagger/*", echo.WrapHandler(http.StripPrefix("/swagger/", http.FileServer(http.FS(openapischema.SwaggerUI)))))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	aw := accrualworker.New(gdb, accrual.NewClient(settings.AppConfig.AccrualSystemAddress), settings.AppConfig.WorkerNum)
	go aw.Run(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = e.Shutdown(shutdownCtx)
	}()

	if err := e.Start(settings.AppConfig.RunAddress); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped", slog.String("err", err.Error()))
	}
}
