package eventlog

type Store interface {
	Append(event *Event) error
	GetByWorkflowID(workflowID string) ([]*Event, error)
	GetByDedupKey(dedupKey string) (*Event, error)
	GetByIdempotencyKey(key string) (*Event, error)
	GetLatestEvent(workflowID string) (*Event, error)
	Close() error
}