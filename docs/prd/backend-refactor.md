# HaloWebUI Backend Refactor PRD

| Field | Value |
| --- | --- |
| Status | Draft, ready for agent implementation planning |
| Owners | HaloWebUI maintainers |
| Target branch | `codex/backend-refactor` |
| Primary objective | Keep slim below 100 MiB and every standard web image below 200 MiB without breaking existing behavior |
| Target host | Fly.io, 1 shared vCPU, 256 MB allocation, approximately 200 MB usable RAM, 1 GB persistent disk |
| Required language strategy | Entire production backend in Go; no Python interpreter, package, worker, or runtime in the final image |
| Explicitly rejected strategy | One-shot full backend rewrite |

## Executive Summary

HaloWebUI currently runs a broad AI platform inside one FastAPI process. The process assembles authentication, persistence, provider routing, streaming chat, WebSocket state, retrieval, vector storage, document extraction, audio, code execution, MCP, HaloClaw, and static delivery. Optional capabilities are partly lazy at model-construction time, but their routers, SDKs, vector adapter, and retrieval framework are still pulled into the startup import graph.

The refactor will preserve the existing frontend and all externally observable backend behavior while replacing the production backend with Go. The existing Python implementation is a temporary behavioral oracle during migration and is removed from the final runtime and image. The work is deliberately staged:

1. Establish reproducible memory, latency, import, and compatibility baselines.
2. Establish the Go slim control module and preserve the existing request contracts incrementally.
3. Introduce a deep Go capability registry and remote adapters for heavyweight capabilities.
4. Define explicit process roles and installation profiles.
5. Move retrieval, document processing, reranking, and audio outside the constrained Fly.io machine.
6. Make task state, cancellation, streaming, health, and observability process-safe.

Every phase must leave the repository releasable and independently rollbackable. No phase may depend on completing a later phase to restore current behavior.

## Problem Statement

Operators experience HaloWebUI as memory-heavy even when only ordinary remote-model chat is required. The current process loads a large Python dependency graph at startup and retains local model runtimes after first use. Increasing the web worker count can multiply global state and model memory. The existing `slim` deployment removes auxiliary runtimes but does not materially narrow the default Python dependency profile.

The operational symptoms have several distinct causes that must not be conflated:

- Baseline memory from eager application assembly, router imports, Chroma, LangChain, provider SDKs, database libraries, and other optional adapters.
- Peak and retained memory from local embedding, reranking, Whisper, OCR, ONNX, CTranslate2, and Torch implementations.
- Additional process memory when multiple Uvicorn workers each import and initialize their own state.
- Request-scoped peaks from file processing, image payloads, streaming buffers, and concurrent tasks.
- Child-process memory from MCP, browser automation, code execution, and optional runtimes.
- Virtual mappings and database caches that may look large but must be distinguished from proportional resident memory.

The codebase is too large and behaviorally rich for a one-shot replacement. The required full Go rewrite must therefore proceed as vertical contract-compatible slices, each independently tested against the current implementation before the Python runtime is removed.

The underlying product problem includes both the Python runtime footprint and insufficiently explicit capability ownership, process roles, and runtime state. The Go design must solve both rather than translating the same monolith line for line.

## Solution

HaloWebUI will become a modular backend with explicit capability seams and independently deployable process roles while retaining the current external interface.

The web/control process uses Go and the standard HTTP stack. It owns external request compatibility, authentication, authorization, persistence orchestration, model/provider selection, streaming coordination, frontend delivery, and feature configuration.

Heavy capabilities move behind remote-provider and Go worker interfaces. Retrieval, document extraction, embedding, reranking, speech recognition, OCR, and browser automation use remote adapters on the constrained deployment. Any local implementation must be written in Go, independently observable, and kept outside the constrained standard image.

The existing Python process may run only in migration CI as a differential oracle. It is not a production fallback. The same contract suite exercises both implementations until each Go slice reaches parity.

The Go module is the only final backend. It owns HTTP, authentication, bounded stream forwarding, static frontend delivery, persistence, integrations, task lifecycle, and capability coordination. Go replaces routes vertically under differential contract tests rather than duplicating the monolith in one change.

## Target Deployment Envelope

The primary acceptance environment is one Fly.io machine with:

- One shared vCPU.
- 256 MB configured memory and approximately 200 MB usable memory.
- One GB persistent disk.
- No swap assumption.
- Remote model providers and remote heavyweight capabilities.
- One application instance by default.

The deployment must reserve headroom for the kernel, Fly runtime, TLS/network buffers, allocator peaks, SQLite, and short-lived request data. Therefore:

- `slim` has a hard 100 MiB cgroup-memory ceiling and a 90 MiB warning threshold.
- Standard web/control has a hard 200 MiB ceiling and a 175 MiB warning threshold.
- The slim runtime image target is at most 250 MiB uncompressed and 100 MiB compressed.
- A fresh writable application volume, excluding user uploads, is at most 100 MiB.
- Logs are written to stdout/stderr, are externally collected, and are not retained without a strict rotation quota.
- SQLite write-ahead logs, temporary files, caches, and generated artifacts have explicit quotas and cleanup.
- At least 300 MiB of the one GB disk remains available after image deployment and fresh application initialization so an upgrade or rollback can be staged safely.
- Local embedding, reranking, Whisper, OCR, Playwright browsers, Pyodide assets, bundled model caches, and stdio MCP runtimes are not part of the constrained slim deployment.

## Goals And Success Metrics

### Product Goals

- Preserve all current user-visible behavior and frontend compatibility.
- Allow a remote-model-only deployment to run without local ML, OCR, browser, or vector implementations loaded in the web/control process.
- Give operators clear deployment profiles for control-only, retrieval, document, audio, and full compatibility workloads.
- Isolate model and document memory so workers can be limited, restarted, scaled, or disabled independently.
- Retain support for local and remote capability adapters.
- Make failures in optional capabilities degrade locally rather than destabilizing the entire web/control process.
- Produce sufficient observability to attribute memory to a process role, capability, model, and workload.

