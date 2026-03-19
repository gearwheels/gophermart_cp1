package main

import (
	"gophermart_cp1/api"
    "oapiexample/pkg/api"
	"embed"
    "github.com/labstack/echo/v4"
)



func main() {
	server := api.NewServer()

    e := echo.New()

    api.RegisterHandlers(e, api.NewStrictHandler(
        server,
        // сюда можно добавить middlewares, если нужно
        []api.StrictMiddlewareFunc{},
    ))

    e.Start("127.0.0.1:8080")
	e.GET("/swagger/*", echo.WrapHandler(http.StripPrefix("/swagger/", http.FileServer(http.FS(api.swaggerUI)))))
}
