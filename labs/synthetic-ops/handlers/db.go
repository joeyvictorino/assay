// Package handlers is a DELIBERATELY VULNERABLE web service used as a known
// ground-truth target for assay in CI. Every planted weakness is marked with
// a "// VULN:" comment and listed in ../ground_truth.json. Never expose it.
package handlers

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go sqlite driver
)

// Open opens (or creates) the SQLite database at dsn. Use ":memory:" for a
// throwaway instance. The pool is pinned to one connection so an in-memory
// database is shared by every request.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// Account is a seeded login. The admin entry is the planted default
// credential and is the only reason it exists.
type Account struct {
	ID       int
	Username string
	Password string
	Email    string
	Role     string
}

// SeedAccounts are the users Seed creates. Exported so lab adapters and
// tests agree on them without re-reading the database.
var SeedAccounts = []Account{
	{1, "admin", "admin123", "admin@synthetic-ops.lab", "admin"}, // VULN: default-creds planted administrator password
	{2, "alice", "Wonderland-1", "alice@synthetic-ops.lab", "user"},
	{3, "bob", "Builder-22", "bob@synthetic-ops.lab", "user"},
}

// Seed creates the schema and seeds users and products. It is idempotent.
func Seed(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			email TEXT NOT NULL,
			role TEXT NOT NULL,
			bio TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			price_cents INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("seed schema: %w", err)
		}
	}
	for _, a := range SeedAccounts {
		if _, err := db.Exec(`INSERT OR IGNORE INTO users (id, username, password, email, role, bio) VALUES (?, ?, ?, ?, ?, ?)`,
			a.ID, a.Username, a.Password, a.Email, a.Role, "Hello, I am "+a.Username+"."); err != nil {
			return fmt.Errorf("seed users: %w", err)
		}
	}
	products := []struct {
		id    int
		name  string
		desc  string
		price int
	}{
		{1, "Widget", "A standard widget.", 1999},
		{2, "Gadget", "A gadget with knobs.", 2999},
		{3, "Gizmo", "A gizmo, batteries not included.", 3999},
		{4, "Doohickey", "Fits most thingamajigs.", 499},
		{5, "Thingamajig", "Pairs with a doohickey.", 599},
	}
	for _, p := range products {
		if _, err := db.Exec(`INSERT OR IGNORE INTO products (id, name, description, price_cents) VALUES (?, ?, ?, ?)`,
			p.id, p.name, p.desc, p.price); err != nil {
			return fmt.Errorf("seed products: %w", err)
		}
	}
	return nil
}
