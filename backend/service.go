package backend

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	_ "github.com/lib/pq"
	"github.com/nu7hatch/gouuid"
	"github.com/sirupsen/logrus"
)

type Service struct {
	cfg           Config
	db            *sql.DB
	processorOnce sync.Once
}

type storedEvent struct {
	EventID         string          `json:"eventId"`
	SchemaVersion   int             `json:"schemaVersion"`
	EventType       string          `json:"eventType"`
	OccurredAt      time.Time       `json:"occurredAt"`
	OwnerUserID     string          `json:"ownerUserId,omitempty"`
	OwnerHandle     string          `json:"ownerHandle,omitempty"`
	Actor           Actor           `json:"actor"`
	Context         Context         `json:"context"`
	Payload         json.RawMessage `json:"payload"`
	RawEvent        json.RawMessage `json:"rawEvent"`
	ProcessorStatus string          `json:"processorStatus"`
	ProcessorError  string          `json:"processorError,omitempty"`
	ProcessedAt     *time.Time      `json:"processedAt,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
}

func NewService(cfg Config) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) Run() error {
	db, err := sql.Open("postgres", s.cfg.DatabaseURL)
	if err != nil {
		return err
	}
	s.db = db
	if err := s.ensureSchema(); err != nil {
		return err
	}
	if err := s.ensureLegacyMarketSchema(); err != nil {
		return err
	}
	s.startProcessor()
	go warmCatalog()

	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(cors.New())
	s.registerRoutes(app)

	logrus.Infof("Standalone backend listening on %s", s.cfg.Addr)
	return app.Listen(s.cfg.Addr)
}

func (s *Service) ensureSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS collector_events (
			event_id TEXT PRIMARY KEY,
			schema_version INTEGER NOT NULL,
			event_type TEXT NOT NULL,
			occurred_at TIMESTAMPTZ NOT NULL,
			owner_user_id TEXT,
			owner_handle TEXT,
			actor_character_id TEXT,
			actor_character_name TEXT,
			context_topic TEXT,
			context_location_id TEXT,
			context_current_map TEXT,
			context_guild_id TEXT,
			context_guild_name TEXT,
			context_game_server_ip TEXT,
			context_aodata_server_id INTEGER,
			context_aodata_ingest_base_url TEXT,
			payload JSONB NOT NULL,
			raw_event JSONB NOT NULL,
			processor_status TEXT NOT NULL DEFAULT 'pending',
			processor_error TEXT,
			processed_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS collector_player_state (
			character_id TEXT PRIMARY KEY,
			owner_user_id TEXT,
			owner_handle TEXT,
			character_name TEXT,
			guild_id TEXT,
			guild_name TEXT,
			location_id TEXT,
			last_event_id TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS collector_market_events (
			event_id TEXT PRIMARY KEY,
			event_type TEXT NOT NULL,
			owner_user_id TEXT,
			owner_handle TEXT,
			actor_character_id TEXT,
			actor_character_name TEXT,
			location_id TEXT,
			topic TEXT,
			payload JSONB NOT NULL,
			occurred_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS collector_trade_reports (
			event_id TEXT PRIMARY KEY,
			owner_user_id TEXT,
			owner_handle TEXT,
			session_id TEXT NOT NULL,
			location TEXT,
			completed_at TIMESTAMPTZ NOT NULL,
			actor_character_name TEXT,
			local_party_name TEXT NOT NULL,
			local_party_guild_name TEXT,
			local_party_silver BIGINT NOT NULL,
			local_party_total BIGINT NOT NULL,
			remote_party_name TEXT NOT NULL,
			remote_party_guild_name TEXT,
			remote_party_silver BIGINT NOT NULL,
			remote_party_total BIGINT NOT NULL,
			net_profit BIGINT NOT NULL,
			raw_payload JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS collector_trade_report_items (
			event_id TEXT NOT NULL,
			party_side TEXT NOT NULL,
			item_id INTEGER NOT NULL,
			item_name TEXT NOT NULL,
			amount INTEGER NOT NULL,
			unit_price BIGINT NOT NULL,
			total_price BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (event_id, party_side, item_id, item_name, amount)
		);`,
		`CREATE TABLE IF NOT EXISTS app_users (
			id TEXT PRIMARY KEY,
			handle TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			preferred_character_name TEXT,
			api_token TEXT NOT NULL UNIQUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`ALTER TABLE collector_events ADD COLUMN IF NOT EXISTS owner_user_id TEXT;`,
		`ALTER TABLE collector_events ADD COLUMN IF NOT EXISTS owner_handle TEXT;`,
		`ALTER TABLE collector_player_state ADD COLUMN IF NOT EXISTS owner_user_id TEXT;`,
		`ALTER TABLE collector_player_state ADD COLUMN IF NOT EXISTS owner_handle TEXT;`,
		`ALTER TABLE collector_market_events ADD COLUMN IF NOT EXISTS owner_user_id TEXT;`,
		`ALTER TABLE collector_market_events ADD COLUMN IF NOT EXISTS owner_handle TEXT;`,
		`ALTER TABLE collector_trade_reports ADD COLUMN IF NOT EXISTS owner_user_id TEXT;`,
		`ALTER TABLE collector_trade_reports ADD COLUMN IF NOT EXISTS owner_handle TEXT;`,
	}
	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) registerRoutes(app *fiber.App) {
	app.Get("/api/collector/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "db": s.db != nil})
	})

	app.Post("/api/collector/events", func(c *fiber.Ctx) error {
		owner, ok := s.collectorOwnerFromRequest(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		var event Envelope
		if err := c.BodyParser(&event); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_json"})
		}
		if strings.TrimSpace(event.EventType) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing_event_type"})
		}
		if err := s.storeEvent(event, owner); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "store_failed", "message": err.Error()})
		}
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "accepted", "eventType": event.EventType, "eventId": event.EventID})
	})

	app.Get("/api/collector/events/recent", s.handleRecentEvents)
	app.Get("/api/collector/processor/status", s.handleProcessorStatus)
	app.Get("/api/collector/projections/player-state", s.handlePlayerState)
	app.Get("/api/collector/projections/trades/recent", s.handleTradesRecent)
	s.registerMarketRoutes(app)
	s.registerPrivateRoutes(app)
}

func (s *Service) collectorOwnerFromRequest(c *fiber.Ctx) (*appUser, bool) {
	authHeader := strings.TrimSpace(c.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		authHeader = strings.TrimSpace(authHeader[7:])
	}

	expected := strings.TrimSpace(s.cfg.CollectorToken)
	if authHeader == "" {
		return nil, expected == ""
	}
	if expected != "" && authHeader == expected {
		return nil, true
	}

	user, err := s.findUserByToken(authHeader)
	if err == nil {
		return &user, true
	}
	return nil, false
}

func (s *Service) storeEvent(event Envelope, owner *appUser) error {
	if s.db == nil {
		return errors.New("database not initialized")
	}
	if event.EventID == "" {
		if id, err := uuid.NewV4(); err == nil {
			event.EventID = id.String()
		}
	}
	if event.SchemaVersion == 0 {
		event.SchemaVersion = SchemaVersion
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	payloadJSON, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	rawJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	ownerUserID := ""
	ownerHandle := ""
	if owner != nil {
		ownerUserID = owner.ID
		ownerHandle = owner.Handle
	}

	_, err = s.db.Exec(`
		INSERT INTO collector_events (
			event_id, schema_version, event_type, occurred_at,
			owner_user_id, owner_handle,
			actor_character_id, actor_character_name,
			context_topic, context_location_id, context_current_map, context_guild_id, context_guild_name,
			context_game_server_ip, context_aodata_server_id, context_aodata_ingest_base_url,
			payload, raw_event
		) VALUES (
			$1, $2, $3, $4,
			$5, $6,
			$7, $8,
			$9, $10, $11, $12, $13,
			$14, $15, $16,
			$17::jsonb, $18::jsonb
		)
		ON CONFLICT (event_id) DO UPDATE SET
			schema_version = EXCLUDED.schema_version,
			event_type = EXCLUDED.event_type,
			occurred_at = EXCLUDED.occurred_at,
			owner_user_id = EXCLUDED.owner_user_id,
			owner_handle = EXCLUDED.owner_handle,
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
		ownerUserID, ownerHandle,
		event.Actor.CharacterID, event.Actor.CharacterName,
		event.Context.Topic, event.Context.LocationID, event.Context.CurrentMap, event.Context.GuildID, event.Context.GuildName,
		event.Context.GameServerIP, event.Context.AODataServerID, event.Context.AODataIngestBaseURL,
		string(payloadJSON), string(rawJSON),
	)
	return err
}

