# Post-MVP scope

Items documented but not implemented in MVP:

- MCP JSON-RPC ingress (`tools.{name}` → driver operations)
- Driver process pool (`DRIVER_POOL_SIZE`)
- Mem0, LangMem, Obsidian, execution-pool drivers
- Kafka/Redpanda when Redis Streams insufficient
- WASM sandbox for drivers vs subprocess IPC
- User dashboard and real `top_up_url` for 402 responses
- OpenTelemetry distributed traces (Prometheus counters only in MVP)
- PostgreSQL RLS policies
- golang-migrate versioning table