### Required Quantitative Gates

The memory limits are absolute release gates. Relative measurements remain useful for diagnosing regressions, but cannot replace the limits.

1. The released `slim` container must remain below **100 MiB** total cgroup memory during readiness, a five-minute steady idle window, health/config polling, login, and ordinary remote-model streaming chat.
2. Every other released web/control container must remain below **200 MiB** total cgroup memory during the same scenarios.
3. A web/control container must remain below its limit while a separate capability worker loads a local embedding, reranking, document, or speech runtime.
4. Capability worker memory, model-server memory, browser memory, MCP child-process memory, and other child-process memory must be recorded as part of the end-to-end deployment total. A release may not hide memory by moving it outside the measured process tree.
5. Any shipped local-capability profile that cannot stay below 200 MiB while executing its declared default workload must be changed to remote execution, on-demand isolated execution with a verified post-job return below 200 MiB, or explicitly removed from the standard image set. It cannot be reported as passing based on idle memory alone.
6. Loading a local capability in a worker must increase control-process proportional memory by no more than 10%.
7. A worker restart after a simulated out-of-memory termination must not terminate authenticated web sessions or the control process.
8. Existing request-contract tests must pass against the Go implementation and the migration oracle.
9. Ordinary remote-model chat time-to-first-token p95 must regress by no more than 10%.
10. Non-streaming request latency p95 must regress by no more than 10% for control-only endpoints.
11. Streaming buffers must have explicit per-request limits and demonstrate stable memory during a 30-minute concurrency test.
12. No capability may cause unbounded queue growth; admission control must reject or defer work predictably.
13. Startup must report the selected process role, loaded capabilities, unavailable optional dependencies, and installation profile.
14. A rollback to the previous Go image must require configuration or image selection only and no data reversal.

### Measurement Rules

- Record container memory as well as per-process RSS, PSS, private dirty memory, and mapped memory.
- Use binary mebibytes for gates: one MiB equals 1,048,576 bytes.
- Evaluate the maximum sampled `memory.current` during the required window, not only the final or average sample.
- Use proportional set size as the primary comparison when shared libraries are involved.
- Measure cold startup, warm idle, first use, steady state, and post-cancellation state separately.
- Separate control process, capability worker, child process, database, browser, and model runtime memory.
- Do not treat image size, package size, virtual address space, or a configured mmap size as resident memory.
- Run each scenario at least five times after one discarded warm-up and report median and p95.
- Run gates with a fixed container memory limit, CPU allocation, architecture, kernel family, and writable-volume state.

## User Stories

1. As an operator, I want remote-model-only chat to avoid loading local ML runtimes, so that a small server can run HaloWebUI reliably.
2. As an operator, I want each capability to report its memory separately, so that I can identify what consumes resources.
3. As an operator, I want to disable retrieval completely, so that retrieval dependencies do not enter the control process.
4. As an operator, I want to use a remote embedding adapter, so that local model memory is unnecessary.
5. As an operator, I want to use a local embedding worker, so that data can remain on my infrastructure.
6. As an operator, I want local reranking to run separately from chat, so that its model lifecycle does not affect ordinary requests.
7. As an operator, I want speech recognition to run in a dedicated worker, so that Whisper memory can be bounded and restarted.
8. As an operator, I want document extraction to run independently, so that large files cannot exhaust the control process.
9. As an operator, I want explicit installation profiles, so that the deployed image contains only the dependencies required by its role.
10. As an operator, I want startup validation for profile and capability mismatches, so that missing dependencies fail clearly.
11. As an operator, I want one command to run a full local development stack, so that process isolation does not damage developer ergonomics.
12. As an operator, I want worker health and readiness checks, so that orchestration only sends jobs to usable workers.
13. As an operator, I want configurable worker concurrency, so that memory use can be matched to available hardware.
14. As an operator, I want configurable queue limits, so that overload produces controlled errors rather than an out-of-memory termination.
15. As an operator, I want configurable job deadlines, so that stalled model or document jobs are reclaimed.
16. As an operator, I want idle model eviction to be optional, so that I can trade latency for memory.
17. As an operator, I want a worker to restart without logging users out, so that capability failure is isolated.
18. As an operator, I want immutable previous Go images, so that I can roll back immediately without starting Python.
19. As an operator, I want current SQLite deployments to continue working with one control worker, so that upgrades do not force a database migration.
20. As an operator, I want clear guidance before enabling multiple control workers, so that unsupported SQLite and global-state combinations are prevented.
21. As an operator, I want metrics for loaded models and active jobs, so that I can correlate memory changes with workload.
22. As an operator, I want import-time diagnostics, so that newly introduced eager dependencies are detected before release.
23. As an administrator, I want all existing settings to retain their meaning, so that an upgrade does not silently change behavior.
24. As an administrator, I want the current model, provider, storage, vector, and audio choices to map to explicit adapters, so that configuration remains understandable.
25. As an administrator, I want unavailable capabilities to include actionable dependency and profile messages, so that failures can be corrected without reading source code.
26. As an administrator, I want worker configuration changes to be validated before activation, so that bad endpoints do not break all requests.
27. As an administrator, I want secrets to stay in the process that needs them, so that capability isolation does not spread credentials.
28. As an administrator, I want worker requests to carry a trusted authorization context, so that user and group permissions remain enforced.
29. As an administrator, I want audit records to preserve actor and capability information, so that isolated execution remains traceable.
30. As an administrator, I want capability status visible through existing administration surfaces, so that I can diagnose deployment state.
31. As an end user, I want existing login, chat, knowledge, audio, file, model, and settings flows to behave identically, so that the refactor is invisible.
32. As an end user, I want streaming responses to preserve event order and content, so that chat rendering remains correct.
33. As an end user, I want cancellation to stop both the visible request and underlying worker job, so that resources are not wasted.
34. As an end user, I want file-processing progress and errors to remain understandable, so that process isolation does not hide failures.
35. As an end user, I want retries to avoid creating duplicate embeddings or duplicate stored content, so that transient failures are safe.
36. As an end user, I want a failed optional capability to leave ordinary chat usable, so that one feature cannot take down the platform.
37. As an end user, I want current conversation and attachment data to survive an upgrade and rollback, so that no content is lost.
38. As a developer, I want one capability registry, so that adding a capability does not require editing many unrelated modules.
39. As a developer, I want optional imports to occur inside capability implementations, so that dependency ownership is local.
40. As a developer, I want capability contracts independent of FastAPI request objects, so that implementations can run in another process.
41. As a developer, I want worker and in-process adapters to satisfy the same interface, so that parity is testable.
42. As a developer, I want request and stream contracts captured as fixtures, so that refactors cannot silently alter frontend behavior.
43. As a developer, I want typed configuration with validation, so that invalid process roles and adapter combinations fail at startup.
44. As a developer, I want explicit job lifecycle states, so that completion, failure, timeout, cancellation, and retry behavior are deterministic.
45. As a developer, I want bounded streaming and file buffers, so that concurrency does not create hidden memory growth.
46. As a developer, I want structured errors across the worker seam, so that callers do not parse log strings.
47. As a developer, I want deterministic cleanup hooks, so that temporary files, tasks, clients, and model references are released.
48. As a developer, I want architecture tests to prevent heavy imports in the control role, so that the memory improvement does not regress.
49. As a developer, I want fault-injection tests, so that worker crashes, partial streams, timeouts, and disconnects are verified.
50. As a developer, I want migration steps to be small and independently reviewable, so that regressions can be localized.
51. As a release manager, I want each phase to have explicit entry and exit gates, so that incomplete architecture does not ship accidentally.
52. As a release manager, I want deployment manifests for mixed-version rollout, so that upgrades can be canaried.
53. As a release manager, I want compatibility tests against the previous release, so that persisted data and external clients remain supported.
54. As a security reviewer, I want internal worker authentication and replay protection, so that the new process seam cannot be abused.
55. As a security reviewer, I want file references validated against approved storage roots, so that workers cannot read arbitrary host paths.
56. As a security reviewer, I want sensitive values redacted from logs and metrics, so that observability does not leak credentials or user content.
57. As an AI implementation agent, I want requirement identifiers, dependency order, test gates, and rollback criteria, so that I can execute work without inventing architecture.
58. As an AI review agent, I want every implementation pull request to cite covered requirements and evidence, so that completeness is mechanically reviewable.

