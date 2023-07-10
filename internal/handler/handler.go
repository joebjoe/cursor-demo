package handler

import (
	"fmt"
	"net/http"

	"github.com/joebjoe/pg-cursor/internal/cursor"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
)

type Handler struct {
	cursor cursor.Cursor
}

func New(c cursor.Cursor) *Handler {
	return &Handler{
		cursor: c,
	}
}

func (h *Handler) NewUserSearch(c echo.Context) error {
	reqID := getRequestID(c)
	ctx := c.Request().Context()

	var req newSearchRequest
	if err := c.Bind(&req); err != nil {
		c.Logger().Errorj(log.JSON{
			"id":      reqID,
			"message": "failed to bind request",
			"error":   err.Error(),
		})

		return err
	}

	// if err := validateRequest(req); err != nil {
	// 	return c.String(http.StatusBadRequest, err.Error())
	// }

	if err := h.cursor.Declare(ctx, reqID, fmt.Sprintf("SELECT * FROM users WHERE id %s", req.IDMatch)); err != nil {
		c.Logger().Errorj(log.JSON{
			"id":      reqID,
			"message": "failed to declare cursor",
			"error":   err.Error(),
		})

		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	var users []user
	more, err := h.cursor.Fetch(ctx, &users, reqID, req.PageSize)
	if err != nil {
		c.Logger().Errorj(log.JSON{
			"id":      reqID,
			"message": "failed to fetch",
			"error":   err.Error(),
		})

		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, userResponse{
		CursorID: reqID,
		Done:     !more,
		Users:    users,
	})
}

func (h *Handler) UserSearchWithCursor(c echo.Context) error {
	reqID := getRequestID(c)

	var req nextPageRequest
	if err := c.Bind(&req); err != nil {
		c.Logger().Errorj(log.JSON{
			"id":      reqID,
			"message": "failed to declare cursor",
			"error":   err.Error(),
		})

		return err
	}

	// if err := validateRequest(req); err != nil {
	// 	return c.String(http.StatusBadRequest, err.Error())
	// }

	if req.Cursor == "" {
		c.Logger().Errorj(log.JSON{
			"id":      reqID,
			"message": "missing cursor",
		})
		return echo.NewHTTPError(http.StatusBadRequest, "missing cursor to fetch")
	}

	var users []user
	more, err := h.cursor.Fetch(c.Request().Context(), &users, req.Cursor, req.PageSize)
	if err != nil {
		c.Logger().Errorj(log.JSON{
			"id":        reqID,
			"cursor_id": req.Cursor,
			"message":   "failed to fetch",
			"error":     err.Error(),
		})

		return echo.NewHTTPError(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, userResponse{
		CursorID: req.Cursor,
		Done:     !more,
		Users:    users,
	})
}

func getRequestID(c echo.Context) string { return c.Response().Header().Get(echo.HeaderXRequestID) }
