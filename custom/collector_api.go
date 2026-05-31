package custom

import (
	"database/sql"
	"strings"
	"time"

	"github.com/ao-data/albiondata-client/client"
	"github.com/gofiber/fiber/v2"
)

func SetupCollectorRoutes(app *fiber.App) {
	app.Get("/api/collector/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "ok",
			"db":     db != nil,
		})
	})

	app.Get("/api/collector/processor/status", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "database_unavailable",
			})
		}

		rows, err := db.Query(`
			SELECT processor_status, COUNT(*)
			FROM collector_events
			GROUP BY processor_status
			ORDER BY processor_status
		`)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "query_failed",
				"message": err.Error(),
			})
		}
		defer rows.Close()

		counts := fiber.Map{}
		for rows.Next() {
			var status string
			var count int
			if err := rows.Scan(&status, &count); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error":   "scan_failed",
					"message": err.Error(),
				})
			}
			counts[status] = count
		}

		return c.JSON(fiber.Map{
			"counts": counts,
		})
	})

	app.Get("/api/collector/projections/player-state", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "database_unavailable",
			})
		}

		rows, err := db.Query(`
			SELECT character_id, character_name, guild_id, guild_name, location_id, last_event_id, updated_at
			FROM collector_player_state
			ORDER BY updated_at DESC
			LIMIT 100
		`)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "query_failed",
				"message": err.Error(),
			})
		}
		defer rows.Close()

		type item struct {
			CharacterID   string `json:"characterId"`
			CharacterName string `json:"characterName"`
			GuildID       string `json:"guildId"`
			GuildName     string `json:"guildName"`
			LocationID    string `json:"locationId"`
			LastEventID   string `json:"lastEventId"`
			UpdatedAt     string `json:"updatedAt"`
		}

		items := make([]item, 0)
		for rows.Next() {
			var entry item
			var updatedAt sql.NullTime
			if err := rows.Scan(
				&entry.CharacterID,
				&entry.CharacterName,
				&entry.GuildID,
				&entry.GuildName,
				&entry.LocationID,
				&entry.LastEventID,
				&updatedAt,
			); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error":   "scan_failed",
					"message": err.Error(),
				})
			}
			if updatedAt.Valid {
				entry.UpdatedAt = updatedAt.Time.UTC().Format(time.RFC3339)
			}
			items = append(items, entry)
		}

		return c.JSON(fiber.Map{"items": items, "count": len(items)})
	})

	app.Get("/api/collector/projections/trades/recent", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "database_unavailable",
			})
		}

		limit := c.QueryInt("limit", 25)
		if limit < 1 {
			limit = 25
		}
		if limit > 100 {
			limit = 100
		}

		rows, err := db.Query(`
			SELECT event_id, session_id, location, completed_at, actor_character_name,
			       local_party_name, local_party_guild_name, local_party_silver, local_party_total,
			       remote_party_name, remote_party_guild_name, remote_party_silver, remote_party_total, net_profit
			FROM collector_trade_reports
			ORDER BY completed_at DESC
			LIMIT $1
		`, limit)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "query_failed",
				"message": err.Error(),
			})
		}
		defer rows.Close()

		type item struct {
			EventID            string `json:"eventId"`
			SessionID          string `json:"sessionId"`
			Location           string `json:"location"`
			CompletedAt        string `json:"completedAt"`
			ActorCharacterName string `json:"actorCharacterName"`
			LocalPartyName     string `json:"localPartyName"`
			LocalPartyGuild    string `json:"localPartyGuildName"`
			LocalSilver        int64  `json:"localSilver"`
			LocalTotal         int64  `json:"localTotal"`
			RemotePartyName    string `json:"remotePartyName"`
			RemotePartyGuild   string `json:"remotePartyGuildName"`
			RemoteSilver       int64  `json:"remoteSilver"`
			RemoteTotal        int64  `json:"remoteTotal"`
			NetProfit          int64  `json:"netProfit"`
		}

		items := make([]item, 0, limit)
		for rows.Next() {
			var entry item
			var completedAt sql.NullTime
			if err := rows.Scan(
				&entry.EventID,
				&entry.SessionID,
				&entry.Location,
				&completedAt,
				&entry.ActorCharacterName,
				&entry.LocalPartyName,
				&entry.LocalPartyGuild,
				&entry.LocalSilver,
				&entry.LocalTotal,
				&entry.RemotePartyName,
				&entry.RemotePartyGuild,
				&entry.RemoteSilver,
				&entry.RemoteTotal,
				&entry.NetProfit,
			); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error":   "scan_failed",
					"message": err.Error(),
				})
			}
			if completedAt.Valid {
				entry.CompletedAt = completedAt.Time.UTC().Format(time.RFC3339)
			}
			items = append(items, entry)
		}

		return c.JSON(fiber.Map{"items": items, "count": len(items)})
	})

	app.Post("/api/collector/events", func(c *fiber.Ctx) error {
		if !collectorRequestAuthorized(c) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}

		var event collectorEnvelope
		if err := c.BodyParser(&event); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid_json",
			})
		}
		if strings.TrimSpace(event.EventType) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "missing_event_type",
			})
		}

		if err := storeCollectorEvent(event); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "store_failed",
				"message": err.Error(),
			})
		}

		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"status":    "accepted",
			"eventType": event.EventType,
			"eventId":   event.EventID,
		})
	})

	app.Get("/api/collector/events/recent", func(c *fiber.Ctx) error {
		if db == nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "database_unavailable",
			})
		}

		limit := c.QueryInt("limit", 25)
		if limit < 1 {
			limit = 25
		}
		if limit > 200 {
			limit = 200
		}

		eventType := strings.TrimSpace(c.Query("eventType"))

		var (
			rows *sql.Rows
			err  error
		)
		if eventType == "" {
			rows, err = db.Query(`
				SELECT event_id, schema_version, event_type, occurred_at,
				       actor_character_id, actor_character_name,
				       context_topic, context_location_id, context_current_map, context_guild_id, context_guild_name,
				       payload, raw_event, processor_status, processor_error, processed_at, created_at
				FROM collector_events
				ORDER BY occurred_at DESC
				LIMIT $1
			`, limit)
		} else {
			rows, err = db.Query(`
				SELECT event_id, schema_version, event_type, occurred_at,
				       actor_character_id, actor_character_name,
				       context_topic, context_location_id, context_current_map, context_guild_id, context_guild_name,
				       payload, raw_event, processor_status, processor_error, processed_at, created_at
				FROM collector_events
				WHERE event_type = $1
				ORDER BY occurred_at DESC
				LIMIT $2
			`, eventType, limit)
		}
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "query_failed",
				"message": err.Error(),
			})
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
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error":   "scan_failed",
					"message": err.Error(),
				})
			}
			event.Payload = []byte(payload)
			event.RawEvent = []byte(rawEvent)
			if processorError.Valid {
				event.ProcessorError = processorError.String
			}
			if processedAt.Valid {
				ts := processedAt.Time
				event.ProcessedAt = &ts
			}
			events = append(events, event)
		}

		return c.JSON(fiber.Map{
			"items": events,
			"count": len(events),
		})
	})
}

func collectorRequestAuthorized(c *fiber.Ctx) bool {
	expected := strings.TrimSpace(client.ConfigGlobal.CollectorAuthToken)
	if expected == "" {
		return true
	}

	authHeader := strings.TrimSpace(c.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		authHeader = strings.TrimSpace(authHeader[7:])
	}

	return authHeader == expected
}
