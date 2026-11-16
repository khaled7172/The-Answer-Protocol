package game

import "testing"

func loadTestWorld(t *testing.T) *World {
	t.Helper()
	world, err := LoadWorld("../../world_data/world.yaml")
	if err != nil {
		t.Fatalf("failed to load world: %v", err)
	}
	return world
}

func TestLoadWorld(t *testing.T) {
	world := loadTestWorld(t)

	room, exists := world.Locations["forest_clearing"]
	if !exists {
		t.Fatal("forest_clearing was not loaded")
	}
	if room.Name != "The Whispering Clearing" {
		t.Errorf("unexpected room name: %s", room.Name)
	}

	item, exists := world.Items["wooden_sword"]
	if !exists {
		t.Fatal("wooden_sword was not loaded")
	}
	if item.Effect == nil || item.Effect.Amount != 15 {
		t.Fatalf("expected wooden_sword damage effect of 15, got %+v", item.Effect)
	}

	if wolf, ok := world.NPCs["wolf"]; !ok || !wolf.Stats.Hostile || wolf.Stats.HP != 40 {
		t.Fatalf("wolf NPC not loaded with stats: %+v", wolf)
	}
	if _, ok := world.Quests["defeat_wolves"]; !ok {
		t.Fatal("defeat_wolves quest was not loaded")
	}
}

func TestLoadWorldMissingFile(t *testing.T) {
	if _, err := LoadWorld("nonexistent.yaml"); err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}

func TestValidateWorld(t *testing.T) {
	if err := ValidateWorld(loadTestWorld(t)); err != nil {
		t.Fatalf("expected valid world, got error: %v", err)
	}
}

func TestValidateWorldRejectsBrokenReferences(t *testing.T) {
	cases := map[string]func(w *World){
		"starting location": func(w *World) { w.StartingLoc = "unknown_room" },
		"exit": func(w *World) {
			w.Locations["forest_clearing"].Exits["north"] = "unknown_room"
		},
		"room item": func(w *World) {
			room := w.Locations["forest_clearing"]
			room.Items = append(room.Items, "unknown_item")
			w.Locations["forest_clearing"] = room
		},
		"spawn": func(w *World) {
			room := w.Locations["forest_clearing"]
			room.Spawns = append(room.Spawns, RoomSpawn{NPCType: "unknown_npc", Count: 1})
			w.Locations["forest_clearing"] = room
		},
		"no exits": func(w *World) {
			room := w.Locations["forest_clearing"]
			room.Exits = nil
			w.Locations["forest_clearing"] = room
		},
		"npc without dialogue": func(w *World) {
			npc := w.NPCs["wolf"]
			npc.Dialogue = nil
			w.NPCs["wolf"] = npc
		},
		"quest giver": func(w *World) {
			quest := w.Quests["defeat_wolves"]
			quest.GiverNPC = "unknown_npc"
			w.Quests["defeat_wolves"] = quest
		},
		"quest target": func(w *World) {
			quest := w.Quests["defeat_wolves"]
			quest.Objective.Item = "unknown_npc"
			w.Quests["defeat_wolves"] = quest
		},
		"objective type": func(w *World) {
			quest := w.Quests["defeat_wolves"]
			quest.Objective.Type = "dance"
			w.Quests["defeat_wolves"] = quest
		},
		"reward item": func(w *World) {
			quest := w.Quests["defeat_wolves"]
			quest.Reward.Item = "unknown_item"
			w.Quests["defeat_wolves"] = quest
		},
	}
	for name, breakWorld := range cases {
		t.Run(name, func(t *testing.T) {
			world := loadTestWorld(t)
			breakWorld(world)
			if err := ValidateWorld(world); err == nil {
				t.Fatalf("expected error for invalid %s", name)
			}
		})
	}
}
