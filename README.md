*This project has been created as part of the 42 curriculum by khhammou, ekarout.*

# The Answer Protocol (TAP)

A real-time multiplayer text adventure (MUD) server and client ecosystem built in Go as part of the 42 curriculum.

Players connect over raw TCP sockets, explore an interconnected virtual world, interact with non-player characters (NPCs), engage in turn-based combat, form party groups, and communicate through scoped chat channels according to the **RFC 42TAP** specification.

---

## Description

The Answer Protocol (TAP) implements a classic text-based shared-world adventure powered by a concurrent, non-blocking TCP server. 

The project emphasizes:
- **Zero-CGO & Minimal Dependencies**: 100% Go standard library for networking, concurrency, and logging.
- **Strict Concurrency Safety**: Goroutine isolation with channel-based I/O buffering and read-write mutex protection.
- **Protocol Adherence**: 100% compliance with the newline-delimited RFC 42TAP text protocol.
- **Integrated Abuse Mitigation**: Built-in sliding-window rate limiting and rapid reconnect flood detection.

---

## Instructions

### Prerequisites
- **Go**: Version 1.22 or later
- **Make**: Standard GNU Make (or execute direct `go` commands)
- **TCP Client**: Netcat (`nc`), Ncat, or Telnet for terminal testing
- **Browser**: Modern web browser (Chrome, Firefox, Edge, Safari) for the Web GUI client

### Quick Start
1. **Clone the repository**:
   ```bash
   git clone <repo-url>
   cd TAP
   ```
2. **Start the server**:
   ```bash
   make run-server
   ```
   The server will start listening on `localhost:8080`.
3. **Connect via CLI Client**:
   ```bash
   make run-client
   ```
4. **Launch the Web GUI Client**:
   ```bash
   make run-client-gui
   ```

---

## Resources

### References
- **RFC 42TAP Specification**: Located at [`rfc/protocol-rfc.html`](rfc/protocol-rfc.html) defining wire protocol grammar, status codes, and broadcast event formats.
- **Go Standard Library Documentation**: 
  - `net` — Network socket listening, dialing, and connection lifecycle management.
  - `bufio` — Buffered I/O scanners for newline-delimited stream reading.
  - `log/slog` — Structured JSON logging and severity filtering.
  - `sync` — Concurrency synchronization primitives (`sync.RWMutex`).

### AI Usage Statement
In accordance with 42 curriculum guidelines, artificial intelligence was utilized during development for:
- Initial architectural scaffolding and concurrency pattern analysis.
- Test case expansion and edge-case scenario generation for network flood and group lifecycle testing.


---

## Architecture

The server employs a decoupled, multi-goroutine concurrency model designed to ensure that slow or unresponsive clients cannot block the server or degrade gameplay for others.

```text
                     ┌────────────────────────────────────────┐
                     │          Accept Loop (server.go)       │
                     │          net.Listen("tcp", :8080)      │
                     └───────────────────┬────────────────────┘
                                         │ Accepts TCP Connection
                                         ▼
                     ┌────────────────────────────────────────┐
                     │     Abuse Tracker (reconnect.go)       │
                     │     Sliding window: >5 conn / 10 sec   │
                     └───────────────────┬────────────────────┘
                                         │ Spawns 2 goroutines per client
                        ┌────────────────┴────────────────┐
                        ▼                                 ▼
             ┌─────────────────────┐           ┌─────────────────────┐
             │ Client.ReadLoop()   │           │ Client.WriteLoop()  │
             │ bufio.NewScanner    │           │ for msg := range    │
             │ Reads line by line  │           │   c.send chan       │
             └──────────┬──────────┘           └──────────▲──────────┘
                        │ Line string                     │ c.Enqueue(msg)
                        ▼                                 │
             ┌─────────────────────┐                      │
             │   Route() Router    │                      │
             │ Rate-limiter (>20/s)│                      │
             └──────────┬──────────┘                      │
                        │                                 │
        ┌───────────────┴───────────────┐                 │
        ▼                               ▼                 │
┌───────────────┐              ┌─────────────────┐        │
│ Router Parser │              │  Hub (hub.go)   │────────┘
│ CONNECT, QUIT,│              │  Clients map    │ Broadcast
│ WHO, CHAT...  │              │  sync.RWMutex   │ scopes
└───────┬───────┘              │  Event Dispatch │
        │                      └─────────────────┘
        ▼ Unhandled Commands
┌───────────────────┐
│ GameHandler Hook  │ ──► Phase 4: Game Engine (server/game/)
└───────────────────┘
```

