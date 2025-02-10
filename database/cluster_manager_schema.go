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
	clusterManagerTable,
}

func clusterManagerTable(ctx context.Context, tx *sql.Tx) error {
	stmt := `
CREATE TABLE cluster_manager (
    id                       INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
	addresses                TEXT NOT NULL,
	server_cert_fingerprint  TEXT NOT NULL,
	update_interval_seconds  INTEGER NOT NULL
);
`

	_, err := tx.ExecContext(ctx, stmt)

	return err
}
