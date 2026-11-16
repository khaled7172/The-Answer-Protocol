*This project has been created as part of the 42 curriculum by khhammou, ekarout.*

# TAP (The Answer Protocol)

## Description

TAP is a small multiplayer text adventure (a MUD) written in Go. Players connect to a TCP server, walk around a shared world, pick up items, talk to NPCs, fight monsters, do quests, form groups and chat. The server speaks the line-based RFC 42TAP protocol, so any client that follows the RFC can connect to it.

The repo contains three programs:

- `server/`: the game server (TCP, port 8080)
- `cli_client/`: a terminal client
- `gui_client/`: a web client (small Go server + one HTML page)

The world itself lives in `world_data/world.yaml`.

## Instructions

You need Go 1.27.1 or newer and `make`. The server and both clients are started from the repository root.

```bash
make install         # download dependencies (yaml.v3)
make run-server      # start the server on :8080
make run-client      # terminal client, in another terminal
make run-client-gui  # web client, opens http://localhost:3000
```

You can also just use `nc localhost 8080` and type protocol commands by hand.

## Resources

- RFC 42TAP, in `rfc/protocol-rfc.html`
- Go docs for [`net`](https://pkg.go.dev/net), [`bufio`](https://pkg.go.dev/bufio), [`sync`](https://pkg.go.dev/sync) and [`log/slog`](https://pkg.go.dev/log/slog)
- [RFC 6455](https://www.rfc-editor.org/rfc/rfc6455), the WebSocket protocol (used by the GUI bridge)
- [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3)
- [Multi-user dungeon](https://en.wikipedia.org/wiki/Multi-user_dungeon) on Wikipedia, for background

**How we used AI.** We used an AI assistant to:

- find concurrency bugs (data races, a crash on double `CONNECT`, the disconnect order);
- write unit tests;

## Architecture

We went with a router (dispatcher) rather than inline handling, and two goroutines per connected player.

```text
accept loop ── new client ──► ReadLoop ──line──► Route() ──► game.HandleCommand()
                                                    │                 │
                                                    │      reply + list of events
                                                    ▼                 ▼
                                           client.Enqueue(reply)   hub.Send(events)
                                                    │
                              WriteLoop ◄── send channel (128 slots)
```

- `server.go` loads and validates the world at startup and refuses to run if the YAML is broken. Then it accepts connections, sends the `OK hello proto=1` greeting and starts the two goroutines.
- `client.go`: `ReadLoop` reads one line at a time and hands it to the router. `WriteLoop` writes queued messages to the socket. Sending is non-blocking: if a client stops reading and its queue fills up, its messages get dropped instead of freezing everyone else.
- `router.go` handles rate limiting, the "must `CONNECT` first" check and the player-level commands (`CONNECT`, `QUIT`, `CHAT`, `WHO`, `GROUP`). Everything else goes to the game.
- `hub.go` is the list of connected players and their groups, behind one `RWMutex`. It delivers events.
- `game/game.go` owns the world state (rooms, items, NPCs, HP, quests) behind its own mutex. It never calls the network code. It just returns the reply and the events to send, along with who should get them, so there is no lock cycle between the two.

When a player disconnects, they are removed from the hub and the game first (their items drop on the floor). Only then are the leave events sent.

## Protocol Implementation

Every command, reply and event from the RFC is implemented. Command names are case-insensitive. Items and NPCs can be named by ID or by display name, including names with spaces (`TAKE magic potion`).

Where we differ from the RFC:

| What | RFC | Us | Why |
|---|---|---|---|
| `WHO` reply | `OK players=<n>` | `OK {"room":[...],"server":n}` | This is the format in the subject's example, and the GUI needs both numbers. |
| `TALK` reply | `OK <text>` | `OK {"npc":"...","dialogue":"..."}` | Same, it's the subject's example. |
| Presence events | sent to the room | not sent to the player who moved | They already get `OK room=<id>`. |
| Extra commands | | `DEFEND`, `FLEE`, `USE <item>` | Combat commands, which the RFC leaves to us. |
| Extra event | | `EVT ROOM COMBAT <player> <npc> <combat\|victory\|defeated>` | The subject wants combat results broadcast, and the RFC has no event for it. |

Extra error replies, for cases the RFC doesn't cover:

| Reply | When |
|---|---|
| `ERR 100 NOT_CONNECTED` | a command other than `CONNECT`/`QUIT` before connecting |
| `ERR 100 ALREADY_CONNECTED` | `CONNECT` a second time on the same connection |
| `ERR 100 MISSING_ARG` | a required argument is missing |
| `ERR 100 UNKNOWN_COMMAND` | unknown command |
| `ERR 100 INVALID_SCOPE`, `ERR 100 UNKNOWN_ACTION` | bad `CHAT` scope or `GROUP` action |
| `ERR 400 IN_COMBAT` | `MOVE` while fighting (use `FLEE`) |
| `ERR 400 NOT_IN_COMBAT` | `DEFEND` or `FLEE` outside a fight |
| `ERR 400 CANNOT_USE` | `USE` on something that doesn't heal |
| `ERR 404 PLAYER_NOT_FOUND`, `ERR 404 GROUP_NOT_FOUND` | `GROUP INVITE`/`JOIN` target is offline or has no group |
| `ERR 429 RATE_LIMITED` | more than 20 commands in one second |

## Combat System

Combat is turn-based: one command is one round. The player always goes first, and the enemy hits back if it's still alive. We kept it deterministic (no dice) so it's easy to follow and to test.

- Start a fight with `ATTACK <npc>`. Only NPCs with `hostile: true` can be attacked. Others answer `ERR 405 NPC_NOT_HOSTILE`. Each player fights their own copy of the enemy at full HP.
- Your damage is 10, plus the best weapon you're carrying (Wooden Sword +15, Steel Sword +30).
- The enemy's damage is its `stats.damage` from the YAML.

| Command | You deal | You take |
|---|---|---|
| `ATTACK` | full damage | full damage |
| `DEFEND` | half | half |
| `USE <item>` | nothing, but you heal (up to 100 HP) | full damage |
| `FLEE` | the fight ends and you run out a random exit | nothing |

Players start with 100 HP and can't `MOVE` during a fight. A defeated enemy disappears from the room and comes back after 60 seconds, so other players can still do kill quests. If you hit 0 HP you respawn at the starting location with 50 HP.

A round replies with `OK {"attacker_hp":..,"target_hp":..,"damage":..,"status":"combat|victory|defeated"}`, plus `"respawn"` when you die. The room gets an `EVT ROOM COMBAT` line, and every round is logged. `STATUS` shows `hp`, `max_hp`, `status` (healthy, wounded, critical), `in_combat`, `gold` and `xp`.

## Quest System

Each quest in the YAML has a giver NPC, an objective (`defeat_npc` or `find_item`, with a `quantity`) and a reward (an item, gold, XP).

- `QUEST <npc>` gives you the first quest from that NPC you haven't finished yet. If an NPC has several quests, you get them one after another, like a short chain.
- Kills of the right NPC are counted while the quest is active. For item quests we just count the items in your inventory. `QUESTS` shows progress like `1/2`.
- To finish, go back and `QUEST <npc>` again. If you've done it, the quest is marked completed and the NPC says its completion line. Items for `find_item` quests are taken from you (you deliver them), and you get the reward. You can't repeat a finished quest. When an NPC has nothing left to give, you get `ERR 406 NO_QUEST_AVAILABLE`.

## World Design

### The Whispering Woods

A dark corruption is spreading through the Whispering Woods. The Forest Spirit, the old guardian of the forest, is getting weaker, and a pack of corrupted wolves is attacking everything that lives there. You arrive in a glowing clearing with a wooden sword and some advice from an old guide.

There are 9 locations in `world_data/world.yaml`. The Forest Spirit gives both quests: **The Corrupted Forest** (kill 2 wolves) and then **The Lost Magic** (bring back the Magic Crystal).

### Map

```
        [Luminara] --- [The Ancient Elder]
            |                   |
   [The Whispering Clearing] [Moonlit Pond]
       /          \           /
[Silver River] [Mushroom Grove]
       |                |
[Hidden Waterfall]  [Crystal Cave]
                         |
                  [Forgotten Shrine]
```

```
forest_clearing (start)
  north -> fairy_village -> elder_tree -> moonlit_pond -> mushroom_grove -> back west to the clearing
  east  -> mushroom_grove -> crystal_cave -> forgotten_shrine
  south -> silver_river -> hidden_waterfall
```

The main loop is clearing → fairy village → elder tree → moonlit pond → mushroom grove → clearing. Two side branches hang off it: the cave path up to the Forgotten Shrine, and the river path down to the Hidden Waterfall.

### Locations

| Location | What's there |
|---|---|
| The Whispering Clearing | start, Wooden Sword, Forest Guide |
| Luminara | Fairy Merchant |
| The Ancient Elder | Forest Spirit (quest giver) |
| The Moonlit Pond | Magic Potion |
| The Glowing Mushroom Grove | 4 Corrupted Wolves |
| The Crystal Cave | Magic Crystal, Crystal Guardian |
| The Forgotten Shrine | ancient runes |
| The Silver River | path south |
| The Hidden Waterfall | Steel Sword |

### NPCs and Items

NPC roles: dialogue (Forest Guide, Fairy Merchant), quest giver (Forest Spirit), enemies (Corrupted Wolf: 40 HP, 5 damage; Crystal Guardian: 70 HP, 15 damage).

| Item | Effect | Where |
|---|---|---|
| Wooden Sword | +15 damage | Whispering Clearing |
| Magic Potion (`health_potion`) | heals 20 HP | Moonlit Pond |
| Magic Crystal | quest item | Crystal Cave |
| Steel Sword | +30 damage | Hidden Waterfall |
| Ancient Seed | quest reward | given by the Forest Spirit |

## Server Logging

The server logs JSON to stdout with Go's `log/slog`, one line per event. Every line has a timestamp and a level (DEBUG, INFO, WARN, ERROR).

| Event | Level | Fields |
|---|---|---|
| `server_started` | INFO | `port` |
| `world_load_failed`, `listen_failed` | ERROR | `err` |
| `connection_accepted` | INFO | `ip` |
| `player_connected`, `player_disconnected` | INFO | `player`, `ip` |
| `command_received` | INFO | `player`, `cmd`, `args` |
| `response_sent` | DEBUG | `player`, `response` |
| `error_sent` | WARN | `player`, `error` |
| `event_broadcast` | INFO | `to`, `payload` |
| `player_moved`, `item_taken`, `item_dropped`, `items_dropped`, `item_used`, `npc_talk` | INFO | `player`, `room`, and the item, NPC or rooms involved |
| `combat_round`, `combat_fled` | INFO | `player`, `npc`, `room`, `dealt`, `taken`, `player_hp`, `npc_hp`, `status` |
| `quest_accepted`, `quest_progress`, `quest_completed` | INFO | `player`, `quest`, progress or rewards |
| `abuse_cmd_flood` | WARN | `player`, `ip`, `count_per_sec` (more than 20 commands in a second) |
| `abuse_rapid_reconnect` | WARN | `ip`, `attempts` (more than 5 connections in 10 seconds from one IP) |

To watch for problems, filter the output:

```bash
make run-server | grep '"level":"WARN"'           # errors sent to players + abuse
make run-server | grep -E 'abuse_|combat_round'   # floods, reconnect spam, fights
```

## Group Contributions

**khhammou**: the server (TCP listener, client goroutines, hub, router), logging and abuse detection, the game engine (movement, items, combat, quests), the web GUI and its WebSocket bridge, the Makefile, and the unit tests for those parts.

**ekarout**: the world (`world_data/world.yaml`: map, lore, NPCs, items, quests), the YAML loader and validator (`server/game/world.go`) with its tests, the terminal client (`cli_client/`), and the World Design part of this README.

## Building and Running

| Target | What it does |
|---|---|
| `make install` | downloads dependencies (`go mod download`) |
| `make build` | builds `tap-server`, `tap-client` and `tap-gui` |
| `make run-server` | runs the server on port 8080 |
| `make run-client` | runs the terminal client |
| `make run-client-gui` | runs the web client |
| `make test` | runs all unit tests |
| `make lint` | `gofmt` check + `go vet` |
| `make clean` | deletes the built binaries |

Run everything from the repository root, because the server loads `world_data/world.yaml` from there.

**CLI client.** We went with the simple option: what you type is sent to the server as-is, so you use the RFC commands directly. Server messages print as soon as they arrive, even while you're typing, coloured by type (`OK` cyan, `ERR` red, `EVT` green). `go run ./cli_client/ host:port` connects somewhere other than `localhost:8080`.

**GUI client.** `gui_client/main.go` serves the page on http://localhost:3000 and forwards WebSocket messages to the TCP server (flags: `-port`, `-server`, `-no-browser`). The page shows:

- the room (exits, items, NPCs), which refreshes on its own;
- your inventory with Take, Drop and Use buttons;
- buttons for every command, and a combat panel;
- player counts for your room and the server;
- chat (Global, Room and Group tabs), kept separate from the log;
- a box for typing raw commands, where you can use IDs or display names.

## Testing

`make test` runs the unit tests. `make lint` checks formatting and runs `go vet`.

**Multiplayer.** Start the server, then open two terminals with `nc localhost 8080` (or `make run-client`):

```text
A: CONNECT alice
B: CONNECT bob             A sees: EVT ROOM PRESENCE ENTER bob, EVT STATS players=2
A: CHAT ROOM hello         B sees: EVT ROOM CHAT alice hello
A: MOVE east               B sees: EVT ROOM PRESENCE LEAVE alice
A: WHO                     OK {"room":["alice"],"server":2}
A: GROUP CREATE
A: GROUP INVITE bob        B sees: EVT GROUP INVITE alice
B: GROUP JOIN alice        A sees: EVT GROUP JOIN bob
B: CHAT GROUP hi           A sees: EVT GROUP CHAT bob hi
```

Close B's terminal and A gets `EVT GROUP LEAVE bob` and `EVT STATS players=1`.

**Combat.**

```text
TAKE wooden sword       OK taken=wooden_sword
MOVE east               OK room=mushroom_grove
ATTACK corrupted wolf   OK {"attacker_hp":95,"target_hp":15,"damage":25,"status":"combat"}
DEFEND                  OK {"attacker_hp":93,"target_hp":3,"damage":12,"status":"combat"}
ATTACK                  OK {"attacker_hp":93,"target_hp":0,"damage":25,"status":"victory"}
```

Anyone else in the room sees `EVT ROOM COMBAT alice wolf ...`. To test dying, go fight the Crystal Guardian (north of the grove) without a sword. You'll respawn in the clearing with 50 HP.

**Quests.**

```text
MOVE north, MOVE east      (to the Ancient Elder)
QUEST spirit               OK {"quest_id":"defeat_wolves",...,"status":"active"}
(kill 2 wolves in the Mushroom Grove)
QUESTS                     OK [{"quest_id":"defeat_wolves","status":"active","progress":"2/2"}]
QUEST spirit               OK {...,"status":"completed","dialogue":"You have done well..."}
QUEST spirit               OK {"quest_id":"find_magic_crystal",...}
```

Then grab the Magic Crystal in the Crystal Cave and `QUEST spirit` again to hand it in for the Ancient Seed.
