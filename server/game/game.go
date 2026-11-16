package game

import (
	"encoding/json"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"

	"tap/server/logger"
)

const (
	ErrMissingArg = "ERR 100 MISSING_ARG"
	errNoNPC      = "ERR 404 NPC_NOT_FOUND"
	errNoCombat   = "ERR 400 NOT_IN_COMBAT"
	maxHP         = 100
	respawnHP     = 50
	baseDamage    = 10
	npcRespawn    = 60 * time.Second
)

type Event struct {
	To  []string
	Msg string
}

type Game struct {
	mu        sync.Mutex
	world     *World
	players   map[string]*PlayerState
	roomItems map[string][]string
	roomNPCs  map[string][]string
}

type lookRoom struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Exits       map[string]string `json:"exits"`
}

type lookOutput struct {
	Room    lookRoom `json:"room"`
	Players []string `json:"players"`
	Items   []string `json:"items"`
	NPCs    []string `json:"npcs"`
}

type talkOutput struct {
	NPC      string `json:"npc"`
	Dialogue string `json:"dialogue"`
}

type combatOutput struct {
	AttackerHP int    `json:"attacker_hp"`
	TargetHP   int    `json:"target_hp"`
	Damage     int    `json:"damage"`
	Status     string `json:"status"`
	Respawn    string `json:"respawn,omitempty"`
}

type statusOutput struct {
	HP       int    `json:"hp"`
	MaxHP    int    `json:"max_hp"`
	Status   string `json:"status"`
	InCombat bool   `json:"in_combat"`
	Gold     int    `json:"gold"`
	XP       int    `json:"xp"`
}

type questOutput struct {
	QuestID     string `json:"quest_id"`
	Description string `json:"description"`
	Reward      string `json:"reward"`
	Status      string `json:"status"`
	Dialogue    string `json:"dialogue,omitempty"`
}

type questListOutput struct {
	QuestID  string `json:"quest_id"`
	Status   string `json:"status"`
	Progress string `json:"progress"`
}

func NewGame(w *World) *Game {
	g := &Game{
		world:     w,
		players:   make(map[string]*PlayerState),
		roomItems: make(map[string][]string),
		roomNPCs:  make(map[string][]string),
	}
	for id, room := range w.Locations {
		g.roomItems[id] = slices.Clone(room.Items)
		for _, spawn := range room.Spawns {
			for range spawn.Count {
				g.roomNPCs[id] = append(g.roomNPCs[id], spawn.NPCType)
			}
		}
	}
	return g
}

func (g *Game) AddPlayer(name string) []Event {
	g.mu.Lock()
	defer g.mu.Unlock()
	room := g.world.StartingLoc
	g.players[name] = &PlayerState{
		RoomID:    room,
		HP:        maxHP,
		Inventory: []string{},
		Quests:    make(map[string]*QuestState),
	}
	return []Event{{g.playersIn(room, name), "EVT ROOM PRESENCE ENTER " + name}}
}

func (g *Game) RemovePlayer(name string) []Event {
	g.mu.Lock()
	defer g.mu.Unlock()
	p := g.players[name]
	delete(g.players, name)
	g.roomItems[p.RoomID] = append(g.roomItems[p.RoomID], p.Inventory...)
	logger.Log.Info("items_dropped", "player", name, "room", p.RoomID, "items", p.Inventory)
	return []Event{{g.playersIn(p.RoomID, ""), "EVT ROOM PRESENCE LEAVE " + name}}
}

func (g *Game) RoomMates(name string) []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.playersIn(g.players[name].RoomID, "")
}

