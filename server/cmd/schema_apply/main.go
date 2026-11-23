package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(os.Getenv("DATABASE_URL"), os.Args[1:]); err != nil {
		log.Fatalf("apply schema: %v", err)
	}
}

func run(databaseURL string, paths []string) error {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if len(paths) == 0 {
		return errors.New("at least one SQL file is required")
	}
	statements := make([][]byte, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			return errors.New("SQL file path is empty")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", filepath.Base(path), err)
		}
		if len(strings.TrimSpace(string(raw))) == 0 {
			return fmt.Errorf("SQL file %s is empty", filepath.Base(path))
		}
		statements = append(statements, raw)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse postgres config: %w", err)
	}
	conn, err := pgx.ConnectConfig(ctx, poolConfig.ConnConfig)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer conn.Close(context.Background())
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback(context.Background())
	for idx, statement := range statements {
		if _, err := tx.Exec(ctx, string(statement)); err != nil {
			return fmt.Errorf("execute %s: %w", filepath.Base(paths[idx]), err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}
	return nil
}
