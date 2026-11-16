package game

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"tap/server/logger"
)

func init() {
	logger.Init()
}

func testWorld() *World {
	return &World{
		StartingLoc: "square",
		Locations: map[string]Room{
			"square": {Name: "Town Square", Exits: map[string]string{"north": "tavern"},
				Items: []string{"stick"}, Spawns: []RoomSpawn{{NPCType: "baker", Count: 1}}},
			"tavern": {Name: "Tavern", Exits: map[string]string{"south": "square", "down": "cellar"},
				Items: []string{"potion", "statue", "crystal"}, Spawns: []RoomSpawn{{NPCType: "innkeeper", Count: 1}}},
			"cellar": {Name: "Cellar", Exits: map[string]string{"up": "tavern"},
				Spawns: []RoomSpawn{{NPCType: "rat", Count: 2}}},
		},
		Items: map[string]Item{
			"stick":   {Name: "Stick", Obtainable: true},
			"potion":  {Name: "Health Potion", Obtainable: true, Effect: &Effect{Type: "heal", Amount: 20}},
			"statue":  {Name: "Heavy Stone Statue"},
			"crystal": {Name: "Magic Crystal", Obtainable: true},
			"sword":   {Name: "Iron Sword", Obtainable: true, Effect: &Effect{Type: "damage", Amount: 15}},
		},
		NPCs: map[string]NPC{
			"baker":     {Name: "Baker Bob", Dialogue: []string{"Fresh bread today!"}},
			"innkeeper": {Name: "Innkeeper Barnaby", Dialogue: []string{"Clear my cellar!"}},
			"rat":       {Name: "Giant Rat", Dialogue: []string{"Squeak!"}, Stats: SpawnStats{HP: 20, Damage: 10, Hostile: true}},
			"dragon":    {Name: "Mighty Dragon", Dialogue: []string{"Roar!"}, Stats: SpawnStats{HP: 300, Damage: 150, Hostile: true}},
		},
		Quests: map[string]Quest{
			"clear_cellar": {Description: "Kill a rat", GiverNPC: "innkeeper",
				Objective: QuestObjective{Type: "defeat_npc", Item: "rat", Quantity: 1},
				Reward:    QuestReward{Gold: 50, Item: "sword", XP: 10}, CompletionDialogue: "Thanks!"},
			"find_crystal": {Description: "Bring a crystal", GiverNPC: "innkeeper",
				Objective: QuestObjective{Type: "find_item", Item: "crystal", Quantity: 1},
				Reward:    QuestReward{Item: "potion"}},
		},
	}
}

func newTestGame(t *testing.T, room string, names ...string) *Game {
	t.Helper()
	g := NewGame(testWorld())
	for _, n := range names {
		g.AddPlayer(n)
		g.players[n].RoomID = room
	}
	return g
}

func run(g *Game, name, line string) (string, []Event) {
	return g.HandleCommand(name, strings.Fields(line))
}

func decode[T any](t *testing.T, resp string) T {
	t.Helper()
	var out T
	if !strings.HasPrefix(resp, "OK ") {
		t.Fatalf("expected OK response, got %q", resp)
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(resp, "OK ")), &out); err != nil {
		t.Fatalf("bad json in %q: %v", resp, err)
	}
	return out
}

func hasEvent(events []Event, to, msg string) bool {
	return slices.ContainsFunc(events, func(e Event) bool {
		return e.Msg == msg && slices.Contains(e.To, to)
	})
}

func TestNewGameSpawnsItemsAndNPCs(t *testing.T) {
	g := NewGame(testWorld())
	if !slices.Equal(g.roomItems["square"], []string{"stick"}) {
		t.Fatalf("unexpected square items: %v", g.roomItems["square"])
	}
	if !slices.Equal(g.roomNPCs["cellar"], []string{"rat", "rat"}) {
		t.Fatalf("expected two rats in cellar, got %v", g.roomNPCs["cellar"])
	}
}

func TestAddPlayerStartsAtStartingLocationAndAnnounces(t *testing.T) {
	g := NewGame(testWorld())
	g.AddPlayer("bob")
	events := g.AddPlayer("alice")
	if g.players["alice"].RoomID != "square" || g.players["alice"].HP != 100 {
		t.Fatalf("unexpected new player state: %+v", g.players["alice"])
	}
	if !hasEvent(events, "bob", "EVT ROOM PRESENCE ENTER alice") || hasEvent(events, "alice", "EVT ROOM PRESENCE ENTER alice") {
		t.Fatalf("expected enter event for bob only, got %+v", events)
	}
}

