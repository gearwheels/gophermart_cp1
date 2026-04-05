package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	openapischema "github.com/gearwheels/gophermart_cp1/api"
	settings "github.com/gearwheels/gophermart_cp1/internal/config"
	loggermid "github.com/gearwheels/gophermart_cp1/internal/middleware/logger"
	api "github.com/gearwheels/gophermart_cp1/internal/pkg/api"

	"github.com/labstack/echo/v4"
)

func main() {
	server := api.NewServer()

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

	e.Use(loggermid.RequestLogger(logger))
	// api.RegisterHandlers(e, api.NewStrictHandler(
	//     server,
	//     // сюда можно добавить middlewares, если нужно
	//     []api.StrictMiddlewareFunc{},
	// ))
	api.RegisterHandlers(e, server)
	e.GET("/swagger/*", echo.WrapHandler(http.StripPrefix("/swagger/", http.FileServer(http.FS(openapischema.SwaggerUI)))))

	e.Start("127.0.0.1:8080")
}