func (g *Game) HandleCommand(name string, fields []string) (string, []Event) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p := g.players[name]
	cmd := strings.ToUpper(fields[0])
	arg := strings.Join(fields[1:], " ")
	if arg == "" && slices.Contains([]string{"MOVE", "TAKE", "DROP", "TALK", "QUEST", "USE"}, cmd) {
		return ErrMissingArg, nil
	}
	switch cmd {
	case "LOOK":
		return g.look(p), nil
	case "MOVE":
		return g.move(name, p, arg)
	case "TAKE":
		return g.take(name, p, arg), nil
	case "DROP":
		return g.drop(name, p, arg), nil
	case "INVENTORY":
		return okJSON(p.Inventory), nil
	case "TALK":
		return g.talk(name, p, arg), nil
	case "ATTACK":
		return g.attack(name, p, arg)
	case "DEFEND":
		return g.defend(name, p)
	case "USE":
		return g.use(name, p, arg)
	case "FLEE":
		return g.flee(name, p)
	case "STATUS":
		return g.status(p), nil
	case "QUEST":
		return g.quest(name, p, arg), nil
	case "QUESTS":
		return g.quests(p), nil
	}
	return "ERR 100 UNKNOWN_COMMAND", nil
}

func (g *Game) look(p *PlayerState) string {
	room := g.world.Locations[p.RoomID]
	return okJSON(lookOutput{
		Room:    lookRoom{p.RoomID, room.Name, room.Description, room.Exits},
		Players: g.playersIn(p.RoomID, ""),
		Items:   append([]string{}, g.roomItems[p.RoomID]...),
		NPCs:    append([]string{}, g.roomNPCs[p.RoomID]...),
	})
}

func (g *Game) move(name string, p *PlayerState, dir string) (string, []Event) {
	if p.CombatWith != "" {
		return "ERR 400 IN_COMBAT", nil
	}
	to, ok := g.world.Locations[p.RoomID].Exits[strings.ToLower(dir)]
	if !ok {
		return "ERR 301 NO_EXIT", nil
	}
	return "OK room=" + to, g.relocate(name, p, to)
}

func (g *Game) take(name string, p *PlayerState, target string) string {
	items := g.roomItems[p.RoomID]
	i := g.findItem(items, target)
	if i < 0 || !g.world.Items[items[i]].Obtainable {
		return "ERR 404 ITEM_NOT_FOUND"
	}
	id := items[i]
	g.roomItems[p.RoomID] = slices.Delete(items, i, i+1)
	p.Inventory = append(p.Inventory, id)
	logger.Log.Info("item_taken", "player", name, "item", id, "room", p.RoomID)
	return "OK taken=" + id
}

func (g *Game) drop(name string, p *PlayerState, target string) string {
	i := g.findItem(p.Inventory, target)
	if i < 0 {
		return "ERR 404 ITEM_NOT_IN_INVENTORY"
	}
	id := p.Inventory[i]
	p.Inventory = slices.Delete(p.Inventory, i, i+1)
	g.roomItems[p.RoomID] = append(g.roomItems[p.RoomID], id)
	logger.Log.Info("item_dropped", "player", name, "item", id, "room", p.RoomID)
	return "OK dropped=" + id
}

func (g *Game) talk(name string, p *PlayerState, target string) string {
	npcs := g.roomNPCs[p.RoomID]
	i := g.findNPC(npcs, target)
	if i < 0 {
		return errNoNPC
	}
	lines := g.world.NPCs[npcs[i]].Dialogue
	logger.Log.Info("npc_talk", "player", name, "npc", npcs[i], "room", p.RoomID)
	return okJSON(talkOutput{npcs[i], lines[rand.IntN(len(lines))]})
}

func (g *Game) attack(name string, p *PlayerState, target string) (string, []Event) {
	if p.CombatWith == "" {
		if target == "" {
			return ErrMissingArg, nil
		}
		npcs := g.roomNPCs[p.RoomID]
		i := g.findNPC(npcs, target)
		if i < 0 {
			return errNoNPC, nil
		}
		npc := g.world.NPCs[npcs[i]]
		if !npc.Stats.Hostile {
			return "ERR 405 NPC_NOT_HOSTILE", nil
		}
		p.CombatWith = npcs[i]
		p.TargetHP = npc.Stats.HP
	}
	return g.round(name, p, g.playerDamage(p), g.world.NPCs[p.CombatWith].Stats.Damage)
}

