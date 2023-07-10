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
	// query used when route does not include the :cursor param; this has a static where clause,
	// but no reason why could not be built from query params
	initCursorQuery = `DECLARE _%s SCROLL CURSOR WITH HOLD FOR %s`

	// query to use when :cursor is found
	fetchFromCursorQuery = `FETCH %d FROM _%s;`

	moveForwardAll = `MOVE FORWARD ALL IN _%s;`

	moveBackwardAll = `MOVE ABSOLUTE 0 IN _%s;`
)

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

type Option func(*cursor)

type fetchOp func(context.Context, any, int) (bool, error)

func New(conn *pgxpool.Pool, opts ...Option) Cursor {
	c := &cursor{
		conn: conn,
		ttl:  maxTTL,
		pool: map[string]fetchOp{},
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.ttl == 0 {
		c.ttl = maxTTL
	}

	return c
}

func WithTTL(d time.Duration) func(c *cursor) {
	return func(c *cursor) {
		if d > maxTTL {
			return
		}

		c.ttl = d
	}
}

func (c *cursor) Close() (err error) {
	for cur := range c.pool {
		_, exErr := c.conn.Exec(context.Background(), "CLOSE "+cur)
		err = errors.Join(err, exErr)
	}

	return err
}

func (c *cursor) Declare(ctx context.Context, rawCurID string, sql string, args ...any) error {
	curID := sanitizeCursorID(rawCurID)
	if curID == "" {
		return fmt.Errorf("invalid cursor id %q", rawCurID)
	}

	_, err := c.conn.Exec(ctx, fmt.Sprintf(initCursorQuery, curID, sql), args...)
	if err != nil {
		return fmt.Errorf("failed to declare cursor: %w", err)
	}

	ct, err := c.conn.Exec(ctx, fmt.Sprintf(moveForwardAll, curID))
	if err != nil {
		return fmt.Errorf("failed to determine total for cursor: %w", err)
	}

	totalCount := ct.RowsAffected()

	if _, err = c.conn.Exec(ctx, fmt.Sprintf(moveBackwardAll, curID)); err != nil {
		return fmt.Errorf("failed to reset cursor: %w", err)
	}

	go func() {
		time.Sleep(c.ttl)

		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second*30))
		defer cancel()

		c.conn.Exec(ctx, fmt.Sprintf("CLOSE %s;", curID))
	}()

	c.pool[curID] = func(ctx context.Context, dst any, pgSize int) (bool, error) {
		rows, err := c.conn.Query(ctx, fmt.Sprintf(fetchFromCursorQuery, pgSize, curID))
		if err != nil {
			return false, fmt.Errorf("failed to fetch: %w", err)
		}

		if err = pgxscan.ScanAll(dst, rows); err != nil {
			return false, fmt.Errorf("failed to scan result: %w", err)
		}

		totalCount -= rows.CommandTag().RowsAffected()

		return totalCount > 0, nil
	}

	return nil
}

func (c *cursor) Fetch(ctx context.Context, dst any, rawCurID string, pgSize int) (bool, error) {
	curID := sanitizeCursorID(rawCurID)
	if pgSize == 0 || pgSize > maxPageSize {
		pgSize = maxPageSize
	}

	fetch, found := c.pool[curID]
	if !found {
		return false, fmt.Errorf("no cursor found with id %q", curID)
	}

	more, err := fetch(ctx, dst, pgSize)
	if err != nil {
		delete(c.pool, curID)

		return false, fmt.Errorf("failed to fetch: %w", err)
	}

	return more, nil
}

var sanitizeCursorID = func() func(string) string {
	re := regexp.MustCompile(`(?i)[^a-z0-9]`)

	return func(s string) string { return re.ReplaceAllString(s, "") }
}()
