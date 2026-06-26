# PROCOM Architecture

## Phase 1 Goals

Phase 1 establishes the project structure and core boundaries without introducing unnecessary operational complexity.

Key decisions:

- Keep the backend domain-first and framework-light. Wails will be an application shell, not the center of the architecture.
- Isolate each major subsystem behind a focused package boundary so audio capture, transcription, keyword detection, and OSC can evolve independently.
- Keep the frontend presentational in the first phase and defer backend transport binding until the backend contracts are stable.
- Preserve portability by making speech recognition pluggable from the start.

## Why This Structure

### Backend

The backend is organized around business capabilities rather than technical layers. This keeps future real-time pipelines localized and reduces framework leakage.

- `audio` will own device enumeration, capture, buffers, and per-channel pipelines.
- `speech` will expose a stable engine interface that hides MLX Whisper worker details.
- `transcript` will own ordered in-memory conversation state.
- `keywords` will own matching and trigger creation.
- `osc` will own asynchronous delivery.
- `events` will provide decoupled fan-out across subsystems.
- `config` will centralize runtime settings.

### Frontend

The frontend is a separate Vite application so it stays fast to iterate on and straightforward to test. State management is reserved for dedicated stores later rather than pushing business logic into components.

## Risks

- The current machine does not yet have Go installed, so backend compilation depends on local toolchain setup.
- CoreAudio and MLX integration will require careful platform-specific adapters to avoid contaminating cross-platform packages.
- Real-time transcription requires disciplined buffering and backpressure management, which will be addressed in the audio and speech phases rather than approximated now.

## Planned Phase Sequence

1. Project setup
2. Core architecture and application runtime
3. Audio capture and buffering
4. Speech engine process integration
5. Transcript manager
6. Frontend integration
7. Channel management
8. Keyword highlighting
9. OSC delivery
10. Settings
11. Testing hardening
12. Performance optimization

## Phase 2 Design

Phase 2 introduces the application control plane.

Key decisions:

- The backend runtime owns lifecycle management through a narrow `Component` contract with explicit `Start` and `Stop` methods.
- Shared infrastructure is injected through a `Dependencies` struct during composition rather than pulled from globals.
- Configuration is loaded through a `config.Source` abstraction and validated before any subsystem starts.
- The event bus is upgraded to a multicast fan-out primitive so audio, speech, transcript, keyword, and UI-facing adapters can subscribe independently.

## Why These Decisions

### Lifecycle Management

Real-time production software cannot rely on ad hoc startup ordering. Components need deterministic startup, reverse-order shutdown, and rollback if one subsystem fails during boot.

### Dependency Injection

Using factories with shared dependencies keeps package boundaries explicit without introducing a heavyweight container. It also makes later subsystem tests straightforward.

### Config Source Boundary

JSON remains the storage format, but the runtime only depends on a `Source` interface. That allows file-backed settings, test fixtures, or future migrations without changing the runtime.

### Multicast Events

The original buffered channel was not a real bus because only one consumer could drain it safely. A proper fan-out model is required before we can add concurrent pipelines that observe the same domain events.

## Phase 2 Risks

- Non-blocking event delivery means overloaded subscribers can miss events. That tradeoff prevents one slow consumer from stalling the rest of the system, but later phases should add metrics and targeted backpressure handling where loss is unacceptable.
- Component startup currently assumes each subsystem can honor context cancellation promptly. Audio and speech adapters will need extra care here.

## Phase 3 Design

Phase 3 introduces the audio subsystem control path and buffering model.

Key decisions:

- Platform-specific capture is isolated behind `audio.Driver` and `audio.Stream` interfaces.
- The audio manager owns one independent session per enabled channel.
- Each session writes into its own fixed-capacity ring buffer and publishes captured chunks onto the internal event bus.
- Missing devices and per-channel startup failures degrade only the affected pipeline; the application remains running.

## Why These Decisions

### Driver Boundary

CoreAudio support is a platform adapter concern. The rest of the application should depend on stable capture contracts instead of a specific macOS implementation.

### Per-Channel Sessions

Each input must remain isolated so slow, failing, or disconnected hardware affects only its own pipeline. A session model gives us the right ownership boundary for later VAD and speech handoff.

### Ring Buffering

Speech engines and diagnostics both need a rolling window of recent audio. A fixed-capacity ring buffer provides predictable memory use and avoids blocking capture paths.

### Event Publication

Publishing inventory, pipeline status, and chunks through the event bus keeps UI adapters and later speech components decoupled from capture internals.

## Phase 3 Risks

