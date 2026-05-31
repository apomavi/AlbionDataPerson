package backend

import (
	"database/sql"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nu7hatch/gouuid"
)

type appUser struct {
	ID                     string `json:"id"`
	Handle                 string `json:"handle"`
	DisplayName            string `json:"displayName"`
	PreferredCharacterName string `json:"preferredCharacterName"`
	APIToken               string `json:"apiToken,omitempty"`
	CreatedAt              string `json:"createdAt,omitempty"`
	UpdatedAt              string `json:"updatedAt,omitempty"`
}

type bootstrapRequest struct {
	Handle                 string `json:"handle"`
	DisplayName            string `json:"displayName"`
	PreferredCharacterName string `json:"preferredCharacterName"`
}

type profileUpdateRequest struct {
	DisplayName            string `json:"displayName"`
	PreferredCharacterName string `json:"preferredCharacterName"`
}

func (s *Service) registerPrivateRoutes(app *fiber.App) {
	app.Post("/api/private/dev/bootstrap", s.handlePrivateBootstrap)
	app.Get("/api/private/me", s.handlePrivateMe)
	app.Post("/api/private/me/profile", s.handlePrivateProfileUpdate)
	app.Get("/api/private/dashboard", s.handlePrivateDashboard)
}

func (s *Service) handlePrivateBootstrap(c *fiber.Ctx) error {
	var req bootstrapRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_json"})
	}

	req.Handle = normalizeHandle(req.Handle)
	if req.Handle == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing_handle"})
	}

	user, err := s.createOrGetDevUser(req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "bootstrap_failed", "message": err.Error()})
	}
	return c.JSON(fiber.Map{"user": user})
}

