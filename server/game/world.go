package game

import(
	"os"
	"fmt"
	"gopkg.in/yaml.v3"
)

func LoadWorld(path string) (*World, error){
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read world file: %w", err)
	}
	var worldFile WorldFile
	err = yaml.Unmarshal(content, &worldFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse world YAML: %w", err)
	}
	return &worldFile.World, nil
}

func ValidateWorld(w *World) error {
	// Starting Location
	_, exists := w.Locations[w.StartingLoc]
	if !exists {
		return fmt.Errorf(
			"starting location: %s not listed in locations",
			w.StartingLoc)
	}
	// Exits
	for roomName, room := range w.Locations {
		for direction, destination := range room.Exits {
			_, exists = w.Locations[destination]
			if !exists {
				return fmt.Errorf(
					"room %s has an invalid %s exit pointing to %s",
					roomName, direction, destination)
			}
		}
	}
	// Items
	for roomName, room := range w.Locations {
		for _, item := range room.Items {
			_, exists = w.Items[item]
			if !exists {
				return fmt.Errorf(
					"item %s in %s not listed in items",
					item, roomName)
			}
		}
	}
	// Spawns
	for roomName, room := range w.Locations {
		for _, spawn := range room.Spawns {
			_, exists = w.NPCs[spawn.NPCType]
			if !exists {
				return fmt.Errorf(
					"Spawn %s in room %s not listed in NPCs",
					spawn.NPCType, roomName)
			}
		}
	}
	// Quests
	for questName, quest := range w.Quests {
		_, exists = w.NPCs[quest.GiverNPC]
		if !exists {
			return fmt.Errorf(
				"NPC %s quest giver of %s not listed in NPCs",
				quest.GiverNPC, questName)
		}
	}
	return nil
}