func TestRemovePlayerDropsInventoryAndAnnouncesLeave(t *testing.T) {
	g := newTestGame(t, "square", "alice", "bob")
	run(g, "alice", "TAKE stick")
	events := g.RemovePlayer("alice")
	if _, ok := g.players["alice"]; ok {
		t.Fatal("alice state not removed")
	}
	if !slices.Contains(g.roomItems["square"], "stick") {
		t.Fatalf("expected stick back on the floor, got %v", g.roomItems["square"])
	}
	if !hasEvent(events, "bob", "EVT ROOM PRESENCE LEAVE alice") {
		t.Fatalf("expected leave event for bob, got %+v", events)
	}
}

func TestLook(t *testing.T) {
	g := newTestGame(t, "square", "bob", "alice")
	resp, _ := run(g, "alice", "LOOK")
	out := decode[lookOutput](t, resp)
	if out.Room.ID != "square" || out.Room.Exits["north"] != "tavern" {
		t.Fatalf("unexpected room: %+v", out.Room)
	}
	if !slices.Equal(out.Players, []string{"alice", "bob"}) {
		t.Fatalf("expected [alice bob], got %v", out.Players)
	}
	if !slices.Equal(out.Items, []string{"stick"}) || !slices.Equal(out.NPCs, []string{"baker"}) {
		t.Fatalf("unexpected items/npcs: %v %v", out.Items, out.NPCs)
	}
}

func TestMove(t *testing.T) {
	g := newTestGame(t, "square", "alice", "bob")
	g.AddPlayer("carol")
	g.players["carol"].RoomID = "tavern"

	if resp, _ := run(g, "alice", "MOVE"); resp != ErrMissingArg {
		t.Fatalf("expected missing arg, got %q", resp)
	}
	if resp, _ := run(g, "alice", "MOVE east"); resp != "ERR 301 NO_EXIT" {
		t.Fatalf("expected no exit, got %q", resp)
	}
	resp, events := run(g, "alice", "MOVE NORTH")
	if resp != "OK room=tavern" || g.players["alice"].RoomID != "tavern" {
		t.Fatalf("expected move to tavern, got %q", resp)
	}
	if !hasEvent(events, "bob", "EVT ROOM PRESENCE LEAVE alice") || !hasEvent(events, "carol", "EVT ROOM PRESENCE ENTER alice") {
		t.Fatalf("missing presence events: %+v", events)
	}
}

func TestTakeAndDropByIDOrDisplayName(t *testing.T) {
	g := newTestGame(t, "tavern", "alice")
	if resp, _ := run(g, "alice", "TAKE nothing"); resp != "ERR 404 ITEM_NOT_FOUND" {
		t.Fatalf("expected item not found, got %q", resp)
	}
	if resp, _ := run(g, "alice", "TAKE heavy stone statue"); resp != "ERR 404 ITEM_NOT_FOUND" {
		t.Fatalf("expected unobtainable statue to be refused, got %q", resp)
	}
	if resp, _ := run(g, "alice", "TAKE health potion"); resp != "OK taken=potion" {
		t.Fatalf("expected take by multi-word name, got %q", resp)
	}
	if resp, _ := run(g, "alice", "TAKE potion"); resp != "ERR 404 ITEM_NOT_FOUND" {
		t.Fatalf("expected potion to be gone from the room, got %q", resp)
	}
	if resp, _ := run(g, "alice", "INVENTORY"); resp != `OK ["potion"]` {
		t.Fatalf("unexpected inventory %q", resp)
	}
	if resp, _ := run(g, "alice", "DROP stick"); resp != "ERR 404 ITEM_NOT_IN_INVENTORY" {
		t.Fatalf("expected not in inventory, got %q", resp)
	}
	if resp, _ := run(g, "alice", "DROP potion"); resp != "OK dropped=potion" {
		t.Fatalf("expected drop, got %q", resp)
	}
	if resp, _ := run(g, "alice", "INVENTORY"); resp != "OK []" {
		t.Fatalf("expected empty inventory, got %q", resp)
	}
}

func TestTalk(t *testing.T) {
	g := newTestGame(t, "square", "alice")
	if resp, _ := run(g, "alice", "TALK ghost"); resp != errNoNPC {
		t.Fatalf("expected npc not found, got %q", resp)
	}
	resp, _ := run(g, "alice", "TALK baker bob")
	if out := decode[talkOutput](t, resp); out.NPC != "baker" || out.Dialogue != "Fresh bread today!" {
		t.Fatalf("unexpected talk output %+v", out)
	}
}