func (g *Game) defend(name string, p *PlayerState) (string, []Event) {
	if p.CombatWith == "" {
		return errNoCombat, nil
	}
	return g.round(name, p, g.playerDamage(p)/2, g.world.NPCs[p.CombatWith].Stats.Damage/2)
}

func (g *Game) use(name string, p *PlayerState, target string) (string, []Event) {
	i := g.findItem(p.Inventory, target)
	if i < 0 {
		return "ERR 404 ITEM_NOT_IN_INVENTORY", nil
	}
	id := p.Inventory[i]
	effect := g.world.Items[id].Effect
	if effect == nil || effect.Type != "heal" {
		return "ERR 400 CANNOT_USE", nil
	}
	p.Inventory = slices.Delete(p.Inventory, i, i+1)
	p.HP = min(p.HP+effect.Amount, maxHP)
	logger.Log.Info("item_used", "player", name, "item", id, "hp", p.HP)
	if p.CombatWith != "" {
		return g.round(name, p, 0, g.world.NPCs[p.CombatWith].Stats.Damage)
	}
	return g.status(p), nil
}

func (g *Game) flee(name string, p *PlayerState) (string, []Event) {
	if p.CombatWith == "" {
		return errNoCombat, nil
	}
	logger.Log.Info("combat_fled", "player", name, "npc", p.CombatWith, "room", p.RoomID)
	p.CombatWith = ""
	exits := slices.Collect(maps.Values(g.world.Locations[p.RoomID].Exits))
	to := exits[rand.IntN(len(exits))]
	return "OK room=" + to, g.relocate(name, p, to)
}

func (g *Game) round(name string, p *PlayerState, dealt, taken int) (string, []Event) {
	npc, room := p.CombatWith, p.RoomID
	p.TargetHP = max(p.TargetHP-dealt, 0)
	out := combatOutput{TargetHP: p.TargetHP, Damage: dealt, Status: "combat"}
	var events []Event
	switch {
	case p.TargetHP == 0:
		out.Status = "victory"
		g.slay(name, p)
	case p.HP <= taken:
		out.Status = "defeated"
		out.Respawn = g.world.StartingLoc
		p.CombatWith = ""
		p.HP = respawnHP
		events = g.relocate(name, p, g.world.StartingLoc)
	default:
		p.HP -= taken
	}
	out.AttackerHP = p.HP
	logger.Log.Info("combat_round", "player", name, "npc", npc, "room", room, "dealt", dealt, "taken", taken,
		"player_hp", p.HP, "npc_hp", p.TargetHP, "status", out.Status)
	events = append(events, Event{g.playersIn(room, name), fmt.Sprintf("EVT ROOM COMBAT %s %s %s", name, npc, out.Status)})
	return okJSON(out), events
}

func (g *Game) slay(name string, p *PlayerState) {
	npc, room := p.CombatWith, p.RoomID
	p.CombatWith = ""
	if i := slices.Index(g.roomNPCs[room], npc); i >= 0 {
		g.roomNPCs[room] = slices.Delete(g.roomNPCs[room], i, i+1)
		time.AfterFunc(npcRespawn, func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			g.roomNPCs[room] = append(g.roomNPCs[room], npc)
		})
	}
	for id, st := range p.Quests {
		obj := g.world.Quests[id].Objective
		if !st.Done && obj.Type == "defeat_npc" && obj.Item == npc {
			st.Kills++
			logger.Log.Info("quest_progress", "player", name, "quest", id, "progress", st.Kills, "needed", obj.Quantity)
		}
	}
}

func (g *Game) status(p *PlayerState) string {
	label := "healthy"
	if p.HP < 30 {
		label = "critical"
	} else if p.HP < 70 {
		label = "wounded"
	}
	return okJSON(statusOutput{p.HP, maxHP, label, p.CombatWith != "", p.Gold, p.XP})
}

