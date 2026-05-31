package custom

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ao-data/albiondata-client/client"
	"github.com/ao-data/albiondata-client/contracts"
	"github.com/ao-data/albiondata-client/custom/bridge"
	"github.com/ao-data/albiondata-client/lib"
	"github.com/ao-data/albiondata-client/log"
	uuid "github.com/nu7hatch/gouuid"
)

const (
	collectorSchemaVersion = contracts.SchemaVersion
	collectorQueueSize     = 128
)

type collectorEnvelope = contracts.Envelope
type collectorActor = contracts.Actor
type collectorContext = contracts.Context
type collectorJoinStatePayload = contracts.JoinStatePayload
type collectorTradeParty = contracts.TradeParty
type collectorTradeItem = contracts.TradeItem
type collectorTradeCompletedPayload = contracts.TradeCompletedPayload

var (
	collectorOnce sync.Once
	collectorChan chan collectorEnvelope
)

func startCollectorService() {
	if strings.TrimSpace(client.ConfigGlobal.CollectorEndpoint) == "" {
		return
	}

	collectorOnce.Do(func() {
		collectorChan = make(chan collectorEnvelope, collectorQueueSize)
		go collectorWorker()
		log.Infof("Normalized collector enabled for %s", client.ConfigGlobal.CollectorEndpoint)
	})
}

func collectorWorker() {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	endpoint := strings.TrimSpace(client.ConfigGlobal.CollectorEndpoint)
	token := strings.TrimSpace(client.ConfigGlobal.CollectorAuthToken)

	for event := range collectorChan {
		body, err := json.Marshal(event)
		if err != nil {
			log.Errorf("Collector marshal failed for %s: %v", event.EventType, err)
			continue
		}

		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			log.Errorf("Collector request build failed for %s: %v", event.EventType, err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			log.Errorf("Collector delivery failed for %s: %v", event.EventType, err)
			continue
		}
		_ = resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Errorf("Collector delivery failed for %s: HTTP %d", event.EventType, resp.StatusCode)
		}
	}
}

func enqueueCollectorEvent(event collectorEnvelope) {
	if collectorChan == nil {
		return
	}

	select {
	case collectorChan <- event:
	default:
		log.Warnf("Collector queue is full, dropping %s", event.EventType)
	}
}

func newCollectorEnvelope(eventType string, actor collectorActor, context collectorContext, payload interface{}) collectorEnvelope {
	eventID := ""
	if identifier, err := uuid.NewV4(); err == nil {
		eventID = identifier.String()
	}

	return collectorEnvelope{
		SchemaVersion: collectorSchemaVersion,
		EventID:       eventID,
		EventType:     eventType,
		OccurredAt:    time.Now().UTC(),
		Actor:         actor,
		Context:       context,
		Payload:       payload,
	}
}

func emitCollectorJoinState(data bridge.JoinStateData) {
	if collectorChan == nil {
		return
	}

	actor := collectorActor{
		CharacterID:   data.CharacterID,
		CharacterName: data.CharacterName,
	}
	context := collectorContext{
		LocationID: data.LocationID,
		GuildID:    data.GuildID,
		GuildName:  data.GuildName,
	}
	payload := collectorJoinStatePayload{
		CharacterID:   data.CharacterID,
		CharacterName: data.CharacterName,
		GuildID:       data.GuildID,
		GuildName:     data.GuildName,
		LocationID:    data.LocationID,
	}

	enqueueCollectorEvent(newCollectorEnvelope("albion.player.join_state", actor, context, payload))
}

func emitCollectorPublicUpload(topic string, jsonData []byte, ctx bridge.UploadContext) {
	if collectorChan == nil {
		return
	}

	actor := collectorActor{
		CharacterID:   ctx.CharacterID,
		CharacterName: ctx.CharacterName,
	}
	context := collectorContext{
		Topic:               topic,
		LocationID:          ctx.LocationID,
		CurrentMap:          ctx.CurrentMap,
		GameServerIP:        ctx.GameServerIP,
		AODataServerID:      ctx.AODataServerID,
		AODataIngestBaseURL: ctx.AODataIngestBaseURL,
	}

	switch topic {
	case lib.NatsMarketOrdersIngest:
		var payload lib.MarketUpload
		if err := json.Unmarshal(jsonData, &payload); err != nil {
			log.Errorf("Collector decode failed for market orders: %v", err)
			return
		}
		enqueueCollectorEvent(newCollectorEnvelope("albion.market.orders_snapshot", actor, context, payload))
	case lib.NatsMarketHistoriesIngest:
		var payload lib.MarketHistoriesUpload
		if err := json.Unmarshal(jsonData, &payload); err != nil {
			log.Errorf("Collector decode failed for market history: %v", err)
			return
		}
		enqueueCollectorEvent(newCollectorEnvelope("albion.market.history_snapshot", actor, context, payload))
	case lib.NatsGoldPricesIngest:
		var payload lib.GoldPricesUpload
		if err := json.Unmarshal(jsonData, &payload); err != nil {
			log.Errorf("Collector decode failed for gold prices: %v", err)
			return
		}
		enqueueCollectorEvent(newCollectorEnvelope("albion.market.gold_snapshot", actor, context, payload))
	}
}

func emitCollectorTradeCompleted(payload collectorTradeCompletedPayload, actor collectorActor, context collectorContext) {
	if collectorChan == nil {
		return
	}

	enqueueCollectorEvent(newCollectorEnvelope("albion.trade.completed", actor, context, payload))
}
