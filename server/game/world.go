package game

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadWorld(path string) (*World, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read world file: %w", err)
	}
	var worldFile WorldFile
	if err := yaml.Unmarshal(content, &worldFile); err != nil {
		return nil, fmt.Errorf("failed to parse world YAML: %w", err)
	}
	return &worldFile.World, nil
}

func ValidateWorld(w *World) error {
	if _, ok := w.Locations[w.StartingLoc]; !ok {
		return fmt.Errorf("starting location %s not listed in locations", w.StartingLoc)
	}
	for name, room := range w.Locations {
		if err := validateRoom(w, name, room); err != nil {
			return err
		}
	}
	for name, npc := range w.NPCs {
		if len(npc.Dialogue) == 0 {
			return fmt.Errorf("npc %s has no dialogue", name)
		}
	}
	for name, quest := range w.Quests {
		if err := validateQuest(w, name, quest); err != nil {
			return err
		}
	}
	return nil
}

func validateRoom(w *World, name string, room Room) error {
	if len(room.Exits) == 0 {
		return fmt.Errorf("room %s has no exits", name)
	}
	for direction, destination := range room.Exits {
		if _, ok := w.Locations[destination]; !ok {
			return fmt.Errorf("room %s has an invalid %s exit pointing to %s", name, direction, destination)
		}
	}
	for _, item := range room.Items {
		if _, ok := w.Items[item]; !ok {
			return fmt.Errorf("item %s in %s not listed in items", item, name)
		}
	}
	for _, spawn := range room.Spawns {
		if _, ok := w.NPCs[spawn.NPCType]; !ok {
			return fmt.Errorf("spawn %s in room %s not listed in npcs", spawn.NPCType, name)
		}
	}
	return nil
}

func validateQuest(w *World, name string, quest Quest) error {
	if _, ok := w.NPCs[quest.GiverNPC]; !ok {
		return fmt.Errorf("quest %s giver %s not listed in npcs", name, quest.GiverNPC)
	}
	_, isNPC := w.NPCs[quest.Objective.Item]
	_, isItem := w.Items[quest.Objective.Item]
	switch {
	case quest.Objective.Type == "defeat_npc" && !isNPC:
		return fmt.Errorf("quest %s targets unknown npc %s", name, quest.Objective.Item)
	case quest.Objective.Type == "find_item" && !isItem:
		return fmt.Errorf("quest %s targets unknown item %s", name, quest.Objective.Item)
	case quest.Objective.Type != "defeat_npc" && quest.Objective.Type != "find_item":
		return fmt.Errorf("quest %s has unknown objective type %s", name, quest.Objective.Type)
	case quest.Objective.Quantity < 1:
		return fmt.Errorf("quest %s needs a quantity of at least 1", name)
	}
	if _, ok := w.Items[quest.Reward.Item]; quest.Reward.Item != "" && !ok {
		return fmt.Errorf("quest %s rewards unknown item %s", name, quest.Reward.Item)
	}
	return nil
}
