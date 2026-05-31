package custom

import (
	"encoding/json"
	"errors"
	"time"

	uuid "github.com/nu7hatch/gouuid"
)

type collectorStoredEvent struct {
	EventID         string           `json:"eventId"`
	SchemaVersion   int              `json:"schemaVersion"`
	EventType       string           `json:"eventType"`
	OccurredAt      time.Time        `json:"occurredAt"`
	Actor           collectorActor   `json:"actor"`
	Context         collectorContext `json:"context"`
	Payload         json.RawMessage  `json:"payload"`
	RawEvent        json.RawMessage  `json:"rawEvent"`
	ProcessorStatus string           `json:"processorStatus"`
	ProcessorError  string           `json:"processorError,omitempty"`
	ProcessedAt     *time.Time       `json:"processedAt,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
}

func ensureCollectorEventID(event *collectorEnvelope) {
	if event.EventID != "" {
		return
	}
	if identifier, err := uuid.NewV4(); err == nil {
		event.EventID = identifier.String()
	}
}

func storeCollectorEvent(event collectorEnvelope) error {
	if db == nil {
		return errors.New("collector storage is not initialized")
	}

	ensureCollectorEventID(&event)
	if event.SchemaVersion == 0 {
		event.SchemaVersion = collectorSchemaVersion
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}

	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	rawEventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT INTO collector_events (
			event_id, schema_version, event_type, occurred_at,
			actor_character_id, actor_character_name,
			context_topic, context_location_id, context_current_map, context_guild_id, context_guild_name,
			context_game_server_ip, context_aodata_server_id, context_aodata_ingest_base_url,
			payload, raw_event
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14,
			$15, $16
		)
		ON CONFLICT (event_id) DO UPDATE SET
			schema_version = EXCLUDED.schema_version,
			event_type = EXCLUDED.event_type,
			occurred_at = EXCLUDED.occurred_at,
			actor_character_id = EXCLUDED.actor_character_id,
			actor_character_name = EXCLUDED.actor_character_name,
			context_topic = EXCLUDED.context_topic,
			context_location_id = EXCLUDED.context_location_id,
			context_current_map = EXCLUDED.context_current_map,
			context_guild_id = EXCLUDED.context_guild_id,
			context_guild_name = EXCLUDED.context_guild_name,
			context_game_server_ip = EXCLUDED.context_game_server_ip,
			context_aodata_server_id = EXCLUDED.context_aodata_server_id,
			context_aodata_ingest_base_url = EXCLUDED.context_aodata_ingest_base_url,
			payload = EXCLUDED.payload,
			raw_event = EXCLUDED.raw_event,
			processor_status = 'pending',
			processor_error = NULL,
			processed_at = NULL
	`,
		event.EventID, event.SchemaVersion, event.EventType, event.OccurredAt,
		event.Actor.CharacterID, event.Actor.CharacterName,
		event.Context.Topic, event.Context.LocationID, event.Context.CurrentMap, event.Context.GuildID, event.Context.GuildName,
		event.Context.GameServerIP, event.Context.AODataServerID, event.Context.AODataIngestBaseURL,
		string(payloadJSON), string(rawEventJSON),
	)

	return err
}
