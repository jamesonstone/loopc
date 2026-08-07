package journal

// schemaStatements build the ledger. They are idempotent, and migrations may
// only add: a recorded cycle cannot be recomputed from a fresh observation, so
// no migration may destroy one.
var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS records (
		sequence    INTEGER PRIMARY KEY AUTOINCREMENT,
		kind        TEXT NOT NULL,
		at          TEXT NOT NULL,
		cycle_id    TEXT NOT NULL DEFAULT '',
		generation  TEXT NOT NULL DEFAULT '',
		mode        TEXT NOT NULL DEFAULT '',
		policy_hash TEXT NOT NULL DEFAULT '',
		payload     TEXT NOT NULL
	)`,

	`CREATE INDEX IF NOT EXISTS records_kind ON records (kind)`,
	`CREATE INDEX IF NOT EXISTS records_cycle ON records (cycle_id)`,
	`CREATE INDEX IF NOT EXISTS records_baseline ON records (kind, policy_hash)`,

	// Append-only is enforced by the database engine, not by this package
	// promising to only ever INSERT. The triggers hold against any writer,
	// including a direct sqlite3 session, so the guarantee does not depend on
	// every future caller being disciplined.
	`CREATE TRIGGER IF NOT EXISTS records_append_only_update
		BEFORE UPDATE ON records
		BEGIN
			SELECT RAISE(ABORT, 'journal is append-only: records may not be updated');
		END`,

	`CREATE TRIGGER IF NOT EXISTS records_append_only_delete
		BEFORE DELETE ON records
		BEGIN
			SELECT RAISE(ABORT, 'journal is append-only: records may not be deleted');
		END`,
}

// pragmas configure durability.
//
// synchronous=FULL is deliberate: in-memory state may only advance after the
// write that persists it commits, so a fast-but-lossy setting would let the
// controller believe it had recorded an action it might not replay after a
// crash.
var pragmas = []string{
	`PRAGMA journal_mode=WAL`,
	`PRAGMA synchronous=FULL`,
	`PRAGMA busy_timeout=5000`,
	`PRAGMA foreign_keys=ON`,
}
