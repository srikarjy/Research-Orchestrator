package assayos.authz

import future.keywords.in

default allow = false

# Allow all in development
allow {
    input.environment == "development"
}

# Admin users have full access
allow {
    input.actor == "admin"
}

# Service-to-service communication
allow {
    input.actor.startswith("service:")
}

# Read access to health/metrics
allow {
    input.action == "GET"
    input.resource in ["/health", "/metrics", "/api/v1/workflows", "/api/v1/tools"]
}

# Workflow engine access
allow {
    input.resource.startswith("/api/v1/workflows")
    input.actor in data.workflow_users
}

# Biolab MCP access
allow {
    input.resource.startswith("/api/v1/agents")
    input.resource.startswith("/api/v1/tools")
    input.resource.startswith("/api/v1/sandbox")
    input.actor in data.biolab_users
}

# Aletheia access
allow {
    input.resource.startswith("/api/v1/aletheia")
    input.actor in data.aletheia_users
}

# Executor endpoints
allow {
    input.resource.startswith("/api/v1/executor")
    input.actor in data.executor_users
}

# User-role mapping
workflow_users := ["researcher", "pi", "lab_manager", "service:workflow-engine"]
biolab_users := ["researcher", "pi", "lab_manager", "service:biolab-mcp"]
aletheia_users := ["researcher", "pi", "service:aletheia"]
executor_users := ["pi", "lab_manager", "service:workflow-engine"]