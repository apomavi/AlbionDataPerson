package bridge

import "sync"

type UploadContext struct {
	CharacterID         string
	CharacterName       string
	LocationID          string
	CurrentMap          string
	AODataServerID      int
	AODataIngestBaseURL string
	GameServerIP        string
}

type JoinStateData struct {
	CharacterID   string
	CharacterName string
	GuildID       string
	GuildName     string
	LocationID    string
}

type Hooks struct {
	StartBackgroundServices func()
	OnPublicUpload          func(topic string, data []byte, ctx UploadContext)
	OnDecodedResponse       func(code uint16, params map[uint8]interface{})
	OnDecodedEvent          func(code uint16, params map[uint8]interface{})
	OnJoinStateUpdated      func(data JoinStateData)
	OnGoldPrices            func(prices []int, timestamps []int64)
}

var (
	hooksMu    sync.RWMutex
	registered Hooks
	currentMap string
)

func RegisterHooks(h Hooks) {
	hooksMu.Lock()
	defer hooksMu.Unlock()

	if h.StartBackgroundServices != nil {
		registered.StartBackgroundServices = h.StartBackgroundServices
	}
	if h.OnPublicUpload != nil {
		registered.OnPublicUpload = h.OnPublicUpload
	}
	if h.OnDecodedResponse != nil {
		registered.OnDecodedResponse = h.OnDecodedResponse
	}
	if h.OnDecodedEvent != nil {
		registered.OnDecodedEvent = h.OnDecodedEvent
	}
	if h.OnJoinStateUpdated != nil {
		registered.OnJoinStateUpdated = h.OnJoinStateUpdated
	}
	if h.OnGoldPrices != nil {
		registered.OnGoldPrices = h.OnGoldPrices
	}
}

func StartBackgroundServices() {
	hooksMu.RLock()
	hook := registered.StartBackgroundServices
	hooksMu.RUnlock()

	if hook != nil {
		hook()
	}
}

func RunPublicUpload(topic string, data []byte, ctx UploadContext) {
	hooksMu.RLock()
	hook := registered.OnPublicUpload
	hooksMu.RUnlock()

	if hook != nil {
		hook(topic, data, ctx)
	}
}

func RunDecodedResponse(code uint16, params map[uint8]interface{}) {
	hooksMu.RLock()
	hook := registered.OnDecodedResponse
	hooksMu.RUnlock()

	if hook != nil {
		hook(code, params)
	}
}

func RunDecodedEvent(code uint16, params map[uint8]interface{}) {
	hooksMu.RLock()
	hook := registered.OnDecodedEvent
	hooksMu.RUnlock()

	if hook != nil {
		hook(code, params)
	}
}

func RunJoinStateUpdated(data JoinStateData) {
	hooksMu.RLock()
	hook := registered.OnJoinStateUpdated
	hooksMu.RUnlock()

	if hook != nil {
		hook(data)
	}
}

func RunGoldPrices(prices []int, timestamps []int64) {
	hooksMu.RLock()
	hook := registered.OnGoldPrices
	hooksMu.RUnlock()

	if hook != nil {
		hook(prices, timestamps)
	}
}

func RecordCurrentMap(location string) {
	hooksMu.Lock()
	currentMap = location
	hooksMu.Unlock()
}

func CurrentMap() string {
	hooksMu.RLock()
	defer hooksMu.RUnlock()
	return currentMap
}
