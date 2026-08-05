# Runtime Isolation and Safe Upgrade Design

## Goal

Eliminate cross-run output contamination, reject invalid workflow references, normalize MCP endpoint identity, make transient Pi model timeouts safe, and deploy the repaired fork without losing existing data.

## Confirmed failure modes

1. Chat SSE callbacks update the last message in global state rather than the message owned by that request. A previous PPT run can therefore append output to a newer table request.
2. Workflow `7490763036713099264` contains edges to Agent `d32f...` while its End inputs reference deleted Agent `96d...`.
3. The table MCP URL is present in both `mcpServerIds` and `mcpServerUrls`; Link attempts a database tool lookup for the URL before using it as an endpoint.
4. A later table run failed before its first model token with `Request timed out`. Retrying a model call is safe only before any side-effecting tool call.
5. The running environment mixes locally modified backends with frontend artifacts that are not reproducibly tied to the same source revision.

## Design

### Request-owned streams

Every streamed bot message receives a stable client request key. Store mutations take that key and update only the matching streaming message. Starting a new request aborts the previous controller. Every callback also verifies that its controller is still the active controller before processing legacy text, reasoning, tools, structured events, terminal events, or errors.

### Workflow reference integrity

Before a workflow is saved or sent to the runtime, validate every `ref` input against the current node/output set. A stale reference is repaired only when the target node has exactly one connected upstream node exposing an output with the same name; otherwise validation fails with a user-facing reconnect instruction. Existing workflow `7490763036713099264` will be repaired through the normal save path after backup.

### MCP identity normalization

HTTP(S) values are endpoint URLs, never server IDs. Console serialization removes URLs from `mcpServerIds` and deduplicates them into `mcpServerUrls`. Core Agent repeats this normalization as a backward-compatible boundary so previously stored workflows cannot trigger Link database lookup errors.

### Pi timeout policy

Pi passes an explicit configurable model timeout to the provider SDK. A transient provider timeout may be retried once only when no tool has executed in the run. Once any remote tool starts, no automatic model retry is allowed because Excel, chart, and PPT operations can have side effects. Timeout errors retain their real cause in the streamed error response.

### Reproducible local deployment

All modified application images and the frontend distribution are built from `siqiSong/astron-agent` at one recorded commit. A local Compose override pins those image tags. Before deployment, MySQL and PostgreSQL logical dumps and a Docker-volume inventory are written outside the containers. Deployment recreates application containers only and never removes volumes.

## Verification

- Unit tests reproduce late events from an older request and prove they cannot mutate the newer message.
- Workflow tests cover repairable and ambiguous stale references.
- Console and Core Agent tests prove URL/ID separation for old and new records.
- Pi tests cover one pre-tool timeout retry and zero post-tool retries.
- End-to-end test creates three department rows, generates XLSX and SVG, renders the URLs through the console domain, and confirms the End node completes.
- Post-deployment checks compare database row counts and verify existing workflows/plugins remain accessible.

## Data safety

No `docker compose down -v`, `docker volume rm`, database reinitialization, or destructive migration is permitted. If backup or integrity verification fails, deployment stops before containers are replaced.