### Key Architectural Pillars
1. **Accept Loop (`server.go`)**: Listens on TCP `:8080`. Upon accept, extracts the caller IP, evaluates it against the rapid reconnect tracker, emits structured connection logs, sends `OK hello proto=1\n`, and hands off the socket.
2. **Channel Decoupling (`client.go`)**: Each client runs two dedicated goroutines:
   - `ReadLoop`: Continuously scans newline-terminated lines from the socket and passes them into the router. Owns socket and channel teardown in its `defer` block.
   - `WriteLoop`: Continuously drains the buffered `c.send` channel (capacity 128) and writes to the socket. Slow clients or socket stalls drop saturated packets non-blockingly without stalling the Hub.
3. **Central Hub (`hub.go`)**: Coordinates client registration, username uniqueness, room player rosters, and event fanout across four distinct scopes (`global`, `room`, `direct`, `group`) protected by `sync.RWMutex`.
4. **Command Router (`router.go`)**: Tokenizes input verbs, enforces authentication guards (`ERR 100 NOT_CONNECTED`), applies per-client rate limits (>20 cmds/sec triggers `ERR 429 RATE_LIMITED`), and dispatches commands. Unrecognized commands delegate cleanly to the `GameHandler` hook.

---

## Protocol Implementation

The server strictly adheres to **RFC 42TAP** with zero deviations:
- **Wire Format**: Line-based ASCII/UTF-8 strings delimited by `\n`.
- **Handshake**: Server greets on connect with `OK hello proto=1\n`.
- **Command Syntax**: `<COMMAND> [ARGUMENTS...]\n`.
- **Response Format**:
  - Success: `OK [payload]`
  - Error: `ERR <code> <ENUM_STRING>` (e.g. `ERR 100 NOT_CONNECTED`, `ERR 201 NAME_IN_USE`, `ERR 401 NOT_IN_GROUP`, `ERR 429 RATE_LIMITED`).
- **Broadcast Events**: `EVT <SCOPE> <EVENT_TYPE> [payload]` (e.g. `EVT GLOBAL CHAT`, `EVT ROOM CHAT`, `EVT GROUP CHAT`, `EVT GROUP INVITE`, `EVT GROUP JOIN`, `EVT GROUP LEAVE`, `EVT ROOM PRESENCE ENTER/LEAVE`).

---

## Combat System

*(In development by khhammou)*

---

## Quest System

*(In development by khhammou)*

---

## World Design

*(To be documented by ekarout from `world_data/world.yaml`)*

---

## Server Logging

The server features structured JSON logging implemented with Go's `log/slog` package, emitting to `os.Stdout` at `LevelDebug` for real-time observability.

### Log Event Taxonomy
| Message Event | Level | Trigger | Key Attributes |
|---|---|---|---|
| `server_started` | INFO | Server boots up | `port` |
| `connection_accepted` | INFO | TCP socket accepted | `ip` |
| `player_connected` | INFO | Handshake `CONNECT <name>` success | `player`, `ip` |
| `player_disconnected` | INFO | Client disconnects or sends `QUIT` | `player` |
| `command_received` | DEBUG | Client submits command line | `player`, `cmd` |
| `response_sent` | DEBUG | Server returns `OK` or `ERR` to client | `player`, `response` |
| `error_sent` | WARN | Server returns an error code | `player`, `code` |
| `event_broadcast` | INFO | Hub fans out an event | `scope`, `payload` |
| `abuse_cmd_flood` | WARN | Client exceeds 20 cmds/sec | `player`, `ip`, `count_per_sec` |
| `abuse_rapid_reconnect`| WARN | IP reconnects >5 times in 10s | `ip`, `attempts` |