- The current implementation provides the production capture boundary and buffer management, but not the CoreAudio adapter itself. That platform adapter should be added without changing the manager or session model.
- Ring buffer overwrite favors liveness over losslessness. If a downstream component cannot tolerate dropped historical frames, it will need a tighter handoff contract than the shared snapshot buffer.

## Phase 4 Design

Phase 4 integrates speech recognition through a supervised worker process.

Key decisions:

- The backend communicates with speech workers using JSON Lines over stdin and stdout.
- The default engine implementation is modeled as an MLX Whisper worker client, but the rest of the application only depends on the `speech.Engine` interface.
- A speech manager subscribes to captured audio events, forwards chunks to the engine, and republishes partial and final transcript events.
- Worker lifecycle, ready handshakes, stderr logging, and process exit supervision remain inside the speech package.

## Why These Decisions

### Worker Isolation

Inference is the heaviest and most failure-prone part of the stack. Running it out of process protects the main runtime from engine crashes and keeps platform-specific dependencies isolated.

### JSON Lines IPC

JSON Lines is simple to inspect, straightforward to test, and adequate for the control and streaming messages required at this stage.

### Event-Driven Speech Flow

The audio manager publishes chunks without knowing anything about inference. The speech manager consumes those events and republishes recognition updates, which keeps subsystem coupling low.

### Supervision Inside Speech

MLX-specific startup, worker readiness, stderr handling, and process shutdown belong in the speech package. Other packages should not know how inference is hosted.

## Phase 4 Risks

- This phase delivers the production IPC boundary and runtime supervision, but it does not bundle the actual MLX Whisper worker binary. Real transcription remains configuration-dependent until that worker command is installed.
- JSON Lines is suitable for early production integration, but very high throughput may later benefit from tighter framing or binary audio transport if profiling shows IPC overhead is material.

## Phase 5 Design

Phase 5 establishes transcript state as an in-memory domain service.

Key decisions:

- The transcript manager subscribes to speech result events rather than talking to speech engines directly.
- Partial and finalized transcripts are stored separately, with one live partial per channel and an ordered finalized timeline.
- Timeline ordering is enforced inside the manager based on transcript timestamps.
- The manager exposes snapshot-style read APIs and publishes lightweight update events when transcript state changes.

## Why These Decisions

### Transcript Ownership

Speech recognition should not also own transcript state. A dedicated transcript manager gives one place to handle ordering, temporary partials, and future keyword annotations.

### Separate Partial State

Temporary text behaves differently from finalized messages. Keeping partials separate avoids polluting the durable in-memory conversation timeline while still supporting live UI rendering.

### Snapshot Reads

Consumers such as the UI and keyword systems need stable read models. Snapshot APIs avoid leaking internal mutable state.

### Lightweight Update Events

Most consumers do not need the entire timeline on every change. Publishing compact update events keeps event traffic smaller while leaving full snapshot reads available on demand.

## Phase 5 Risks

- Timeline storage is currently in-memory only, which matches the product requirement, but very long sessions may eventually need retention limits or virtualization strategies in later layers.
- Sorting on every finalized insert is acceptable at current scope, but if channel count and transcript volume increase significantly, insertion strategy should be revisited.

## Phase 6 Design

Phase 6 integrates the frontend against a typed runtime-facing transcript model.

Key decisions:

- React components stay presentational while a Zustand store owns frontend transcript state.
- The UI talks to a narrow backend adapter interface instead of embedding transport assumptions in components.
- The first adapter is a mock offline bridge that mirrors the transcript snapshot and update model from the backend.
- Transcript filtering and runtime status rendering happen in the frontend state layer rather than inside individual UI fragments.

## Why These Decisions

### Typed Frontend Boundary

The frontend should depend on stable view models, not on raw backend events or framework-specific transport calls. A typed adapter keeps the eventual Wails binding replaceable.

### Store-Owned State

Conversation state changes continuously. Keeping it in Zustand avoids scattering subscription and merge logic across React components.

### Mock Bridge First

Wails integration is still intentionally deferred. A mock bridge allows the UI architecture, live update flow, and filtering behavior to be validated now against the same shape the real backend will provide.

### Presentation-Focused Components

The UI is split into focused sidebar, header, and timeline components so layout work stays separate from transcript state orchestration.

## Phase 6 Risks

- The frontend currently simulates the runtime bridge rather than consuming a real desktop transport. The next integration step should swap the adapter implementation, not rewrite the UI state model.
- Very high-frequency transcript updates may eventually need finer-grained store update patterns, but the current structure is appropriate for the initial two-channel scope.

## Phase 7 Design

Phase 7 makes channels a mutable runtime-owned domain model.

Key decisions:

