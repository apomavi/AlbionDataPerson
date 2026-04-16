package custom

import (
	"fmt"
	"strings"

	"github.com/ao-data/albiondata-client/custom/bridge"
	"github.com/ao-data/albiondata-client/log"
)

var (
	MyCharacterName = "SEN (Local)"
	MyGuildName     = "Loncasiz"
	CurrentMap      = "Bilinmeyen Konum"
)

func onJoinStateUpdated(data bridge.JoinStateData) {
	if data.CharacterName != "" {
		MyCharacterName = data.CharacterName
	}
	if data.GuildName != "" && data.GuildName != "[]" {
		MyGuildName = data.GuildName
	}
	if data.LocationID != "" {
		CurrentMap = data.LocationID
	}

	log.Infof("[SYSTEM CONNECTED] Player: %s | Guild: %s | Location: %s", MyCharacterName, MyGuildName, CurrentMap)
}

func onDecodedResponse(code uint16, params map[uint8]interface{}) {
	if code != 155 {
		return
	}

	tradeID := fmt.Sprintf("%v", params[6])
	if tradeID == "" || tradeID == "<nil>" {
		return
	}

	p1Name := MyCharacterName
	p2Name := fmt.Sprintf("%v", params[1])
	p2Guild := fmt.Sprintf("%v", params[2])

	if activeTrades[tradeID] == nil {
		activeTrades[tradeID] = &TradeSession{
			T1Name:  p1Name,
			T1Guild: MyGuildName,
			T2Name:  p2Name,
			T2Guild: p2Guild,
			T1Items: make(map[int]int),
			T2Items: make(map[int]int),
		}
	}

	log.Infof("[TRADE STARTED] %s <-> %s (Session: %s)", p1Name, p2Name, tradeID)
}

func onDecodedEvent(eventType uint16, params map[uint8]interface{}) {
	if eventType < 176 || eventType > 179 {
		return
	}

	AnalyzeTrade(int16(eventType), params)
}

func parseLocation(rawLoc string) string {
	if name, exists := LocationNames[rawLoc]; exists {
		return fmt.Sprintf("%s (ID: %s)", name, rawLoc)
	}
	if rawLoc == "" {
		return "Unknown"
	}
	if strings.Contains(rawLoc, "@ISLAND@") {
		return fmt.Sprintf("Player Island (ID: %s)", rawLoc)
	}
	if strings.Contains(rawLoc, "BLACKBANK") || strings.Contains(rawLoc, "HomeObject") {
		return fmt.Sprintf("Bank / Storage Area (ID: %s)", rawLoc)
	}
	return fmt.Sprintf("Unknown Region (ID: %s)", rawLoc)
}