### Sample JSON Output
```json
{"time":"2026-11-09T10:53:07.240Z","level":"INFO","msg":"player_connected","player":"alice","ip":"127.0.0.1:54321"}
{"time":"2026-11-09T10:53:07.241Z","level":"DEBUG","msg":"command_received","player":"alice","cmd":"CHAT"}
{"time":"2026-11-09T10:53:07.242Z","level":"INFO","msg":"event_broadcast","scope":"global","payload":"EVT GLOBAL CHAT alice Hello!"}
{"time":"2026-11-09T10:53:07.242Z","level":"WARN","msg":"abuse_cmd_flood","player":"alice","ip":"127.0.0.1","count_per_sec":21}
```

---

## Group Contributions

Per the official 42 Subject specification (Chapters II & VII.2), responsibilities are divided between two group members:

### 🧑‍💻 khhammou (Person 1) — *Server Core, Combat Engine & Web GUI*
- **Server Network Architecture**: TCP socket listener, Accept loop, per-client goroutine pumps, and channel buffering (`server/network/server.go`, `client.go`).
- **Hub & Routing Engine**: Central thread-safe client registry, broadcast fanout, and command router (`server/network/hub.go`, `router.go`).
- **Structured Logging & Abuse Detection**: JSON logging, sliding-window command rate limiting, and rapid reconnect mitigation (`server/logger/logger.go`, `server/network/reconnect.go`).
- **Combat & State Engine**: Turn-based combat system, health tracking, player state machine, and command routing (`server/game/game.go`, `types.go`).
- **Local Web UI (Zero-CGO)**: Browser-based graphical interface powered by standard library `net/http` and WebSocket/TCP bridge (`gui_client/main.go`, `gui_client/index.html`).
- **Build Infrastructure & Tests**: `Makefile`, `go.mod`, and 41 comprehensive unit tests.

### 🛠️ ekarout (Person 2) — *World Lore, YAML Engine & Terminal CLI *
- **World Design & Lore**: Static game world configuration featuring 8+ rooms, circular routes, branching paths, NPCs, items, and quests (`world_data/world.yaml`).
- **Backend YAML Loader & Validator**: Go engine that parses `world.yaml` using `gopkg.in/yaml.v3`, validates exit graph connectivity, verifies item/NPC spawns, and includes unit tests (`server/game/world.go`, `server/game/world_test.go`).
- **Terminal CLI Client**: Pure Go interactive terminal client with ANSI color formatting, unbuffered prompt reading, and clean exit handling (`cli_client/main.go`).
- **Documentation**: World lore guide, room map layout, and CLI walkthroughs in `README.md`.

---

## Building and Running

### Build All Components
```bash
make install
```

### Run the Server
```bash
make run-server
# Or directly:
go run server/network/*.go
```

### Run the CLI Client
```bash
make run-client
# Or directly:
go run cli_client/main.go
```

### Run the GUI Client
```bash
make run-client-gui
# Or directly:
go run gui_client/main.go
```

### Clean Build Artifacts
```bash
make clean
```

---

## Testing

### Automated Unit Test Suite
Run all unit tests across the codebase:
```bash
make test
# Or directly:
go test -v ./...
```
*(All 41 unit tests pass with zero failures).*

### Static Analysis
Run Go's official static analysis tool:
```bash
make lint
# Or directly:
go vet ./...
```
*(Zero warnings or errors).*

### Manual Multiplayer Verification
To verify real-time multiplayer features manually:
1. Start the server in terminal 1:
   ```bash
   make run-server
   ```
2. In terminal 2, connect as Alice:
   ```bash
   nc localhost 8080
   CONNECT alice
   MOVE tavern
   ```
3. In terminal 3, connect as Bob:
   ```bash
   nc localhost 8080
   CONNECT bob
   MOVE tavern
   ```
4. Observe that terminal 2 receives `EVT ROOM PRESENCE ENTER bob`.
5. Send room and global chat messages to verify scoped broadcasts.
6. For full manual test scripts and edge-case testing, refer to the local test runbook in `docs/tests.md`.