## Implementation Decisions

### 1. Refactor Strategy

- Use a strangler-style migration inside the existing product rather than a separate replacement application.
- Preserve the external interface throughout the migration.
- Replace every production Python route, task, migration, and integration with Go before release.
- Use remote providers or Go-native implementations for embedding, reranking, speech recognition, document extraction, and related runtimes.
- Do not ship a Python interpreter, wheel, source package, virtual environment, or Python child process.
- Do not introduce a third backend language in this program.
- Every new seam must have at least two real adapters during migration: the Go adapter and the migration-oracle adapter. Avoid hypothetical abstraction.

### 2. External Compatibility Contract

The following are immutable unless a separate product change is approved:

- Existing HTTP methods, paths, query parameters, form fields, multipart behavior, response status codes, and response bodies.
- Existing authentication cookies, bearer tokens, API keys, authorization rules, and trusted-header behavior.
- Existing Server-Sent Events framing, event ordering, termination markers, keepalive behavior, and error propagation.
- Existing WebSocket namespace, event names, room semantics, reconnection behavior, and usage tracking.
- Existing frontend configuration payloads and feature flags.
- Existing model/provider identifiers and selection behavior.
- Existing upload, cache, static, and generated-file URL behavior.
- Existing database records, identifiers, timestamps, ownership, access-control semantics, and migrations.
- Existing plugin/function behavior while the migration oracle remains available in CI.
- Existing HaloClaw and external gateway behavior.

Before implementation changes, the agent must capture representative golden contracts for all critical routes and streams. Compatibility is judged by externally observable behavior, not by internal class or function equality.

### 3. Process Roles

The runtime will support explicit roles:

- **Control role:** Go authentication, authorization, request compatibility, configuration, persistence, provider routing, chat orchestration, WebSocket coordination, static delivery, and worker coordination.
- **Retrieval role:** Go vector adapters, chunking, hybrid search, and remote embedding/reranking coordination.
- **Document role:** Go text extraction, metadata normalization, temporary-file lifecycle, and remote OCR adapters.
- **Audio role:** Go remote speech and text-to-speech adapters.
- **Migration oracle role:** the old implementation, permitted only in CI until differential tests pass and never included in a release image.

A deployment may combine retrieval and document workers initially when operational simplicity is more important than fine-grained scaling. The interface and metrics must still identify the active capability.

### 4. Capability Registry

- Introduce one authoritative registry for backend capabilities.
- A capability declaration contains its stable name, availability, selected adapter, required configuration, dependency profile, health state, and whether it supports streaming, cancellation, and retries.
- Registry discovery must not import heavyweight implementations.
- Capability implementations load only after configuration selects them and a caller requests them.
- Availability checks must use package metadata or lightweight module discovery and must not import the optional package merely to test for existence.
- The control role must be able to start when an unused optional dependency is absent.
- An enabled but unavailable capability must fail startup or readiness according to configuration, with an actionable structured message.
- Registry state must be inspectable through administration and diagnostics without exposing secrets.

