package store

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestNew(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := New(db)
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.db == nil {
		t.Fatal("Store.db is nil")
	}
}

func TestPing(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := New(db)
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error: %v", err)
	}
}
