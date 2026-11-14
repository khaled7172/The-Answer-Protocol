package game

import "testing"

func TestLoadWorld(t *testing.T) {
	world, err := LoadWorld("../../world_data/world.yaml")

	if err != nil {
		t.Fatalf("expected world to load successfully, got error: %v", err)
	}

	if world == nil {
		t.Fatal("expected a world, got nil")
	}

	// Check locations
	room, exists := world.Locations["forest_clearing"]
	if !exists {
		t.Fatal("forest_clearing was not loaded")
	}

	if room.Name != "The Whispering Clearing" {
		t.Errorf("unexpected room name: %s", room.Name)
	}

	// Check items
	item, exists := world.Items["wooden_sword"]
	if !exists {
		t.Fatal("wooden_sword was not loaded")
	}

	if item.Effect == nil {
		t.Fatal("expected wooden_sword to have an effect")
	}

	if item.Effect.Amount != 15 {
		t.Errorf("expected damage amount 15, got %d", item.Effect.Amount)
	}

	// Check NPCs
	_, exists = world.NPCs["wolf"]
	if !exists {
		t.Fatal("wolf NPC was not loaded")
	}

	// Check quests
	_, exists = world.Quests["defeat_wolves"]
	if !exists {
		t.Fatal("defeat_wolves quest was not loaded")
	}
}

func TestLoadWorldMissingFile(t *testing.T) {
	_, err := LoadWorld("nonexistent.yaml")

	if err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}

// ValidateWorld tests

func TestValidateWorld(t *testing.T) {
	world, err := LoadWorld("../../world_data/world.yaml")
	if err != nil {
		t.Fatalf("failed to load world: %v", err)
	}

	err = ValidateWorld(world)
	if err != nil {
		t.Fatalf("expected valid world, got error: %v", err)
	}
}

func TestValidateWorldInvalidStartingLocation(t *testing.T) {
	world, err := LoadWorld("../../world_data/world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	world.StartingLoc = "unknown_room"

	err = ValidateWorld(world)
	if err == nil {
		t.Fatal("expected error for invalid starting location")
	}
}

func TestValidateWorldInvalidExit(t *testing.T) {
	world, err := LoadWorld("../../world_data/world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	room := world.Locations["forest_clearing"]
	room.Exits["north"] = "unknown_room"
	world.Locations["forest_clearing"] = room

	err = ValidateWorld(world)
	if err == nil {
		t.Fatal("expected error for invalid exit")
	}
}

func TestValidateWorldInvalidItem(t *testing.T) {
	world, err := LoadWorld("../../world_data/world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	room := world.Locations["forest_clearing"]
	room.Items = append(room.Items, "unknown_item")
	world.Locations["forest_clearing"] = room

	err = ValidateWorld(world)
	if err == nil {
		t.Fatal("expected error for invalid item")
	}
}

func TestValidateWorldInvalidSpawn(t *testing.T) {
	world, err := LoadWorld("../../world_data/world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	room := world.Locations["forest_clearing"]
	room.Spawns = append(room.Spawns, RoomSpawn{
		NPCType: "unknown_npc",
		Count:   1,
	})
	world.Locations["forest_clearing"] = room

	err = ValidateWorld(world)
	if err == nil {
		t.Fatal("expected error for invalid NPC spawn")
	}
}

func TestValidateWorldInvalidQuestGiver(t *testing.T) {
	world, err := LoadWorld("../../world_data/world.yaml")
	if err != nil {
		t.Fatal(err)
	}

	quest := world.Quests["defeat_wolves"]
	quest.GiverNPC = "unknown_npc"
	world.Quests["defeat_wolves"] = quest

	err = ValidateWorld(world)
	if err == nil {
		t.Fatal("expected error for invalid quest giver")
	}
}
