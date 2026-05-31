package contracts

import "time"

const SchemaVersion = 1

type Envelope struct {
	SchemaVersion int       `json:"schemaVersion"`
	EventID       string    `json:"eventId"`
	EventType     string    `json:"eventType"`
	OccurredAt    time.Time `json:"occurredAt"`
	Actor         Actor     `json:"actor,omitempty"`
	Context       Context   `json:"context,omitempty"`
	Payload       any       `json:"payload"`
}

type Actor struct {
	CharacterID   string `json:"characterId,omitempty"`
	CharacterName string `json:"characterName,omitempty"`
}

type Context struct {
	Topic               string `json:"topic,omitempty"`
	LocationID          string `json:"locationId,omitempty"`
	CurrentMap          string `json:"currentMap,omitempty"`
	GuildID             string `json:"guildId,omitempty"`
	GuildName           string `json:"guildName,omitempty"`
	GameServerIP        string `json:"gameServerIp,omitempty"`
	AODataServerID      int    `json:"aoDataServerId,omitempty"`
	AODataIngestBaseURL string `json:"aoDataIngestBaseUrl,omitempty"`
}

type JoinStatePayload struct {
	CharacterID   string `json:"characterId,omitempty"`
	CharacterName string `json:"characterName,omitempty"`
	GuildID       string `json:"guildId,omitempty"`
	GuildName     string `json:"guildName,omitempty"`
	LocationID    string `json:"locationId,omitempty"`
}

type TradeParty struct {
	Name      string      `json:"name"`
	GuildName string      `json:"guildName,omitempty"`
	Silver    int64       `json:"silver"`
	Total     int64       `json:"total"`
	Items     []TradeItem `json:"items"`
}

type TradeItem struct {
	ItemID     int    `json:"itemId"`
	ItemName   string `json:"itemName"`
	Amount     int    `json:"amount"`
	UnitPrice  int64  `json:"unitPrice"`
	TotalPrice int64  `json:"totalPrice"`
}

type TradeCompletedPayload struct {
	SessionID   string     `json:"sessionId"`
	Location    string     `json:"location"`
	CompletedAt time.Time  `json:"completedAt"`
	LocalParty  TradeParty `json:"localParty"`
	RemoteParty TradeParty `json:"remoteParty"`
	NetProfit   int64      `json:"netProfit"`
}