func TestAttackErrors(t *testing.T) {
	g := newTestGame(t, "square", "alice")
	if resp, _ := run(g, "alice", "ATTACK"); resp != ErrMissingArg {
		t.Fatalf("expected missing arg, got %q", resp)
	}
	if resp, _ := run(g, "alice", "ATTACK rat"); resp != errNoNPC {
		t.Fatalf("expected npc not found, got %q", resp)
	}
	if resp, _ := run(g, "alice", "ATTACK baker"); resp != "ERR 405 NPC_NOT_HOSTILE" {
		t.Fatalf("expected not hostile, got %q", resp)
	}
}

func TestCombatVictoryBroadcastsAndCountsQuestKill(t *testing.T) {
	g := newTestGame(t, "tavern", "alice", "bob")
	run(g, "alice", "QUEST innkeeper")
	g.players["alice"].RoomID = "cellar"
	g.players["bob"].RoomID = "cellar"

	resp, events := run(g, "alice", "ATTACK giant rat")
	out := decode[combatOutput](t, resp)
	if out.Status != "combat" || out.TargetHP != 10 || out.AttackerHP != 90 || out.Damage != 10 {
		t.Fatalf("unexpected first round %+v", out)
	}
	if !hasEvent(events, "bob", "EVT ROOM COMBAT alice rat combat") {
		t.Fatalf("expected combat event for bob, got %+v", events)
	}
	if resp, _ := run(g, "alice", "MOVE up"); resp != "ERR 400 IN_COMBAT" {
		t.Fatalf("expected move blocked in combat, got %q", resp)
	}

	resp, events = run(g, "alice", "ATTACK")
	if out := decode[combatOutput](t, resp); out.Status != "victory" || out.TargetHP != 0 || out.AttackerHP != 90 {
		t.Fatalf("unexpected victory round %+v", out)
	}
	if !hasEvent(events, "bob", "EVT ROOM COMBAT alice rat victory") {
		t.Fatalf("expected victory event for bob, got %+v", events)
	}
	if !slices.Equal(g.roomNPCs["cellar"], []string{"rat"}) {
		t.Fatalf("expected one rat left, got %v", g.roomNPCs["cellar"])
	}
	if g.players["alice"].Quests["clear_cellar"].Kills != 1 {
		t.Fatal("kill not counted toward quest")
	}
}

func TestCombatDefeatRespawnsAtStart(t *testing.T) {
	g := newTestGame(t, "cellar", "alice")
	g.roomNPCs["cellar"] = append(g.roomNPCs["cellar"], "dragon")

	resp, events := run(g, "alice", "ATTACK dragon")
	out := decode[combatOutput](t, resp)
	if out.Status != "defeated" || out.AttackerHP != 50 || out.Respawn != "square" {
		t.Fatalf("unexpected defeat output %+v", out)
	}
	p := g.players["alice"]
	if p.RoomID != "square" || p.HP != 50 || p.CombatWith != "" {
		t.Fatalf("unexpected state after defeat %+v", p)
	}
	if len(events) != 3 {
		t.Fatalf("expected leave, enter and combat events, got %+v", events)
	}
}

func TestDefendCostsTurnAndHalvesDamage(t *testing.T) {
	g := newTestGame(t, "cellar", "alice")
	if resp, _ := run(g, "alice", "DEFEND"); resp != errNoCombat {
		t.Fatalf("expected not in combat, got %q", resp)
	}
	run(g, "alice", "ATTACK rat")
	resp, _ := run(g, "alice", "DEFEND")
	if out := decode[combatOutput](t, resp); out.AttackerHP != 85 || out.TargetHP != 5 || out.Damage != 5 {
		t.Fatalf("expected half damage both ways, got %+v", out)
	}
}

func TestUseHealsAndConsumes(t *testing.T) {
	g := newTestGame(t, "square", "alice")
	run(g, "alice", "TAKE stick")
	if resp, _ := run(g, "alice", "USE stick"); resp != "ERR 400 CANNOT_USE" {
		t.Fatalf("expected cannot use, got %q", resp)
	}
	g.players["alice"].RoomID = "tavern"
	run(g, "alice", "TAKE potion")
	g.players["alice"].HP = 50
	resp, _ := run(g, "alice", "USE health potion")
	if out := decode[statusOutput](t, resp); out.HP != 70 {
		t.Fatalf("expected 70 hp after potion, got %+v", out)
	}
	if slices.Contains(g.players["alice"].Inventory, "potion") {
		t.Fatal("potion not consumed")
	}
}

