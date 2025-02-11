// Package database provides the database access functions and schema.
package database

import (
	"context"
	"database/sql"

	"github.com/canonical/lxd/lxd/db/schema"
)

// SchemaExtensions is a list of schema extensions that can be passed to the MicroCluster daemon.
// Each entry will increase the database schema version by one, and will be applied after internal schema updates.
var SchemaExtensions = []schema.Update{
	sitesTable,
	sitesConfigTable,
}

func sitesTable(ctx context.Context, tx *sql.Tx) error {
	stmt := `
CREATE TABLE sites (
	id          INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
	name        TEXT NOT NULL,
	addresses	TEXT NOT NULL,
	type   	    INTEGER NOT NULL,
	description TEXT NOT NULL,
	UNIQUE(name)
);
  `

	_, err := tx.ExecContext(ctx, stmt)

	return err
}

func sitesConfigTable(ctx context.Context, tx *sql.Tx) error {
	stmt := `
CREATE TABLE sites_config (
	id		INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
	site_id	INTEGER NOT NULL,
	key		TEXT NOT NULL,
	value		TEXT NOT NULL
);
  `

	_, err := tx.ExecContext(ctx, stmt)

	return err
}
