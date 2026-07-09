package postgres

// Store is the Postgres-backed persistence facade. It exposes typed repos that
// satisfy the service port interfaces. All repos issue SQL through the
// sqld-generated query set bound to db.TxDB (the ctx-aware pgtx executor), so a
// repo call made inside the service's tx.Do* runs in that same transaction;
// outside one it runs on the pool (auto-commit).
type Store struct{ db *DB }

// New wraps a *DB. It does not touch the database.
func New(db *DB) *Store { return &Store{db: db} }

// Store returns the typed-repo facade over this connection bundle.
func (db *DB) Store() *Store { return &Store{db: db} }