### 5. Application Assembly

- Separate application assembly from capability implementation.
- Router registration, configuration binding, lifecycle hooks, and background tasks must be registered through feature modules.
- Disabled features must not register background tasks or initialize clients.
- Startup database migration must be a distinct, observable phase with an explicit lock and failure policy.
- Importing a module for tests or tooling must not unexpectedly execute migrations, create clients, download models, create directories, or start tasks.
- Global application state must be catalogued. Each entry must be classified as immutable configuration, cache, client, runtime model, task state, or compatibility state.
- Runtime models and capability clients must move behind their owning module rather than remain anonymous global state.

### 6. Dependency And Image Profiles

Define dependency profiles that correspond to process roles rather than marketing labels:

- `control`: the Go binary and frontend only; excludes local model, OCR, browser, and unused vector implementations.
- `retrieval-worker`: Go-native retrieval and remote embedding/reranking adapters.
- `document-worker`: Go-native parsers and remote OCR adapters.
- `audio-worker`: Go remote speech adapters.
- `full`: one Go binary with all Go capability adapters enabled.
- `dev-test`: test, formatting, and diagnostic tooling only.

Profile rules:

- The control profile must not transitively install Torch, sentence-transformers, faster-whisper, local OCR, or browser binaries.
- Vector adapters beyond the chosen default must be optional.
- Cloud storage and telemetry dependencies must be separately selectable.
- Image build checks must verify that role entrypoints start with only their declared profile.
- The existing `slim` concept may remain, but documentation must distinguish image toolchain size from process memory.
- Lock files must remain reproducible across supported platforms.

### 7. Worker Interface

The worker interface is transport-neutral at the domain level. The first adapter will use authenticated internal HTTP with JSON request metadata and newline-delimited JSON for progress or streamed results. Large inputs must be passed as approved storage references or streamed bodies rather than embedded repeatedly in JSON.

Required operations:

- Return identity, version, supported capabilities, loaded runtimes, and limits.
- Return liveness and readiness separately.
- Submit a job with actor context, capability, operation, input reference, options, deadline, idempotency key, and trace context.
- Observe job status and progress.
- Stream result events in order.
- Cancel a job idempotently.
- Retrieve a terminal result for reconnect or recovery when permitted.
- Report structured capability errors.

The initial transport is an adapter, not the domain interface. A later Go or message-queue adapter must not require capability implementations to change.

### 8. Job Lifecycle

All cross-process work uses the following conceptual states:

- `accepted`
- `queued`
- `running`
- `streaming`
- `completed`
- `failed`
- `cancelled`
- `timed_out`
- `expired`

Lifecycle rules:

- State transitions are monotonic and terminal states cannot transition again.
- Cancellation is idempotent and propagates to child tasks and model calls when supported.
- Client disconnect triggers the configured cancellation policy.
- Every job has an absolute deadline in addition to transport timeouts.
- Retries require an idempotency key and operation-specific safety declaration.
- Document indexing must not create duplicate vectors after retry.
- Temporary resources are owned by the job and cleaned after terminal state or expiry.
- Terminal metadata records capability, adapter, duration, attempt, worker identity, and sanitized error classification.

Durable job storage is additive and may be introduced when reconnect or restart recovery is required. Initial synchronous jobs may remain ephemeral if they preserve current behavior and cannot outlive their request.

### 9. Streaming And Backpressure

- Streaming must use a bounded incremental decoder; concatenating unbounded raw input is prohibited.
- Define maximum event, line, buffered, and cumulative payload sizes per operation.
- Preserve provider event ordering and current frontend framing.
- Backpressure must propagate from browser to control module to worker or provider adapter.
- Slow consumers must not cause unbounded memory retention.
- Disconnect must release queues, callbacks, temporary buffers, and underlying network responses.
- Base64 images and other large events require explicit size policies and tests.
- Partial worker streams must terminate with a structured error event compatible with current callers.

### 10. Model Runtime Lifecycle

- A worker owns model objects; the control role never owns local model objects.
- Model loading is serialized per model identity to prevent duplicate concurrent loads.
- Runtime identity includes model, revision, device, quantization or compute type, and trust settings.
- Workers expose whether a model is unloaded, loading, ready, busy, evicting, or failed.
- A failed load must release partial resources before retry.
- Optional idle eviction must consider active references and a configurable minimum residency period.
- Worker concurrency defaults must be conservative and model-aware.
- GPU workers must report device allocation and must not silently fall back to CPU unless explicitly configured.
- Model downloads must use an explicit cache, lock, timeout, and offline policy.

### 11. Retrieval And Vector Adapters

- Vector storage is a real seam with separate adapters for local and remote implementations.
- Importing configuration must not import or construct a vector client.
- A vector adapter is initialized only when retrieval is enabled and selected.
- Retrieval orchestration owns chunking, embedding, query, hybrid search, reranking, and result normalization behind one deep interface.
- Web search adapters and document loaders load only when selected.
- Retrieval results must preserve current identifiers, metadata sanitization, score interpretation, ordering, and access control.
- Changing the embedding model continues to require explicit re-embedding behavior; the refactor must not hide vector incompatibility.

### 12. Document Processing

- Document workers receive validated storage references and metadata, not arbitrary host paths.
- Extraction providers remain adapters with common normalized document output.
- Large documents are streamed or processed incrementally where supported.
- Temporary files have quotas, unique ownership, and deterministic cleanup.
- Remote extraction providers use bounded timeouts, polling limits, and sanitized errors.
- Local fallback behavior remains compatible and must be visible in result metadata.
- Extraction and indexing are separate phases so failures and retries are attributable.

### 13. Audio Processing

