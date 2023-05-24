// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.18.0
// source: local_storage.sql

package localdb

import (
	"context"
	"database/sql"
)

const getCurrentRaftIndex = `-- name: GetCurrentRaftIndex :one
SELECT id, term, "index" FROM raft_index LIMIT 1
`

func (q *Queries) GetCurrentRaftIndex(ctx context.Context) (RaftIndex, error) {
	row := q.db.QueryRowContext(ctx, getCurrentRaftIndex)
	var i RaftIndex
	err := row.Scan(&i.ID, &i.Term, &i.Index)
	return i, err
}

const getCurrentWireguardKey = `-- name: GetCurrentWireguardKey :one
SELECT id, private_key, expires_at FROM wireguard_key LIMIT 1
`

func (q *Queries) GetCurrentWireguardKey(ctx context.Context) (WireguardKey, error) {
	row := q.db.QueryRowContext(ctx, getCurrentWireguardKey)
	var i WireguardKey
	err := row.Scan(&i.ID, &i.PrivateKey, &i.ExpiresAt)
	return i, err
}

const setCurrentRaftIndex = `-- name: SetCurrentRaftIndex :exec
INSERT OR REPLACE INTO raft_index (
    id,
    term,
    index
) VALUES (1, ?, ?)
`

type SetCurrentRaftIndexParams struct {
	Term  int64 `json:"term"`
	Index int64 `json:"index"`
}

func (q *Queries) SetCurrentRaftIndex(ctx context.Context, arg SetCurrentRaftIndexParams) error {
	_, err := q.db.ExecContext(ctx, setCurrentRaftIndex, arg.Term, arg.Index)
	return err
}

const setCurrentWireguardKey = `-- name: SetCurrentWireguardKey :exec
INSERT OR REPLACE INTO wireguard_key (
    id, 
    private_key, 
    expires_at
) VALUES (1, ?, ?)
`

type SetCurrentWireguardKeyParams struct {
	PrivateKey string       `json:"private_key"`
	ExpiresAt  sql.NullTime `json:"expires_at"`
}

func (q *Queries) SetCurrentWireguardKey(ctx context.Context, arg SetCurrentWireguardKeyParams) error {
	_, err := q.db.ExecContext(ctx, setCurrentWireguardKey, arg.PrivateKey, arg.ExpiresAt)
	return err
}
