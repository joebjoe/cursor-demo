package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joebjoe/pg-cursor/internal/cursor"
	"github.com/joebjoe/pg-cursor/internal/handler"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	gomlog "github.com/labstack/gommon/log"
	"k8s.io/utils/env"
)

func main() {
	e := echo.New()
	defer e.Shutdown(context.Background())

	e.Logger.SetLevel(gomlog.INFO)

	e.Use(
		middleware.RequestID(),
		middleware.BodyDump(
			func(c echo.Context, reqb, respb []byte) {
				c.Logger().Infoj(gomlog.JSON{
					"request": gomlog.JSON{
						"body":    string(reqb),
						"headers": c.Request().Header,
						"query":   c.Request().URL.Query(),
					},
					"response": gomlog.JSON{
						"body":    string(respb),
						"headers": c.Response().Header(),
					},
				})
			}),
	)
	e.HideBanner = true

	connStr, ok := os.LookupEnv("DB_CONNECTION")
	if !ok {
		e.Logger.Panic("DB_CONNECTION is not set")
	}

	conn, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		e.Logger.Panicf("failed to connect to db: %v", err)
	}
	defer conn.Close()

	if err := conn.Ping(context.Background()); err != nil {
		e.Logger.Panicf("failed to test connection: %v", err)
	}

	c := cursor.New(conn)
	defer c.Close()

	h := handler.New(c)

	e.GET("/users", h.NewUserSearch)
	e.GET("/users/:cursor", h.UserSearchWithCursor)

	log.Print(e.Start(env.GetString("PORT", ":80")))
}