- Local speech recognition runs only in the audio worker.
- Remote speech and text-to-speech adapters may remain in the control role if they do not pull local runtime dependencies; otherwise they use the worker interface.
- Input-size limits, conversion limits, temporary-file cleanup, and ffmpeg availability are validated before model invocation.
- Current accepted formats, output formats, language behavior, voice selection, and errors remain compatible.
- The worker exposes loaded speech model identity and memory state.

### 14. Persistence And Migration

- The first two phases do not replace the database technology or change existing record semantics.
- Current SQLite support remains single-control-worker unless a separate concurrency design is approved.
- Multi-control-worker deployments require an external database and external coordination for shared caches, tasks, and WebSocket state.
- Import-time migration side effects must move into an explicit startup or command phase.
- Any new tables are additive, nullable or independently removable, and readable by the previous release when possible.
- Schema migration, data backfill, feature activation, and cleanup occur in separate releases.
- The Go adapter must not require data reversal during rollback.
- Database access in capability workers is minimized. Prefer passing immutable job inputs and using owning interfaces rather than sharing ORM globals.

### 15. Configuration

- Environment values, persisted administrative settings, and defaults have one documented precedence order.
- Existing setting names retain compatibility; new names may be aliases during migration.
- Process role, adapter selection, worker endpoint, concurrency, queue, timeout, memory, and eviction settings are typed and validated.
- Invalid combinations fail before readiness, not on first user request.
- Secrets are represented as references or injected only into the adapter that needs them.
- Diagnostics redact secrets and user content.
- Configuration changes that require worker restart report that fact explicitly.

### 16. Authentication And Internal Trust

- External authentication remains in the control role.
- Internal worker calls use mutually authenticated credentials or a signed short-lived internal token.
- Actor context includes stable user identity, role, groups or scopes, request identity, and trace identity, but excludes unnecessary profile data.
- Workers trust only the control issuer and reject expired, replayed, or capability-mismatched requests.
- Authorization is enforced before job submission and, for sensitive operations, revalidated by the owning capability.
- Internal endpoints are not exposed publicly by default.
- File and URL inputs undergo the same SSRF, path, and content validation as current behavior.

### 17. Dynamic Functions, MCP, And Code Execution

- Dynamic functions migrate to a Go sandbox seam using a supported WASM or Starlark-compatible format; legacy Python source is reported as requiring migration and is never executed in production.
- The refactor must inventory whether each function requires control state, user state, model routing, storage, or network access.
- MCP child processes, browser automation, and code execution are measured separately from the control process.
- Child-process managers enforce concurrency, idle reaping, cancellation, output limits, and sanitized environment construction.
- Moving these capabilities to workers requires a separate security review and compatibility gate.

### 18. HaloClaw And External Gateway

- HaloClaw adapters currently depend on application model state, users, functions, and chat orchestration; they are not treated as independently migratable until those dependencies cross explicit interfaces.
- The external gateway preserves model visibility, tool policy, audit behavior, and stream compatibility.
- Go owns these paths after golden contract tests exist.
- Provider, authorization, model selection, and persistence authority remain in one Go control module during the migration.
- No shared database writes from Go are permitted in the first Go milestone.

### 19. Observability

Required structured telemetry includes:

- Process role, version, profile, capability, adapter, and worker identity.
- Startup phase durations and import-time summaries.
- RSS, PSS where available, private dirty memory, container memory, and worker child-process memory.
- Loaded runtime identity, load duration, last-used time, active references, and eviction count.
- Job queue depth, admission rejection, active jobs, duration, cancellation, timeout, retry, and failure classification.
- HTTP and stream latency, time to first token or progress event, bytes transferred, and disconnect count.
- Worker availability, restarts, crash reason, and readiness transitions.
- Cache hit rate and bounded cache size.

Metrics must avoid labels with unbounded user, chat, model, file, or request cardinality. Logs must be structured and correlated by trace and job identifiers without exposing secrets or user content.

### 20. Go Completion Gate

The full Go implementation may be released only when all of the following are true:

- Every external route has a Go handler or an explicit structured unsupported response approved by product scope.
- Control and capability memory and latency baselines have been remeasured.
- The target path has complete golden request, stream, authentication, and failure contracts.
- The final image contains no Python executable, source, package, or child process.
- An architecture decision records ownership, on-call implications, build/release changes, dependency policy, and rollback.

The first Go milestone implements health, bounded reverse proxying, rate limiting, static delivery, and stream forwarding. It runs beside the migration oracle in CI and is removable by routing configuration until the complete Go route set is ready.

## Functional Requirements

### Capability Loading

- **FR-001:** The control role shall start successfully without local retrieval, audio, OCR, browser, or alternate-vector packages installed when those capabilities are disabled.
- **FR-002:** Enabling an unavailable capability shall return an actionable startup or readiness error.
- **FR-003:** Capability discovery shall not load the capability implementation.
- **FR-004:** Loaded capabilities and adapters shall be visible through diagnostics.
- **FR-005:** Capability reset shall release owned clients and runtime references where the underlying library permits it.

### Worker Execution

- **FR-010:** The control role shall submit, observe, stream, cancel, and time out worker jobs.
- **FR-011:** Workers shall enforce queue and concurrency limits.
- **FR-012:** Workers shall reject unsupported capability or operation combinations with structured errors.
- **FR-013:** Worker restart shall not corrupt control state or persisted user data.
- **FR-014:** Retriable jobs shall use idempotency protection.
- **FR-015:** Worker health shall distinguish liveness from readiness.

### Compatibility

- **FR-020:** Current frontend flows shall operate without frontend changes during the initial milestones.
- **FR-021:** Current HTTP, SSE, and WebSocket contracts shall remain compatible.
- **FR-022:** Current authentication and authorization behavior shall remain compatible.
- **FR-023:** Current persisted configuration shall be migrated or aliased without silent reset.
- **FR-024:** Compatibility mode shall restore in-process execution through configuration.

