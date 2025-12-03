// Package database provides the database access functions and schema.
package database

import (
	"context"
	"database/sql"

	"github.com/canonical/microcluster/v3/microcluster/db"
)

// SchemaExtensions is a list of schema extensions that can be passed to the MicroCluster daemon.
// Each entry will increase the database schema version by one, and will be applied after internal schema updates.
var SchemaExtensions = []db.Update{
	clusterManagerTables,
}

func clusterManagerTables(ctx context.Context, tx *sql.Tx) error {
	stmt := `
CREATE TABLE cluster_manager (
    id                         INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    addresses                  TEXT NOT NULL,
    certificate_fingerprint    TEXT NOT NULL,
    name                       TEXT NOT NULL,
    status_last_success_time   DATETIME,
    status_last_error_time     DATETIME,
    status_last_error_response TEXT,
    UNIQUE (name)
);
`

	_, err := tx.ExecContext(ctx, stmt)
	if err != nil {
		return err
	}

	stmt = `
CREATE TABLE cluster_manager_config (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
    cluster_manager_id  INTEGER NOT NULL,
    key                TEXT NOT NULL,
    value               TEXT NOT NULL,
    UNIQUE (cluster_manager_id, key),
    FOREIGN KEY (cluster_manager_id) REFERENCES cluster_manager (id) ON DELETE CASCADE
);
`

	_, err = tx.ExecContext(ctx, stmt)

	return err
}
