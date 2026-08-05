# Runtime Isolation and Safe Upgrade Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix stream ownership, workflow reference integrity, MCP identity normalization, and safe Pi timeout behavior, then deploy one fork revision without losing data.

**Architecture:** Enforce ownership and validation at both client and service boundaries, keep compatibility sanitizers for existing records, and deploy all changed components from one pinned source revision. Each behavioral fix is developed test-first and verified independently before the end-to-end upgrade.

**Tech Stack:** React/TypeScript/Zustand, Java/Spring, Python/aiohttp, Node.js/TypeScript/Pi SDK, Docker Compose, MySQL, PostgreSQL.

## Global Constraints

- Preserve all existing database data and Docker volumes.
- Never run `docker compose down -v` or delete a database volume.
- Retry a transient model timeout only before any side-effecting tool call.
- Build every changed runtime component from the same fork commit.

---

### Task 1: Request-owned frontend streaming

**Files:**
- Modify: `console/frontend/src/hooks/use-chat.ts`
- Modify: `console/frontend/src/store/chat-store.ts`
- Modify: `console/frontend/src/types/chat.ts`
- Test: `console/frontend/_tests_/chat-store-streaming.test.js`
- Test: `console/frontend/_tests_/chat-stream-guard.test.js`

- [ ] Add a failing test where request A emits after request B starts and assert B is unchanged.
- [ ] Run the focused frontend tests and confirm the late event changes B before the fix.
- [ ] Add message/request-key arguments to stream store mutations, abort the previous controller, and reject callbacks from inactive controllers.
- [ ] Run focused and full frontend tests.
- [ ] Commit the frontend isolation fix.

### Task 2: Workflow reference integrity

**Files:**
- Modify: `console/frontend/src/components/workflow/store/flow-chat-function.ts` or the shared workflow serialization helper selected by code tracing
- Modify: `console/backend/toolkit/src/main/java/com/iflytek/astron/console/toolkit/service/workflow/WorkflowService.java`
- Test: `console/backend/toolkit/src/test/java/com/iflytek/astron/console/toolkit/service/workflow/WorkflowServiceReferenceIntegrityTest.java`
- Test: relevant frontend workflow serialization test

- [ ] Add failing tests for a repairable stale End reference and an ambiguous stale reference.
- [ ] Confirm current serialization retains the deleted node ID.
- [ ] Implement deterministic same-output-name repair and fail closed for ambiguity.
- [ ] Run focused Java/frontend tests and workflow test suites.
- [ ] Commit the reference-integrity fix.

### Task 3: MCP URL and ID normalization

**Files:**
- Modify: `console/backend/toolkit/src/main/java/com/iflytek/astron/console/toolkit/service/workflow/WorkflowService.java`
- Modify: `core/agent/service/builder/base_builder.py`
- Test: `console/backend/toolkit/src/test/java/com/iflytek/astron/console/toolkit/service/workflow/WorkflowServiceMcpRuntimeConfigTest.java`
- Test: `core/agent/tests/test_base_builder.py`

- [ ] Add failing tests proving HTTP(S) values are removed from IDs and retained once in URLs.
- [ ] Run both focused test suites and confirm the erroneous ID survives today.
- [ ] Implement normalization at console serialization and Core Agent input boundaries.
- [ ] Run focused and full component tests.
- [ ] Commit the MCP identity fix.

### Task 4: Safe Pi timeout behavior

**Files:**
- Modify: `core/pi-agent/src/config.ts`
- Modify: `core/pi-agent/src/run-agent.ts`
- Modify: `core/pi-agent/src/model.ts`
- Test: `core/pi-agent/test/run-agent.test.ts`
- Test: `core/pi-agent/test/model.test.ts`
- Modify: `docker/astronAgent/docker-compose.yaml`

- [ ] Add failing tests for one timeout retry before tools and no retry after a tool starts.
- [ ] Confirm the current runner terminates immediately on the first timeout.
- [ ] Pass an explicit configurable timeout to the Pi provider stream and implement the side-effect-aware single retry.
- [ ] Run Pi typecheck, focused tests, and full tests.
- [ ] Commit the timeout fix.

### Task 5: Data-safe deployment and end-to-end verification

**Files:**
- Create: `docker/astronAgent/docker-compose.local-fork.yaml`
- Create: `docker/astronAgent/scripts/backup-before-local-upgrade.sh`
- Create: `docker/astronAgent/scripts/verify-local-upgrade.sh`
- Test: shell syntax checks and Compose config validation

- [ ] Write tests/checks that reject volume-removal commands and require successful dumps before upgrade.
- [ ] Build frontend, Core Agent, Core Workflow, Pi runtime, Console Hub/Toolkit, Link, and table tools from the same commit as required by their changes.
- [ ] Back up MySQL/PostgreSQL and record volume/container inventory.
- [ ] Recreate application containers only with pinned local-fork tags.
- [ ] Repair the current workflow through validated persistence and run the three-department XLSX/SVG scenario.
- [ ] Verify existing row counts, workflows, plugins, downloads, chart rendering, traces, and container health.
- [ ] Commit deployment assets, push `feat/pi-agent-runtime` to `siqiSong/astron-agent`, and update PR #1.
