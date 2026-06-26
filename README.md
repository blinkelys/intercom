# INTERCOM

INTERCOM is an offline desktop transcription tool for live event production.

## Current Status

This repository currently contains the working desktop runtime for:

- a Go backend with explicit package boundaries for audio, speech, transcript, keywords, OSC, configuration, and events
- a lifecycle-managed application runtime with dependency injection, startup rollback, and graceful shutdown
- validated configuration loading through pluggable config sources
- a multicast internal event bus suitable for concurrent subsystems
- an audio manager that owns per-channel capture sessions, ring buffers, and pipeline status events
- a native macOS audio helper that enumerates real capture devices and streams PCM audio into the backend pipeline
- a supervised MLX Whisper worker client using JSON Lines over stdin and stdout
- a speech manager that bridges captured audio events into partial and final transcript events
- a transcript manager that owns ordered finalized entries and per-channel live partial state
- snapshot and update-event APIs for live transcript consumers
- a React and TypeScript frontend that reads from a typed runtime adapter through Zustand-managed state
- a mock offline frontend bridge that mirrors backend transcript snapshots and live updates
- a backend channel manager with typed in-memory updates and channel change events
- a settings editor that can rename channels and change icon, color, language, input device, and enabled state through the adapter boundary
- a backend keyword matcher that enriches finalized transcript entries with detected phrases and highlight ranges
- timeline rendering that displays keyword highlights from transcript metadata
- a Wails-facing frontend bridge service and client that can replace the mock adapter when the desktop runtime is attached
- a root Wails desktop entrypoint that binds the frontend bridge and runs the backend runtime inside the app shell
- asynchronous OSC delivery triggered from finalized keyword matches
- architecture notes describing the phase plan and initial constraints

## Structure

- `main.go`: Wails desktop entrypoint
- `cmd/procom`: backend executable entrypoint
- `internal/app`: application bootstrap, dependency composition, and lifecycle
- `internal/audio`: audio capture contracts, ring buffers, and per-channel pipeline orchestration
- `internal/channels`: runtime-owned mutable channel state and update events
- `internal/keywords`: finalized-transcript keyword matching rules and highlight range generation
- `internal/bootstrap`: shared runtime wiring used by headless and desktop entrypoints
- `internal/frontendbridge`: Wails-bindable frontend transport bridge and snapshot mapping
- `internal/speech`: worker-backed speech engine integration and transcript event bridging
- `internal/transcript`: in-memory transcript timeline, partial state, and snapshot APIs
- `internal/config`: runtime configuration types, validation, and sources
- `internal/events`: internal event bus and subscriber management
- `frontend/src/backend`: frontend runtime adapter boundary
- `frontend/src/store`: Zustand-owned UI state
- `frontend/src/components`: presentation-focused transcript UI components
- `internal/osc`: asynchronous OSC client contracts
- `frontend`: React UI shell
- `docs`: architecture notes and phase rationale

## Development

Requirements:

- macOS with Apple Silicon for the current MLX Whisper path
- Xcode command line tools available through `xcrun` and `swift`
- Python 3 with `mlx-whisper` installed
- Go 1.23+

Install the speech dependency:

```bash
python3 -m pip install --user mlx-whisper
```

Frontend:

```bash
cd frontend
npm install --cache ../.npm-cache
npm run build
```

Backend:

```bash
go test ./...
```

Desktop shell without the Wails CLI:

```bash
npm --prefix frontend run build
go run .
```

Repo-local Wails CLI wrapper:

```bash
./scripts/wails.sh doctor
./scripts/wails.sh dev
```

Accuracy launch profiles:

```bash
# Balanced laptop testing
./scripts/run-profile.sh laptop-balanced dev

# Strict laptop mode (higher latency, higher stability)
./scripts/run-profile.sh laptop-hyper dev

# Production talkback feed mode
./scripts/run-profile.sh talkback-hyper dev
```

Available profile env files:

- `scripts/profiles/laptop-balanced.env`
- `scripts/profiles/laptop-hyper.env`
- `scripts/profiles/talkback-hyper.env`

### Live Production Accuracy Checklist

1. Use a direct line/talkback feed into your audio interface whenever possible.
2. Set each channel language explicitly in settings (avoid auto-detection behavior).
3. Keep gain conservative (avoid clipping) and verify clean peaks before show start.
4. Confirm the selected input device in settings is the line/talkback source.
5. Run `./scripts/run-profile.sh talkback-hyper dev` for show mode.
6. Use laptop profiles only for rehearsal or fallback testing.

The wrapper installs `wails` into `.tools/bin` on first use, so it does not require modifying your global `PATH`.

Wails runtime integration is wired through the root desktop entrypoint and the frontend bridge. On macOS, the audio runtime now uses the native Swift helper by default, and speech defaults to the embedded MLX Whisper worker launched through `python3`.