### Resource Control

- **FR-030:** Every streaming path shall use bounded buffers.
- **FR-031:** Every job shall have a deadline and cleanup policy.
- **FR-032:** Every worker shall expose configurable concurrency and queue depth.
- **FR-033:** Local model runtimes shall be owned by workers only in worker mode.
- **FR-034:** Temporary files shall have quotas and deterministic cleanup.
- **FR-035:** Child processes shall have output, idle, and lifecycle limits.

### Operations

- **FR-040:** Supported process roles shall have build and deployment definitions.
- **FR-041:** Role/profile mismatches shall fail before readiness.
- **FR-042:** Metrics shall attribute memory and jobs to process role and capability.
- **FR-043:** Deployments shall support mixed compatibility and worker adapters during rollout.
- **FR-044:** Rollback shall not require reversing user data.

## Non-Functional Requirements

- **NFR-001 Reliability:** Optional capability failure must not crash the control process.
- **NFR-002 Memory:** Meet the quantitative memory gates defined above.
- **NFR-003 Performance:** Meet latency and time-to-first-token gates.
- **NFR-004 Security:** Internal workers must authenticate calls, validate references, and redact sensitive telemetry.
- **NFR-005 Maintainability:** A new capability must register through one module and have one contract-test surface.
- **NFR-006 Testability:** Compatibility and worker adapters must be testable with the same behavior suite.
- **NFR-007 Portability:** Preserve supported CPU architectures and clearly declare capability-specific platform limitations.
- **NFR-008 Reproducibility:** All dependency profiles and images must build from locked inputs.
- **NFR-009 Observability:** Every cross-process request must carry trace and job correlation.
- **NFR-010 Rollback:** Every release phase must support configuration or image rollback without destructive migration.

## Testing Decisions

### Testing Philosophy

- Test externally observable behavior at each module interface.
- Do not assert private function structure, import order beyond architecture constraints, or incidental implementation details.
- Use the same behavior suite for compatibility and worker adapters.
- Treat the current application as the behavioral oracle only after ambiguous or buggy behavior has been reviewed; do not fossilize known security or correctness defects silently.
- Every bug found during refactoring receives a failing regression test before the fix.
- Performance and memory gates are tests with controlled environments, not informal observations.

### Required Test Suites

#### Contract Tests

- Authentication, authorization, cookies, bearer tokens, API keys, and trusted headers.
- Model listing, provider selection, provider errors, and configuration updates.
- Chat completion request and response payloads.
- SSE event order, termination, error frames, keepalives, and disconnect behavior.
- WebSocket connection, room, event, reconnection, and cancellation behavior.
- File upload, retrieval, knowledge, audio, image, and generated-file behavior.
- Administration settings and capability diagnostics.
- HaloClaw and external gateway representative flows.

#### Capability Behavior Tests

- Capability absent, disabled, enabled, misconfigured, and healthy states.
- Lazy load occurs only on first use.
- Concurrent first use constructs one runtime per identity.
- Reset and configuration change release or replace runtime ownership correctly.
- Local and remote adapter result normalization.
- Go adapter and migration-oracle parity.

#### Worker Protocol Tests

- Authentication success, expiry, replay rejection, and wrong capability.
- Job submission, progress, streaming, completion, and terminal result retrieval.
- Cancellation before queue, during execution, and during streaming.
- Deadline exceeded and transport timeout distinction.
- Idempotent retry and duplicate submission.
- Worker restart, partial response, corrupt event, unavailable dependency, and overload.
- Maximum event size, buffer size, file size, queue depth, and concurrency.

#### Data Tests

- Upgrade from representative existing SQLite and PostgreSQL datasets.
- Rollback while new additive metadata exists.
- Existing users, chats, files, knowledge, model settings, access control, and external integrations.
- Duplicate-prevention for document extraction and vector indexing retries.
- Migration lock behavior under simultaneous startup.

#### Security Tests

- Worker endpoint exposure and authentication.
- Path traversal, unapproved file references, SSRF, malformed URLs, and oversized payloads.
- Authorization context tampering.
- Secret and content redaction in logs, traces, errors, and metrics.
- Child-process environment allowlisting and output limits.
- Dynamic-function and MCP behavior remains no less isolated than before.

#### Performance And Memory Tests

Run the following scenarios for control, compatibility, and worker configurations:

1. Cold startup to liveness.
2. Cold startup to readiness.
3. Ten-minute idle after readiness.
4. First ordinary remote-model chat.
5. Repeated ordinary remote-model chat.
6. First local embedding query.
7. Repeated local embedding query.
8. First local reranking query.
9. First local speech recognition request.
10. Representative small and large document extraction.
11. Concurrent large SSE or image-bearing streams.
12. Cancellation during each heavy operation.
13. Thirty-minute steady concurrency.
14. Worker crash and recovery.
15. One versus two control workers where supported.
16. Local versus remote vector adapter.

Collect process and container memory, latency, throughput, event bytes, task counts, open descriptors, temporary disk, and post-test cleanup state.

### Architecture Tests

- Starting or importing the control role must not load a denylist of heavy optional packages.
- Capability declarations must not import their implementations.
- Control modules must not import concrete local model implementations.
- Worker domain objects must not depend on FastAPI request objects.
- Cross-process jobs must not carry arbitrary local paths.
- Streaming implementations must declare and enforce buffer limits.
- New global state requires an ownership classification and cleanup hook.

### Prior Art In The Repository

The existing repository already contains unit tests for model caching, Chroma behavior, retrieval metadata, streaming errors, MCP behavior, data management, provider health checks, image settings, and frontend stream parsing. Agents should extend these behavior patterns and add cross-adapter contract suites rather than duplicating tests around internal functions.

### Test Modules Required By Phase