- Channel state is owned by a dedicated backend manager instead of being treated as immutable startup configuration.
- Channel edits flow through a typed update request and publish change events.
- The frontend adapter now supports channel writes in addition to transcript reads.
- Channel editing UI lives in the settings view while the sidebar remains a read-only projection of current channel state.

## Why These Decisions

### Backend Channel Ownership

Renaming channels and changing their presentation metadata are domain operations. They should be handled by a backend service that owns validation and update events.

### Typed Update Path

Using a focused update request keeps the future Wails bridge explicit and testable, rather than pushing ad hoc object mutation across the frontend boundary.

### Read Model Separation

The sidebar should render the current channel snapshot, not manage edits itself. Keeping editing in the settings surface avoids mixing command and projection logic in the same UI control.

### Mock Write Flow First

The frontend can now exercise real channel edits against the adapter boundary before the desktop transport is present, which validates the shape of the eventual integration.

## Phase 7 Risks

- Historical transcript entries currently update their displayed channel metadata in the mock frontend bridge. The backend still treats transcript entries and channel state independently, so a later product decision is needed on whether past messages should preserve original labels.
- Channel edits are still in-memory only. Persisted settings and restart semantics belong to the later settings phase.

## Phase 8 Design

Phase 8 introduces keyword detection and transcript highlighting for finalized speech.

Key decisions:

- Keyword matching runs at the finalized transcript boundary, not during partial recognition.
- Matching logic lives in `internal/keywords` and is consumed by the transcript manager when entries are finalized.
- Transcript entries carry detected keyword phrases and highlight ranges as part of their domain model.
- The frontend renders highlights from transcript metadata rather than reimplementing matching rules locally.

## Why These Decisions

### Finalized-Only Matching

Keyword triggers should be stable and low-noise. Running detection on finalized transcript text avoids churn from partial hypotheses.

### Dedicated Matcher Package

Matching rules, case sensitivity, and whole-word behavior are domain logic. Keeping that logic in `internal/keywords` makes it reusable for later OSC triggering.

### Transcript-Carried Highlight Metadata

Once a finalized message is stored, its matched keyword metadata should travel with it. That avoids re-matching in every downstream consumer.

### Frontend Rendering Only

The frontend should render highlight metadata, not decide what counts as a keyword match. That keeps business rules backend-owned.

## Phase 8 Risks

- Highlight ranges are currently computed in rune offsets, which is the right semantic model for text, but the transport layer will need to preserve that interpretation consistently.
- The mock frontend bridge still simulates highlighted entries locally until the real backend transport is connected.

## Desktop Transport

The frontend now prefers a Wails-facing runtime adapter when the desktop transport is present and falls back to the mock adapter only when running outside that environment.

Key decisions:

- Wails-bindable methods are exposed through a dedicated frontend bridge service rather than being attached directly to the application runtime.
- The desktop bridge emits compact state updates from internal transcript and channel events using the same snapshot and update shapes the mock adapter already used.
- Transport-specific concerns stay outside the domain packages so the backend remains compileable and testable without requiring the full desktop shell during everyday development.

Risks:

- The shell now binds a concrete Wails emitter in the desktop entrypoint, but browser builds still correctly fall back to the mock adapter when the runtime is absent.
- Asset delivery currently serves the built `frontend/dist` directory from the local filesystem. Packaging workflows may later choose to embed assets for distribution builds.

## Phase 9 Design

Phase 9 introduces asynchronous OSC delivery from finalized keyword matches.

Key decisions:

- OSC dispatch is triggered from finalized transcript entries that already contain keyword metadata.
- The OSC manager subscribes to transcript update events rather than re-running keyword detection itself.
- UDP send work is queued and dispatched asynchronously so network errors never block transcription or UI updates.
- The built-in OSC sender currently supports the configured string argument model directly from keyword rules.

## Why These Decisions

### Reuse Finalized Keyword Metadata

Keyword matching already happens once at transcript finalization. OSC should consume that result instead of introducing a second, divergent detection path.

### Event-Driven OSC

Using transcript update events keeps OSC decoupled from speech and transcript internals while still triggering promptly after finalization.

### Asynchronous Delivery

OSC failures must never interrupt live operation. A queued sender isolates outbound network failures from the rest of the runtime.

### Narrow Initial Payload Model

Current keyword configuration already models OSC arguments as strings. Supporting that shape first keeps the sender simple and aligned with existing configuration.

## Phase 9 Risks

- The current OSC encoder supports the configured string-argument model only. If typed numeric arguments become necessary, the message encoder should be expanded deliberately rather than inferred ad hoc.
- OSC delivery is in-memory and fire-and-forget by design. If operators need delivery observability, later phases should add surfaced diagnostics rather than coupling send success back into transcript flow.