func TestWeaponAddsDamage(t *testing.T) {
	g := newTestGame(t, "cellar", "alice")
	g.players["alice"].Inventory = []string{"sword"}
	resp, _ := run(g, "alice", "ATTACK rat")
	if out := decode[combatOutput](t, resp); out.Damage != 25 || out.Status != "victory" {
		t.Fatalf("expected 25 damage kill with sword, got %+v", out)
	}
}

func TestFlee(t *testing.T) {
	g := newTestGame(t, "cellar", "alice")
	if resp, _ := run(g, "alice", "FLEE"); resp != errNoCombat {
		t.Fatalf("expected not in combat, got %q", resp)
	}
	run(g, "alice", "ATTACK rat")
	resp, _ := run(g, "alice", "FLEE")
	if resp != "OK room=tavern" || g.players["alice"].CombatWith != "" {
		t.Fatalf("expected flee to tavern, got %q", resp)
	}
}

func TestQuestChainHandInAndRewards(t *testing.T) {
	g := newTestGame(t, "square", "alice")
	if resp, _ := run(g, "alice", "QUEST baker"); resp != "ERR 406 NO_QUEST_AVAILABLE" {
		t.Fatalf("expected no quest from baker, got %q", resp)
	}
	g.players["alice"].RoomID = "tavern"

	resp, _ := run(g, "alice", "QUEST innkeeper")
	if out := decode[questOutput](t, resp); out.QuestID != "clear_cellar" || out.Status != "active" {
		t.Fatalf("expected clear_cellar accepted, got %+v", out)
	}
	resp, _ = run(g, "alice", "QUEST innkeeper")
	if out := decode[questOutput](t, resp); out.Status != "active" {
		t.Fatalf("expected quest still active before objective, got %+v", out)
	}

	g.players["alice"].Quests["clear_cellar"].Kills = 1
	resp, _ = run(g, "alice", "QUEST innkeeper")
	if out := decode[questOutput](t, resp); out.Status != "completed" || out.Dialogue != "Thanks!" {
		t.Fatalf("expected completion, got %+v", out)
	}
	p := g.players["alice"]
	if !slices.Contains(p.Inventory, "sword") || p.Gold != 50 || p.XP != 10 {
		t.Fatalf("rewards not granted: %+v", p)
	}

	resp, _ = run(g, "alice", "QUEST innkeeper")
	if out := decode[questOutput](t, resp); out.QuestID != "find_crystal" {
		t.Fatalf("expected next quest in chain, got %+v", out)
	}
	run(g, "alice", "TAKE magic crystal")
	resp, _ = run(g, "alice", "QUESTS")
	list := decode[[]questListOutput](t, resp)
	if len(list) != 2 || list[0].Status != "completed" || list[1].Progress != "1/1" {
		t.Fatalf("unexpected quest list %+v", list)
	}
	resp, _ = run(g, "alice", "QUEST innkeeper")
	if out := decode[questOutput](t, resp); out.Status != "completed" {
		t.Fatalf("expected crystal hand-in, got %+v", out)
	}
	if slices.Contains(p.Inventory, "crystal") || !slices.Contains(p.Inventory, "potion") {
		t.Fatalf("expected crystal delivered and potion rewarded, got %v", p.Inventory)
	}
	if resp, _ := run(g, "alice", "QUEST innkeeper"); resp != "ERR 406 NO_QUEST_AVAILABLE" {
		t.Fatalf("expected no quests left, got %q", resp)
	}
}

func TestStatus(t *testing.T) {
	g := newTestGame(t, "square", "alice")
	resp, _ := run(g, "alice", "STATUS")
	if out := decode[statusOutput](t, resp); out.HP != 100 || out.MaxHP != 100 || out.Status != "healthy" || out.InCombat {
		t.Fatalf("unexpected status %+v", out)
	}
}

func TestUnknownCommand(t *testing.T) {
	g := newTestGame(t, "square", "alice")
	if resp, _ := run(g, "alice", "DANCE"); resp != "ERR 100 UNKNOWN_COMMAND" {
		t.Fatalf("expected unknown command, got %q", resp)
	}
}