- Phase 0: measurement harness and golden external contracts.
- Phase 1: capability registry, lazy import, application assembly, and dependency-profile tests.
- Phase 2: Go persistence, authentication, job lifecycle, cancellation, fault injection, and adapter parity tests.
- Phase 3: Go retrieval, vector, document, audio, and remote-provider behavior suites.
- Phase 4: Go administration, tools, MCP, HaloClaw, gateway, deployment, migration, rollback, load, and soak tests.
- Phase 5: final image deletion checks, differential stream tests, and failure/rollback tests.

## Delivery Plan

### Phase 0: Baseline And Contract Capture

**Deliverables**

- Reproducible benchmark harness and documented environment manifest.
- Baseline matrix for control idle, ordinary chat, retrieval, document, audio, concurrent streaming, and multiple workers.
- Import-time report and process tree attribution.
- Golden HTTP, SSE, WebSocket, authentication, and representative persistence contracts.
- Initial risk register and architecture decisions for process roles and transport.

**Exit gate**

- Baselines run in CI-compatible automation or a documented benchmark environment.
- Results separate control, worker candidate, child process, and database memory.
- Critical external contracts are executable tests.

### Phase 1: Go Control Foundation

**Deliverables**

- Go application assembly and capability registry.
- Static frontend delivery, health, version, and configuration contracts.
- Go SQLite adapter and additive migrations.
- Go authentication, sessions, and authorization context.
- Go control dependency profile and final-image deletion checks.

**Exit gate**

- Go control role starts without Python or heavyweight local runtimes.
- Existing behavior tests pass against the migration oracle.
- `slim` control memory stays below 100 MiB in the target envelope.
- The image contains no Python runtime or source.

### Phase 2: Go Provider And Worker Foundation

**Deliverables**

- Go worker domain interface and authenticated transport adapter.
- Capability discovery, health, job lifecycle, streaming, cancellation, deadlines, and structured errors.
- Admission control, queue metrics, cleanup, and fault injection.
- Remote-provider orchestration and local development harness.
- Go and migration-oracle parity harness.

**Exit gate**

- A synthetic capability passes the full protocol and fault suite.
- Worker crash does not crash control or lose user sessions.
- Bounded streaming and queue behavior pass load tests.
- Previous Go image remains selectable for rollback.

### Phase 3: Retrieval And Document Extraction

**Deliverables**

- Retrieval runtime behind the worker interface.
- Vector adapters initialized only inside retrieval ownership.
- Remote embedding and reranking adapters with quotas and cleanup.
- Go document extraction worker with remote OCR adapters.
- Idempotent indexing and retry behavior.
- Remote embedding/reranking and remote extraction parity.

**Exit gate**

- Retrieval and document behavior suites pass against Go and the migration oracle.
- Control memory remains stable while remote capability jobs process large files.
- Worker restart and retry do not duplicate vectors or document records.
- Current knowledge and file flows require no frontend changes.

### Phase 4: Complete Go Feature Rollout

**Deliverables**

- Remote speech adapters in Go.
- Role-specific container and orchestration definitions.
- Go-only rollout configuration.
- Operational dashboards, alerts, runbooks, capacity guidance, and rollback documentation.
- Upgrade and rollback validation for supported databases.

**Exit gate**

- All quantitative gates pass.
- Canary deployment completes without contract regressions.
- Operators can attribute memory and restart Go workers independently.
- Migration oracle is CI-only and absent from release images.

### Phase 5: Final Go Image And Release

**Deliverables**

- Complete Go route and behavior inventory.
- Final image SBOM and deletion checks.
- Differential HTTP/SSE/WebSocket tests against the migration oracle.
- Build, release, observability, security, and rollback integration.

**Exit gate**

- Go contract and failure parity are complete.
- No Python runtime, source, package, or child process is present in any release image.
- Removing a migrated route requires selecting a previous Go image only.

## AI Agent Execution Protocol

An AI Agent implementing this PRD must follow these rules:

1. Read the current application assembly, configuration precedence, lifecycle, persistence, task registry, stream handling, and relevant capability code before proposing changes.
2. Re-run the baseline or relevant subset before each phase; do not rely on estimates from this PRD as measured facts.
3. Work phase by phase and do not begin a later phase before the current exit gate is met.
4. Use small vertical changes that include interface, adapter, behavior tests, metrics, documentation, and rollback together.
5. Preserve unrelated user changes and avoid broad formatting or dependency churn.
6. Add a failing test before changing ambiguous behavior.
7. Do not introduce an interface unless both compatibility and new adapters use it.
8. Do not move a concrete implementation to another process while it still depends on implicit global application state; make the dependency explicit first.
9. Do not share ORM session objects, FastAPI request objects, model objects, asyncio tasks, or file handles across the worker seam.
10. Do not use arbitrary filesystem paths as worker inputs.
11. Do not claim a memory improvement using package size, image size, virtual memory, or a single RSS snapshot.
12. Do not introduce Go until the Phase 5 entry gate is satisfied.
13. Every pull request must list covered requirement identifiers, new or changed contracts, measurements, test evidence, rollout setting, and rollback method.
14. Every phase-ending pull request must update the risk register and record unresolved deviations.
15. Stop and request an architecture decision when external behavior, persistent data, security posture, or language ownership must change.

## Pull Request Template For Implementation Work

Each implementation pull request should contain:

- **Scope:** phase and requirement identifiers.
- **Behavior:** externally observable behavior changed or explicitly unchanged.
- **Architecture:** module depth gained, interface introduced, and real adapters using it.
- **Compatibility:** golden contracts exercised.
- **Memory evidence:** before/after environment and measurements when relevant.
- **Tests:** exact suites and fault scenarios executed.
- **Deployment:** new role, profile, setting, or manifest behavior.
- **Security:** trust, secret, path, or process implications.
- **Rollout:** disabled, opt-in, canary, or default state.
- **Rollback:** configuration, image, and data requirements.
- **Known gaps:** explicit deviations and follow-up requirements.

