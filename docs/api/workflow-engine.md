# Workflow Engine API

Base URL: `http://localhost:8080`

## Workflows

### Create Workflow
```http
POST /api/v1/workflows
Content-Type: application/json

{
  "name": "Research Investigation",
  "query": "explain why BRAF V600E reduces binding affinity",
  "steps": []  // Optional: custom steps, otherwise uses demo workflow
}
```

**Response**: `201 Created`
```json
{
  "id": "uuid",
  "name": "Research Investigation",
  "query": "explain why BRAF V600E reduces binding affinity",
  "steps": [...],
  "status": "running",
  "created_at": "2026-01-15T10:30:00Z",
  "updated_at": "2026-01-15T10:30:00Z"
}
```

### List Workflows
```http
GET /api/v1/workflows
```

### Get Workflow
```http
GET /api/v1/workflows/{id}
```

### Execute Workflow
```http
POST /api/v1/workflows/{id}/execute
```

**Response**: `202 Accepted`
```json
{"status": "execution started"}
```

### Get Workflow Events
```http
GET /api/v1/workflows/{id}/events
```

**Response**: Array of `WorkflowEvent`
```json
[
  {
    "id": "uuid",
    "workflow_id": "uuid",
    "step_id": "pubmed",
    "type": "step_completed",
    "payload": {"tool": "PubMed", "output": {...}, "latency_ms": 142, "retries": 0},
    "dedup_key": "sha256...",
    "timestamp": "2026-01-15T10:30:05Z"
  }
]
```

### Workflow WebSocket (Real-time)
```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/workflows/{id}/ws');
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  if (msg.type === 'workflow_state') { ... }
  if (msg.type === 'event') { ... }
};
```

## Executor Endpoints

### Calendar Events
```http
GET /api/v1/executor/calendar
```

### Notifications
```http
GET /api/v1/executor/notifications
```

### Running Tasks
```http
GET /api/v1/executor/tasks
```

### Mark Notification Read
```http
POST /api/v1/executor/notifications/{id}/read
```

## Health
```http
GET /health
```
```json
{"status": "ok", "time": "2026-01-15T10:30:00Z"}
```