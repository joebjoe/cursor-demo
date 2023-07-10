package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const numSeedRecords = 10000

func main() {

	connStr, ok := os.LookupEnv("DB_CONNECTION")
	if !ok {
		log.Panic("DB_CONNECTION is not set")
	}

	conn, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Panicf("failed to connect to db: %v", err)
	}

	if err = seedDB(conn); err != nil {
		log.Panic(err)
	}

	log.Print("seed complete")

}

func seedDB(conn *pgxpool.Pool) error {
	query := &strings.Builder{}

	query.WriteString("INSERT INTO users (name) VALUES ")

	args := make([]any, numSeedRecords)
	for i := 0; i < numSeedRecords; i++ {
		if i > 0 {
			query.WriteString(",")
		}

		fmt.Fprintf(query, " ($%d)", i+1)
		args[i] = uuid.NewString()
	}

	ct, err := conn.Exec(context.Background(), query.String(), args...)
	if err != nil {
		return fmt.Errorf("failed to execute statement: %w", err)
	}

	if ct.RowsAffected() != numSeedRecords {
		return fmt.Errorf("command tag returned unexpected result: %#v", ct)
	}

	return nil
}