func (g *Game) quest(name string, p *PlayerState, target string) string {
	npcs := g.roomNPCs[p.RoomID]
	i := g.findNPC(npcs, target)
	if i < 0 {
		return errNoNPC
	}
	for _, id := range slices.Sorted(maps.Keys(g.world.Quests)) {
		q := g.world.Quests[id]
		st := p.Quests[id]
		if q.GiverNPC != npcs[i] || (st != nil && st.Done) {
			continue
		}
		out := questOutput{QuestID: id, Description: q.Description, Reward: q.Reward.Item, Status: "active"}
		if st == nil {
			p.Quests[id] = &QuestState{}
			logger.Log.Info("quest_accepted", "player", name, "quest", id)
		} else if g.progress(p, id) >= q.Objective.Quantity {
			g.complete(name, p, id)
			out.Status = "completed"
			out.Dialogue = q.CompletionDialogue
		}
		return okJSON(out)
	}
	return "ERR 406 NO_QUEST_AVAILABLE"
}

func (g *Game) complete(name string, p *PlayerState, id string) {
	q := g.world.Quests[id]
	if q.Objective.Type == "find_item" {
		for range q.Objective.Quantity {
			i := slices.Index(p.Inventory, q.Objective.Item)
			p.Inventory = slices.Delete(p.Inventory, i, i+1)
		}
	}
	if q.Reward.Item != "" {
		p.Inventory = append(p.Inventory, q.Reward.Item)
	}
	p.Gold += q.Reward.Gold
	p.XP += q.Reward.XP
	p.Quests[id].Done = true
	logger.Log.Info("quest_completed", "player", name, "quest", id, "reward_item", q.Reward.Item,
		"gold", q.Reward.Gold, "xp", q.Reward.XP)
}

func (g *Game) progress(p *PlayerState, id string) int {
	obj := g.world.Quests[id].Objective
	if obj.Type == "defeat_npc" {
		return p.Quests[id].Kills
	}
	n := 0
	for _, item := range p.Inventory {
		if item == obj.Item {
			n++
		}
	}
	return n
}

func (g *Game) quests(p *PlayerState) string {
	list := []questListOutput{}
	for _, id := range slices.Sorted(maps.Keys(p.Quests)) {
		need := g.world.Quests[id].Objective.Quantity
		status, have := "active", min(g.progress(p, id), need)
		if p.Quests[id].Done {
			status, have = "completed", need
		}
		list = append(list, questListOutput{id, status, fmt.Sprintf("%d/%d", have, need)})
	}
	return okJSON(list)
}

func (g *Game) relocate(name string, p *PlayerState, to string) []Event {
	from := p.RoomID
	p.RoomID = to
	logger.Log.Info("player_moved", "player", name, "from", from, "to", to)
	return []Event{
		{g.playersIn(from, name), "EVT ROOM PRESENCE LEAVE " + name},
		{g.playersIn(to, name), "EVT ROOM PRESENCE ENTER " + name},
	}
}

func (g *Game) playersIn(room, except string) []string {
	names := []string{}
	for name, p := range g.players {
		if p.RoomID == room && name != except {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func (g *Game) playerDamage(p *PlayerState) int {
	bonus := 0
	for _, id := range p.Inventory {
		if e := g.world.Items[id].Effect; e != nil && e.Type == "damage" {
			bonus = max(bonus, e.Amount)
		}
	}
	return baseDamage + bonus
}

func (g *Game) findItem(ids []string, target string) int {
	return slices.IndexFunc(ids, func(id string) bool {
		return strings.EqualFold(id, target) || strings.EqualFold(g.world.Items[id].Name, target)
	})
}

func (g *Game) findNPC(ids []string, target string) int {
	return slices.IndexFunc(ids, func(id string) bool {
		return strings.EqualFold(id, target) || strings.EqualFold(g.world.NPCs[id].Name, target)
	})
}

func okJSON(v any) string {
	data, _ := json.Marshal(v)
	return "OK " + string(data)
}
