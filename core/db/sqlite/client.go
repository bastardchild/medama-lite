package sqlite

import (
	"github.com/go-faster/errors"
	"github.com/jmoiron/sqlx"
	"github.com/medama-io/medama/db"
)

type Client struct {
	*sqlx.DB
}

const dateUpdatedKey = "date_updated"

// Compile time check for Handler.
var _ db.AppClient = (*Client)(nil)

// NewClient returns a new instance of Client with the given configuration.
func NewClient(host string) (*Client, error) {
	db, err := sqlx.Connect("sqlite3", host)
	if err != nil {
		return nil, errors.Wrap(err, "sqlite")
	}

	// Recommended SQLite pragmas for concurrency, data safety, and performance.
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA foreign_keys = ON;",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, errors.Wrap(err, "sqlite: exec pragma")
		}
	}

	// Restrict to a single open connection to prevent in-process database lock contention.
	db.SetMaxOpenConns(1)

	return &Client{
		DB: db,
	}, nil
}

// Close closes the database connection and any prepared statements.
func (c *Client) Close() error {
	return c.DB.Close()
}