func (s *Service) startProcessor() {
	s.processorOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				s.processPending(50)
				<-ticker.C
			}
		}()
	})
}

func (s *Service) processPending(limit int) {
	events, err := s.loadPending(limit)
	if err != nil {
		logrus.Errorf("Standalone processor query failed: %v", err)
		return
	}
	for _, event := range events {
		if err := s.processEvent(event); err != nil {
			logrus.Errorf("Standalone processor failed for %s: %v", event.EventID, err)
			s.markFailed(event.EventID, err.Error())
			continue
		}
		s.markProcessed(event.EventID)
	}
}

func (s *Service) loadPending(limit int) ([]storedEvent, error) {
	rows, err := s.db.Query(`
		SELECT event_id, schema_version, event_type, occurred_at, owner_user_id, owner_handle,
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

	items := make([]storedEvent, 0, limit)
	for rows.Next() {
		var event storedEvent
		var ownerUserID sql.NullString
		var ownerHandle sql.NullString
		var processorError sql.NullString
		var processedAt sql.NullTime
		if err := rows.Scan(
			&event.EventID, &event.SchemaVersion, &event.EventType, &event.OccurredAt, &ownerUserID, &ownerHandle,
			&event.Actor.CharacterID, &event.Actor.CharacterName,
			&event.Context.Topic, &event.Context.LocationID, &event.Context.CurrentMap, &event.Context.GuildID, &event.Context.GuildName,
			&event.Payload, &event.RawEvent, &event.ProcessorStatus, &processorError, &processedAt, &event.CreatedAt,
		); err != nil {
			return nil, err
		}
		if ownerUserID.Valid {
			event.OwnerUserID = ownerUserID.String
		}
		if ownerHandle.Valid {
			event.OwnerHandle = ownerHandle.String
		}
		if processorError.Valid {
			event.ProcessorError = processorError.String
		}
		if processedAt.Valid {
			ts := processedAt.Time
			event.ProcessedAt = &ts
		}
		items = append(items, event)
	}
	return items, nil
}

func (s *Service) processEvent(event storedEvent) error {
	switch event.EventType {
	case "albion.player.join_state":
		var payload JoinStatePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		_, err := s.db.Exec(`
			INSERT INTO collector_player_state (character_id, owner_user_id, owner_handle, character_name, guild_id, guild_name, location_id, last_event_id, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (character_id) DO UPDATE SET
				owner_user_id = EXCLUDED.owner_user_id,
				owner_handle = EXCLUDED.owner_handle,
				character_name = EXCLUDED.character_name,
				guild_id = EXCLUDED.guild_id,
				guild_name = EXCLUDED.guild_name,
				location_id = EXCLUDED.location_id,
				last_event_id = EXCLUDED.last_event_id,
				updated_at = EXCLUDED.updated_at
		`, firstNonEmpty(payload.CharacterID, event.Actor.CharacterID), event.OwnerUserID, event.OwnerHandle, firstNonEmpty(payload.CharacterName, event.Actor.CharacterName), payload.GuildID, payload.GuildName, payload.LocationID, event.EventID, event.OccurredAt)
		return err
	case "albion.market.orders_snapshot", "albion.market.history_snapshot", "albion.market.gold_snapshot":
		_, err := s.db.Exec(`
			INSERT INTO collector_market_events (event_id, event_type, owner_user_id, owner_handle, actor_character_id, actor_character_name, location_id, topic, payload, occurred_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10)
			ON CONFLICT (event_id) DO UPDATE SET
				event_type = EXCLUDED.event_type,
				owner_user_id = EXCLUDED.owner_user_id,
				owner_handle = EXCLUDED.owner_handle,
				actor_character_id = EXCLUDED.actor_character_id,
				actor_character_name = EXCLUDED.actor_character_name,
				location_id = EXCLUDED.location_id,
				topic = EXCLUDED.topic,
				payload = EXCLUDED.payload,
				occurred_at = EXCLUDED.occurred_at
		`, event.EventID, event.EventType, event.OwnerUserID, event.OwnerHandle, event.Actor.CharacterID, event.Actor.CharacterName, firstNonEmpty(event.Context.LocationID, event.Context.CurrentMap), event.Context.Topic, string(event.Payload), event.OccurredAt)
		if err != nil {
			return err
		}
		return s.projectLegacyMarketData(event)
	case "albion.trade.completed":
		var payload TradeCompletedPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		_, err = tx.Exec(`
			INSERT INTO collector_trade_reports (
				event_id, owner_user_id, owner_handle, session_id, location, completed_at, actor_character_name,
				local_party_name, local_party_guild_name, local_party_silver, local_party_total,
				remote_party_name, remote_party_guild_name, remote_party_silver, remote_party_total,
				net_profit, raw_payload
			) VALUES (
				$1,$2,$3,$4,$5,$6,$7,
				$8,$9,$10,$11,
				$12,$13,$14,$15,
				$16,$17::jsonb
			)
			ON CONFLICT (event_id) DO UPDATE SET
				owner_user_id = EXCLUDED.owner_user_id,
				owner_handle = EXCLUDED.owner_handle,
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
		`, event.EventID, event.OwnerUserID, event.OwnerHandle, payload.SessionID, payload.Location, payload.CompletedAt, firstNonEmpty(event.Actor.CharacterName, payload.LocalParty.Name),
			payload.LocalParty.Name, payload.LocalParty.GuildName, payload.LocalParty.Silver, payload.LocalParty.Total,
			payload.RemoteParty.Name, payload.RemoteParty.GuildName, payload.RemoteParty.Silver, payload.RemoteParty.Total,
			payload.NetProfit, string(event.Payload))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM collector_trade_report_items WHERE event_id = $1`, event.EventID); err != nil {
			return err
		}
		for _, item := range payload.LocalParty.Items {
			if _, err := tx.Exec(`INSERT INTO collector_trade_report_items (event_id, party_side, item_id, item_name, amount, unit_price, total_price) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				event.EventID, "local", item.ItemID, item.ItemName, item.Amount, item.UnitPrice, item.TotalPrice); err != nil {
				return err
			}
		}
		for _, item := range payload.RemoteParty.Items {
			if _, err := tx.Exec(`INSERT INTO collector_trade_report_items (event_id, party_side, item_id, item_name, amount, unit_price, total_price) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
				event.EventID, "remote", item.ItemID, item.ItemName, item.Amount, item.UnitPrice, item.TotalPrice); err != nil {
				return err
			}
		}
		return tx.Commit()
	default:
		return nil
	}
}

