package cursor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sync"
	"time"

	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxTTL = time.Minute * 20
const maxPageSize = 500

const (
	// The query used for initiating the cursor.
	//
	// SCROLL - gives us the ability to go backwards; crucial because the first thing we do is
	// advance to the end in order to get the total row count.
	//
	// WITH HOLD - gives us the ability to allow this cursor to be accessed/live on outside of
	// the initiating transaction.
	initCursorQuery = `DECLARE _%s SCROLL CURSOR WITH HOLD FOR %s`

	fetchFromCursorQuery = `FETCH %d FROM _%s;`

	moveForwardAll = `MOVE FORWARD ALL IN _%s;`

	moveBackwardAll = `MOVE ABSOLUTE 0 IN _%s;`
)

// This could be a separte interface as implemented here, or functionality of a greater Database interface.
type Cursor interface {
	io.Closer
	Declare(ctx context.Context, curID string, sql string, args ...any) error
	Fetch(ctx context.Context, dst any, curID string, pgSize int) (more bool, err error)
}

type cursor struct {
	sync.Mutex
	conn *pgxpool.Pool
	pool map[string]fetchOp
	ttl  time.Duration
}

type fetchOp func(context.Context, any, int) (bool, error)

func New(conn *pgxpool.Pool) Cursor {
	c := &cursor{
		conn: conn,
		ttl:  maxTTL,
		pool: map[string]fetchOp{},
	}

	return c
}

func (c *cursor) Close() (err error) {
	for cur := range c.pool {
		_, exErr := c.conn.Exec(context.Background(), "CLOSE "+cur)
		err = errors.Join(err, exErr)
	}

	return err
}

func (c *cursor) Declare(ctx context.Context, rawCurID string, sql string, args ...any) error {
	defer c.Unlock()
	c.Lock()

	curID := sanitizeCursorID(rawCurID)
	if curID == "" {
		return fmt.Errorf("invalid cursor id %q", rawCurID)
	}

	// create the cursor
	_, err := c.conn.Exec(ctx, fmt.Sprintf(initCursorQuery, curID, sql), args...)
	if err != nil {
		return fmt.Errorf("failed to declare cursor: %w", err)
	}

	// advance to the end and read the total rows that exist for the cursor.
	ct, err := c.conn.Exec(ctx, fmt.Sprintf(moveForwardAll, curID))
	if err != nil {
		return fmt.Errorf("failed to determine total for cursor: %w", err)
	}

	totalCount := ct.RowsAffected()

	// reposition cursor back to the beginning
	if _, err = c.conn.Exec(ctx, fmt.Sprintf(moveBackwardAll, curID)); err != nil {
		return fmt.Errorf("failed to reset cursor: %w", err)
	}

	// set timer for cleaning up the cursor
	go func() {
		defer c.Unlock()
		time.Sleep(c.ttl)

		c.Lock()

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*30))
		defer cancel()

		delete(c.pool, curID)
		c.conn.Exec(ctx, fmt.Sprintf("CLOSE %s;", curID))
	}()

	// add the fetch op to the pool so this cursor is accessible by following requests
	// creating as anonymous function in order scope the `curID` and (most importantly) `totalCount` variables.
	c.pool[curID] = func(ctx context.Context, dst any, pgSize int) (bool, error) {
		rows, err := c.conn.Query(ctx, fmt.Sprintf(fetchFromCursorQuery, pgSize, curID))
		if err != nil {
			return false, fmt.Errorf("failed to fetch: %w", err)
		}

		if err = pgxscan.ScanAll(dst, rows); err != nil {
			return false, fmt.Errorf("failed to scan result: %w", err)
		}

		totalCount -= rows.CommandTag().RowsAffected()

		more := totalCount > 0

		if !more {
			defer c.Unlock()
			c.Lock()

			delete(c.pool, curID)
			c.conn.Exec(ctx, fmt.Sprintf("CLOSE %s;", curID))
		}

		return more, nil
	}

	return nil
}

func (c *cursor) Fetch(ctx context.Context, dst any, rawCurID string, pgSize int) (bool, error) {
	curID := sanitizeCursorID(rawCurID)
	if curID == "" {
		return false, fmt.Errorf("invalid cursor id %q", rawCurID)
	}

	if pgSize == 0 || pgSize > maxPageSize {
		pgSize = maxPageSize
	}

	fetch, found := c.pool[curID]
	if !found {
		return false, fmt.Errorf("no cursor found with id %q", curID)
	}

	more, err := fetch(ctx, dst, pgSize)
	if err != nil {
		return false, fmt.Errorf("failed to fetch: %w", err)
	}

	return more, nil
}

var sanitizeCursorID = func() func(string) string {
	re := regexp.MustCompile(`(?i)[^a-z0-9]`)

	return func(s string) string { return re.ReplaceAllString(s, "") }
}()