func (s *Service) handlePrivateMe(c *fiber.Ctx) error {
	user, err := s.userFromRequest(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	return c.JSON(fiber.Map{"user": user})
}

func (s *Service) handlePrivateProfileUpdate(c *fiber.Ctx) error {
	user, err := s.userFromRequest(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	var req profileUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid_json"})
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = user.DisplayName
	}
	preferredCharacterName := strings.TrimSpace(req.PreferredCharacterName)

	_, err = s.db.Exec(`
		UPDATE app_users
		SET display_name = $2,
		    preferred_character_name = $3,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, user.ID, displayName, preferredCharacterName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "update_failed", "message": err.Error()})
	}

	updated, err := s.findUserByToken(user.APIToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "reload_failed", "message": err.Error()})
	}
	return c.JSON(fiber.Map{"user": updated})
}

func (s *Service) handlePrivateDashboard(c *fiber.Ctx) error {
	user, err := s.userFromRequest(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}

	characterName := strings.TrimSpace(user.PreferredCharacterName)
	if characterName == "" {
		characterName = strings.TrimSpace(user.DisplayName)
	}

	var playerState fiber.Map
	if characterName != "" {
		row := s.db.QueryRow(`
			SELECT character_id, character_name, guild_id, guild_name, location_id, updated_at
			FROM collector_player_state
			WHERE owner_user_id = $1 OR ($2 <> '' AND LOWER(character_name) = LOWER($2))
			ORDER BY updated_at DESC
			LIMIT 1
		`, user.ID, characterName)

		var characterID, resolvedName, guildID, guildName, locationID sql.NullString
		var updatedAt sql.NullTime
		err := row.Scan(&characterID, &resolvedName, &guildID, &guildName, &locationID, &updatedAt)
		if err == nil {
			playerState = fiber.Map{
				"characterId":   characterID.String,
				"characterName": resolvedName.String,
				"guildId":       guildID.String,
				"guildName":     guildName.String,
				"locationId":    locationID.String,
				"updatedAt":     updatedAt.Time.UTC().Format(time.RFC3339),
			}
		}
	}

	rows, err := s.db.Query(`
		SELECT event_id, session_id, location, completed_at, actor_character_name,
		       local_party_name, local_party_guild_name, local_party_silver, local_party_total,
		       remote_party_name, remote_party_guild_name, remote_party_silver, remote_party_total, net_profit
		FROM collector_trade_reports
		WHERE owner_user_id = $1 OR ($2 <> '' AND (LOWER(actor_character_name) = LOWER($2) OR LOWER(local_party_name) = LOWER($2)))
		ORDER BY completed_at DESC
		LIMIT 10
	`, user.ID, characterName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "dashboard_query_failed", "message": err.Error()})
	}
	defer rows.Close()

	type tradeItem struct {
		EventID         string `json:"eventId"`
		SessionID       string `json:"sessionId"`
		Location        string `json:"location"`
		CompletedAt     string `json:"completedAt"`
		LocalPartyName  string `json:"localPartyName"`
		RemotePartyName string `json:"remotePartyName"`
		NetProfit       int64  `json:"netProfit"`
	}

	trades := make([]tradeItem, 0, 10)
	for rows.Next() {
		var item tradeItem
		var actorName, localGuild, remoteGuild sql.NullString
		var localSilver, localTotal, remoteSilver, remoteTotal sql.NullInt64
		var completedAt sql.NullTime
		if err := rows.Scan(
			&item.EventID, &item.SessionID, &item.Location, &completedAt, &actorName,
			&item.LocalPartyName, &localGuild, &localSilver, &localTotal,
			&item.RemotePartyName, &remoteGuild, &remoteSilver, &remoteTotal, &item.NetProfit,
		); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "dashboard_scan_failed", "message": err.Error()})
		}
		if completedAt.Valid {
			item.CompletedAt = completedAt.Time.UTC().Format(time.RFC3339)
		}
		trades = append(trades, item)
	}

	recentEvents := make([]fiber.Map, 0, 10)
	eventRows, err := s.db.Query(`
		SELECT event_id, event_type, occurred_at, context_location_id
		FROM collector_events
		WHERE owner_user_id = $1 OR ($2 <> '' AND LOWER(actor_character_name) = LOWER($2))
		ORDER BY occurred_at DESC
		LIMIT 10
	`, user.ID, characterName)
	if err == nil {
		defer eventRows.Close()
		for eventRows.Next() {
			var eventID, eventType, locationID sql.NullString
			var occurredAt sql.NullTime
			if err := eventRows.Scan(&eventID, &eventType, &occurredAt, &locationID); err == nil {
				recentEvents = append(recentEvents, fiber.Map{
					"eventId":    eventID.String,
					"eventType":  eventType.String,
					"occurredAt": occurredAt.Time.UTC().Format(time.RFC3339),
					"locationId": locationID.String,
				})
			}
		}
	}

	return c.JSON(fiber.Map{
		"user":                user,
		"playerState":         playerState,
		"recentTrades":        trades,
		"recentEvents":        recentEvents,
		"filter":              characterName,
		"collectorLaunchArgs": collectorLaunchArgs(user),
	})
}

func (s *Service) createOrGetDevUser(req bootstrapRequest) (appUser, error) {
	existing, err := s.findUserByHandle(req.Handle)
	if err == nil {
		return existing, nil
	}
	if err != sql.ErrNoRows {
		return appUser{}, err
	}

	id, err := uuid.NewV4()
	if err != nil {
		return appUser{}, err
	}
	tokenID, err := uuid.NewV4()
	if err != nil {
		return appUser{}, err
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = req.Handle
	}
	preferredCharacterName := strings.TrimSpace(req.PreferredCharacterName)
	apiToken := tokenID.String()

	_, err = s.db.Exec(`
		INSERT INTO app_users (id, handle, display_name, preferred_character_name, api_token)
		VALUES ($1, $2, $3, $4, $5)
	`, id.String(), req.Handle, displayName, preferredCharacterName, apiToken)
	if err != nil {
		return appUser{}, err
	}

	return s.findUserByToken(apiToken)
}

func (s *Service) userFromRequest(c *fiber.Ctx) (appUser, error) {
	authHeader := strings.TrimSpace(c.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		authHeader = strings.TrimSpace(authHeader[7:])
	}
	if authHeader == "" {
		authHeader = strings.TrimSpace(c.Query("token"))
	}
	if authHeader == "" {
		return appUser{}, sql.ErrNoRows
	}
	return s.findUserByToken(authHeader)
}

func (s *Service) findUserByHandle(handle string) (appUser, error) {
	return s.scanUser(s.db.QueryRow(`
		SELECT id, handle, display_name, preferred_character_name, api_token, created_at, updated_at
		FROM app_users
		WHERE handle = $1
	`, handle))
}

func (s *Service) findUserByToken(token string) (appUser, error) {
	return s.scanUser(s.db.QueryRow(`
		SELECT id, handle, display_name, preferred_character_name, api_token, created_at, updated_at
		FROM app_users
		WHERE api_token = $1
	`, token))
}

func (s *Service) scanUser(scanner interface {
	Scan(dest ...interface{}) error
}) (appUser, error) {
	var user appUser
	var preferredCharacterName sql.NullString
	var createdAt sql.NullTime
	var updatedAt sql.NullTime
	err := scanner.Scan(
		&user.ID,
		&user.Handle,
		&user.DisplayName,
		&preferredCharacterName,
		&user.APIToken,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return appUser{}, err
	}
	if preferredCharacterName.Valid {
		user.PreferredCharacterName = preferredCharacterName.String
	}
	if createdAt.Valid {
		user.CreatedAt = createdAt.Time.UTC().Format(time.RFC3339)
	}
	if updatedAt.Valid {
		user.UpdatedAt = updatedAt.Time.UTC().Format(time.RFC3339)
	}
	return user, nil
}

func normalizeHandle(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func collectorLaunchArgs(user appUser) string {
	return "-collector-url http://localhost:8082/api/collector/events -collector-token " + user.APIToken
}