## Risk Register

| Risk | Impact | Mitigation | Release Gate |
| --- | --- | --- | --- |
| External request or stream drift | Frontend and clients break | Golden differential contracts | All critical contracts pass |
| Hidden global state crosses process seam | Incorrect models, users, or tasks | State inventory and explicit job context | No implicit application objects in worker interface |
| Duplicate indexing after retry | Corrupt or duplicated knowledge | Idempotency keys and data tests | Retry suite passes |
| Worker overload | Memory exhaustion or latency collapse | Queue limits, admission control, model-aware concurrency | Load test stable |
| Worker crash loses jobs | User-visible failures | Terminal states, retry policy, optional durable records | Crash/recovery suite passes |
| Secrets leak into worker telemetry | Security incident | Minimal context and redaction tests | Security suite passes |
| SQLite used with unsupported concurrency | Locking or corruption | Enforce single control worker and document external DB requirement | Startup validation passes |
| Dependency profile remains transitive-heavy | No baseline memory gain | Architecture/import tests and lock inspection | Control profile denylist passes |
| Model loaded twice concurrently | Large memory spike | Per-runtime load lock | Concurrent first-use test passes |
| Slow stream consumer grows memory | OOM under concurrency | Bounded buffers and backpressure | 30-minute stream test stable |
| Temporary files accumulate | Disk exhaustion and privacy risk | Quotas and deterministic cleanup | Cancellation and crash cleanup pass |
| Mixed versions disagree on contract | Canary failures | Versioned worker handshake and compatibility window | Mixed-version suite passes |
| Go creates duplicate business logic | Divergent behavior and maintenance cost | Delegate authority initially and require decision gate | Differential suite passes |
| Benchmarks are not comparable | False optimization claims | Fixed environment manifest and repeated runs | Benchmark review approved |

## Rollout And Rollback

- New Go capability adapters ship disabled by default until their phase exit gate passes.
- Enable by capability and deployment cohort, not as a global irreversible switch.
- Support mixed Go mode so retrieval and audio adapters can roll out independently.
- Persist the selected adapter explicitly and expose it in diagnostics.
- During canary, compare contract errors, latency, memory, cancellations, worker restarts, and queue rejection against the migration oracle and previous Go image.
- Rollback selects the previous Go image; production never starts the migration oracle.
- Additive database records must be ignored safely by the previous version.
- Destructive schema cleanup occurs only after the compatibility window and in a separate release.
- Model caches and vector data are not deleted automatically during rollback.

## Documentation Deliverables

- Architecture overview of process roles and capability ownership.
- Configuration reference with precedence and examples.
- Dependency-profile and image matrix.
- Local development instructions.
- Production deployment examples for single-host and orchestrated environments.
- Memory benchmarking guide.
- Capacity and concurrency guidance.
- Worker authentication and network exposure guide.
- Upgrade, mixed-version rollout, and rollback runbooks.
- Troubleshooting guide for unavailable dependencies, worker health, model loading, queues, timeouts, and cleanup.
- Architecture decisions for worker transport, job durability, process roles, and any Go introduction.

## Out Of Scope

- Introducing any production backend language other than Go.
- Rewriting the Svelte frontend or replacing its existing request modules.
- Changing user-visible product behavior solely to simplify the refactor.
- Replacing the current primary database in the initial phases.
- Replacing every ORM in the same program.
- Designing a general distributed workflow platform.
- Guaranteeing seamless recovery of every currently ephemeral request across process restart in the first worker milestone.
- Bundling heavyweight local ML inference, OCR, or browser runtimes into the constrained image; these use remote Go adapters.
- Executing arbitrary legacy Python functions in production; supported functions must migrate to Go/WASM/Starlark or be explicitly disabled with a diagnostic.
- Removing the migration oracle from CI before differential contracts pass.
- Optimizing frontend bundle size, browser memory, model-server memory, or Ollama memory except where needed to attribute backend process usage.

## Further Notes

### Current Evidence Versus Required Measurement

Static code evidence establishes that application assembly imports all major routers, the default vector configuration imports Chroma, retrieval imports a broad framework and loader graph, and local model implementations retain model objects after first use. Dependency profiles also show that local workloads add Torch, sentence-transformers, faster-whisper, ONNX, OCR, and document packages.

Static evidence does not establish the exact resident-memory contribution of each dependency on a production host. SQLite mappings, shared libraries, model caches, allocator behavior, child processes, and container accounting must be measured. Phase 0 exists to prevent language or architecture decisions from being based on misleading memory numbers.

### Recommended First Implementation Slice

The first implementation slice should be a tracer bullet for the capability registry:

- Capture a startup import and memory baseline.
- Register one currently optional capability without importing its implementation.
- Keep the old implementation only as a CI migration oracle; it must not be copied into the final image.
- Add a disabled/unavailable/healthy diagnostic contract.
- Prove through an architecture test that the final image contains only the Go backend.
- Record before/after memory and startup behavior.

Retrieval is the highest-value candidate but also highly coupled. A lower-risk optional adapter may be used first to validate the registry mechanics, followed immediately by vector and retrieval ownership.

### Definition Of Done

This PRD is complete only when:

- All functional and non-functional requirements are either satisfied or explicitly waived by an architecture decision.
- All phase exit gates pass with stored evidence.
- The control-only memory gate passes.
- Heavy capabilities execute through remote Go adapters outside the constrained control process.
- Compatibility contracts pass without required frontend changes.
- Production deployment, observability, security, upgrade, and rollback documentation is complete.
- The migration oracle is absent from release images and has a documented CI removal plan.
- The complete Go backend has passed contract, resource, security, and rollout evidence.
