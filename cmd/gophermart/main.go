package main

import (
	api "github.com/gearwheels/gophermart_cp1/internal/pkg/api"
	openapischema "github.com/gearwheels/gophermart_cp1/api"
	"net/http"

	"github.com/labstack/echo/v4"
)



func main() {
	server := api.NewServer()

    e := echo.New()

    // api.RegisterHandlers(e, api.NewStrictHandler(
    //     server,
    //     // сюда можно добавить middlewares, если нужно
    //     []api.StrictMiddlewareFunc{},
    // ))
	api.RegisterHandlers(e, server)
	e.GET("/swagger/*", echo.WrapHandler(http.StripPrefix("/swagger/", http.FileServer(http.FS(openapischema.SwaggerUI)))))

    e.Start("127.0.0.1:8080")
}


