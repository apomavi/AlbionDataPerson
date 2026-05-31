package custom

import (
	"database/sql"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/ao-data/albiondata-client/log"
)

const collectorProcessorBatchSize = 50

var collectorProcessorOnce sync.Once

func startCollectorProcessor() {
	if db == nil {
		return
	}

	collectorProcessorOnce.Do(func() {
		go collectorProcessorLoop()
		log.Info("Collector processor started.")
	})
}

func collectorProcessorLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		processPendingCollectorEvents()
		<-ticker.C
	}
}

func processPendingCollectorEvents() {
	events, err := loadPendingCollectorEvents(collectorProcessorBatchSize)
	if err != nil {
		log.Errorf("Collector processor query failed: %v", err)
		return
	}

	for _, event := range events {
		if err := processCollectorEvent(event); err != nil {
			log.Errorf("Collector processor failed for %s: %v", event.EventID, err)
			markCollectorEventFailed(event.EventID, err.Error())
			continue
		}
		markCollectorEventProcessed(event.EventID)
	}
}

func loadPendingCollectorEvents(limit int) ([]collectorStoredEvent, error) {
	rows, err := db.Query(`
		SELECT event_id, schema_version, event_type, occurred_at,
		       actor_character_id, actor_character_name,
		       context_topic, context_location_id, context_current_map, context_guild_id, context_guild_name,
		       payload, raw_event, processor_status, processor_error, processed_at, created_at
		FROM collector_events
		WHERE processed_at IS NULL AND processor_status = 'pending'
		ORDER BY occurred_at ASC, created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]collectorStoredEvent, 0, limit)
	for rows.Next() {
		var event collectorStoredEvent
		var processorError sql.NullString
		var processedAt sql.NullTime
		var payload string
		var rawEvent string
		if err := rows.Scan(
			&event.EventID,
			&event.SchemaVersion,
			&event.EventType,
			&event.OccurredAt,
			&event.Actor.CharacterID,
			&event.Actor.CharacterName,
			&event.Context.Topic,
			&event.Context.LocationID,
			&event.Context.CurrentMap,
			&event.Context.GuildID,
			&event.Context.GuildName,
			&payload,
			&rawEvent,
			&event.ProcessorStatus,
			&processorError,
			&processedAt,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		event.Payload = json.RawMessage(payload)
		event.RawEvent = json.RawMessage(rawEvent)
		if processorError.Valid {
			event.ProcessorError = processorError.String
		}
		if processedAt.Valid {
			ts := processedAt.Time
			event.ProcessedAt = &ts
		}
		events = append(events, event)
	}

	return events, nil
}

func processCollectorEvent(event collectorStoredEvent) error {
	switch event.EventType {
	case "albion.player.join_state":
		return processCollectorJoinState(event)
	case "albion.market.orders_snapshot", "albion.market.history_snapshot", "albion.market.gold_snapshot":
		return processCollectorMarketEvent(event)
	case "albion.trade.completed":
		return processCollectorTradeCompleted(event)
	default:
		return nil
	}
}

func processCollectorJoinState(event collectorStoredEvent) error {
	var payload collectorJoinStatePayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	_, err := db.Exec(`
		INSERT INTO collector_player_state (
			character_id, character_name, guild_id, guild_name, location_id, last_event_id, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (character_id) DO UPDATE SET
			character_name = EXCLUDED.character_name,
			guild_id = EXCLUDED.guild_id,
			guild_name = EXCLUDED.guild_name,
			location_id = EXCLUDED.location_id,
			last_event_id = EXCLUDED.last_event_id,
			updated_at = EXCLUDED.updated_at
	`,
		blankToFallback(payload.CharacterID, event.Actor.CharacterID),
		blankToFallback(payload.CharacterName, event.Actor.CharacterName),
		payload.GuildID,
		payload.GuildName,
		payload.LocationID,
		event.EventID,
		event.OccurredAt,
	)

	return err
}

func processCollectorMarketEvent(event collectorStoredEvent) error {
	_, err := db.Exec(`
		INSERT INTO collector_market_events (
			event_id, event_type, actor_character_id, actor_character_name, location_id, topic, payload, occurred_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (event_id) DO UPDATE SET
			event_type = EXCLUDED.event_type,
			actor_character_id = EXCLUDED.actor_character_id,
			actor_character_name = EXCLUDED.actor_character_name,
			location_id = EXCLUDED.location_id,
			topic = EXCLUDED.topic,
			payload = EXCLUDED.payload,
			occurred_at = EXCLUDED.occurred_at
	`,
		event.EventID,
		event.EventType,
		event.Actor.CharacterID,
		event.Actor.CharacterName,
		firstNonEmpty(event.Context.LocationID, event.Context.CurrentMap),
		event.Context.Topic,
		string(event.Payload),
		event.OccurredAt,
	)

	return err
}

func processCollectorTradeCompleted(event collectorStoredEvent) error {
	var payload collectorTradeCompletedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO collector_trade_reports (
			event_id, session_id, location, completed_at, actor_character_name,
			local_party_name, local_party_guild_name, local_party_silver, local_party_total,
			remote_party_name, remote_party_guild_name, remote_party_silver, remote_party_total,
			net_profit, raw_payload
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15
		)
		ON CONFLICT (event_id) DO UPDATE SET
			session_id = EXCLUDED.session_id,
			location = EXCLUDED.location,
			completed_at = EXCLUDED.completed_at,
			actor_character_name = EXCLUDED.actor_character_name,
			local_party_name = EXCLUDED.local_party_name,
			local_party_guild_name = EXCLUDED.local_party_guild_name,
			local_party_silver = EXCLUDED.local_party_silver,
			local_party_total = EXCLUDED.local_party_total,
			remote_party_name = EXCLUDED.remote_party_name,
			remote_party_guild_name = EXCLUDED.remote_party_guild_name,
			remote_party_silver = EXCLUDED.remote_party_silver,
			remote_party_total = EXCLUDED.remote_party_total,
			net_profit = EXCLUDED.net_profit,
			raw_payload = EXCLUDED.raw_payload
	`,
		event.EventID,
		payload.SessionID,
		payload.Location,
		payload.CompletedAt,
		firstNonEmpty(event.Actor.CharacterName, payload.LocalParty.Name),
		payload.LocalParty.Name,
		payload.LocalParty.GuildName,
		payload.LocalParty.Silver,
		payload.LocalParty.Total,
		payload.RemoteParty.Name,
		payload.RemoteParty.GuildName,
		payload.RemoteParty.Silver,
		payload.RemoteParty.Total,
		payload.NetProfit,
		string(event.Payload),
	)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM collector_trade_report_items WHERE event_id = $1`, event.EventID); err != nil {
		return err
	}

	for _, item := range payload.LocalParty.Items {
		if err := insertCollectorTradeItem(tx, event.EventID, "local", item); err != nil {
			return err
		}
	}
	for _, item := range payload.RemoteParty.Items {
		if err := insertCollectorTradeItem(tx, event.EventID, "remote", item); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func insertCollectorTradeItem(tx *sql.Tx, eventID string, side string, item collectorTradeItem) error {
	_, err := tx.Exec(`
		INSERT INTO collector_trade_report_items (
			event_id, party_side, item_id, item_name, amount, unit_price, total_price
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		eventID, side, item.ItemID, item.ItemName, item.Amount, item.UnitPrice, item.TotalPrice,
	)
	return err
}

func markCollectorEventProcessed(eventID string) {
	_, _ = db.Exec(`
		UPDATE collector_events
		SET processor_status = 'processed', processor_error = NULL, processed_at = CURRENT_TIMESTAMP
		WHERE event_id = $1
	`, eventID)
}

func markCollectorEventFailed(eventID string, message string) {
	trimmed := strings.TrimSpace(message)
	if len(trimmed) > 1000 {
		trimmed = trimmed[:1000]
	}
	_, _ = db.Exec(`
		UPDATE collector_events
		SET processor_status = 'failed', processor_error = $2
		WHERE event_id = $1
	`, eventID, trimmed)
}

func blankToFallback(primary string, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
