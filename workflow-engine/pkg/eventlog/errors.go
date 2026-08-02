package eventlog

import "errors"

var (
	ErrEventNotFound       = errors.New("event not found")
	ErrDuplicateDedupKey   = errors.New("duplicate dedup key")
	ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")
)