func (s *Service) markProcessed(eventID string) {
	_, _ = s.db.Exec(`UPDATE collector_events SET processor_status='processed', processor_error=NULL, processed_at=CURRENT_TIMESTAMP WHERE event_id = $1`, eventID)
}

func (s *Service) markFailed(eventID string, msg string) {
	msg = strings.TrimSpace(msg)
	if len(msg) > 1000 {
		msg = msg[:1000]
	}
	_, _ = s.db.Exec(`UPDATE collector_events SET processor_status='failed', processor_error=$2 WHERE event_id=$1`, eventID, msg)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (s *Service) handleRecentEvents(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 25)
	if limit < 1 {
		limit = 25
	}
	if limit > 200 {
		limit = 200
	}
	eventType := strings.TrimSpace(c.Query("eventType"))
	var rows *sql.Rows
	var err error
	if eventType == "" {
		rows, err = s.db.Query(`
			SELECT event_id, schema_version, event_type, occurred_at, owner_user_id, owner_handle,
			       actor_character_id, actor_character_name,
			       context_topic, context_location_id, context_current_map, context_guild_id, context_guild_name,
			       payload, raw_event, processor_status, processor_error, processed_at, created_at
			FROM collector_events
			ORDER BY occurred_at DESC
			LIMIT $1
		`, limit)
	} else {
		rows, err = s.db.Query(`
			SELECT event_id, schema_version, event_type, occurred_at, owner_user_id, owner_handle,
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
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "query_failed", "message": err.Error()})
	}
	defer rows.Close()
	items := make([]storedEvent, 0, limit)
	for rows.Next() {
		var event storedEvent
		var ownerUserID sql.NullString
		var ownerHandle sql.NullString
		var processorError sql.NullString
		var processedAt sql.NullTime
		if err := rows.Scan(
			&event.EventID, &event.SchemaVersion, &event.EventType, &event.OccurredAt, &ownerUserID, &ownerHandle,
			&event.Actor.CharacterID, &event.Actor.CharacterName,
			&event.Context.Topic, &event.Context.LocationID, &event.Context.CurrentMap, &event.Context.GuildID, &event.Context.GuildName,
			&event.Payload, &event.RawEvent, &event.ProcessorStatus, &processorError, &processedAt, &event.CreatedAt,
		); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "scan_failed", "message": err.Error()})
		}
		if ownerUserID.Valid {
			event.OwnerUserID = ownerUserID.String
		}
		if ownerHandle.Valid {
			event.OwnerHandle = ownerHandle.String
		}
		if processorError.Valid {
			event.ProcessorError = processorError.String
		}
		if processedAt.Valid {
			ts := processedAt.Time
			event.ProcessedAt = &ts
		}
		items = append(items, event)
	}
	return c.JSON(fiber.Map{"items": items, "count": len(items)})
}

func (s *Service) handleProcessorStatus(c *fiber.Ctx) error {
	rows, err := s.db.Query(`SELECT processor_status, COUNT(*) FROM collector_events GROUP BY processor_status ORDER BY processor_status`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "query_failed", "message": err.Error()})
	}
	defer rows.Close()
	counts := fiber.Map{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "scan_failed", "message": err.Error()})
		}
		counts[status] = count
	}
	return c.JSON(fiber.Map{"counts": counts})
}

func (s *Service) handlePlayerState(c *fiber.Ctx) error {
	rows, err := s.db.Query(`
		SELECT character_id, character_name, guild_id, guild_name, location_id, last_event_id, updated_at
		FROM collector_player_state
		ORDER BY updated_at DESC
		LIMIT 100
	`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "query_failed", "message": err.Error()})
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
		if err := rows.Scan(&entry.CharacterID, &entry.CharacterName, &entry.GuildID, &entry.GuildName, &entry.LocationID, &entry.LastEventID, &updatedAt); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "scan_failed", "message": err.Error()})
		}
		if updatedAt.Valid {
			entry.UpdatedAt = updatedAt.Time.UTC().Format(time.RFC3339)
		}
		items = append(items, entry)
	}
	return c.JSON(fiber.Map{"items": items, "count": len(items)})
}

func (s *Service) handleTradesRecent(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 25)
	if limit < 1 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT event_id, session_id, location, completed_at, actor_character_name,
		       local_party_name, local_party_guild_name, local_party_silver, local_party_total,
		       remote_party_name, remote_party_guild_name, remote_party_silver, remote_party_total, net_profit
		FROM collector_trade_reports
		ORDER BY completed_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "query_failed", "message": err.Error()})
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
		if err := rows.Scan(&entry.EventID, &entry.SessionID, &entry.Location, &completedAt, &entry.ActorCharacterName, &entry.LocalPartyName, &entry.LocalPartyGuild, &entry.LocalSilver, &entry.LocalTotal, &entry.RemotePartyName, &entry.RemotePartyGuild, &entry.RemoteSilver, &entry.RemoteTotal, &entry.NetProfit); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "scan_failed", "message": err.Error()})
		}
		if completedAt.Valid {
			entry.CompletedAt = completedAt.Time.UTC().Format(time.RFC3339)
		}
		items = append(items, entry)
	}
	return c.JSON(fiber.Map{"items": items, "count": len(items)})
}
