package game

type WorldFile struct {
	World World `yaml:"world"`
}

type World struct {
	StartingLoc	string			 `yaml:"starting_location"`
	Locations 	map[string]Room  `yaml:"locations"`
	Items		map[string]Item  `yaml:"items"`
	NPCs      	map[string]NPC   `yaml:"npcs"`
	Quests    	map[string]Quest `yaml:"quests"`
}

type Room struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Exits       map[string]string `yaml:"exits"`
	Items       []string          `yaml:"items"`
	Spawns      []RoomSpawn       `yaml:"spawns"`
}

type RoomSpawn struct {
	NPCType string `yaml:"npc_type"`
	Count   int    `yaml:"count"`
}

type Item struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Obtainable  bool    `yaml:"obtainable"`
	Effect      *Effect `yaml:"effect"`
}

type Effect struct {
	Type   string `yaml:"type"`
	Amount int   `yaml:"amount"`
}

type NPC struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Dialogue    []string   `yaml:"dialogue"`
	Stats       SpawnStats `yaml:"stats"`
}

type SpawnStats struct {
	HP      int  `yaml:"hp"`
	Damage  int  `yaml:"damage"`
	Hostile bool `yaml:"hostile"`
}

type Quest struct {
	Name               string         `yaml:"name"`
	Description        string         `yaml:"description"`
	GiverNPC           string         `yaml:"giver_npc"`
	Objective          QuestObjective `yaml:"objective"`
	Reward             QuestReward    `yaml:"reward"`
	CompletionDialogue string         `yaml:"completion_dialogue"`
}

type QuestObjective struct {
	Type     string `yaml:"type"`
	Item     string `yaml:"item"`
	Quantity int    `yaml:"quantity"`
}

type QuestReward struct {
	Gold int    `yaml:"gold"`
	Item string `yaml:"item"`
	XP   int    `yaml:"xp"`
}
