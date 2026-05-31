package backend

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ao-data/albiondata-client/lib"
)

func (s *Service) ensureLegacyMarketSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS market_orders (
			albion_id BIGINT PRIMARY KEY,
			item_id TEXT NOT NULL,
			location INTEGER NOT NULL,
			quality_level INTEGER NOT NULL,
			enchantment_level INTEGER NOT NULL DEFAULT 0,
			price BIGINT NOT NULL,
			initial_amount INTEGER NOT NULL DEFAULT 0,
			amount INTEGER NOT NULL DEFAULT 0,
			auction_type TEXT NOT NULL,
			expires TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			item_name TEXT,
			tier TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS market_history (
			item_amount BIGINT NOT NULL,
			silver_amount BIGINT NOT NULL,
			item_id TEXT NOT NULL,
			location INTEGER NOT NULL,
			quality INTEGER NOT NULL,
			timestamp TIMESTAMPTZ NOT NULL,
			aggregation INTEGER NOT NULL,
			item_name TEXT,
			tier TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS gold_prices (
			timestamp TIMESTAMPTZ PRIMARY KEY,
			price INTEGER NOT NULL
		);`,
		`ALTER TABLE market_orders ADD COLUMN IF NOT EXISTS item_name TEXT;`,
		`ALTER TABLE market_orders ADD COLUMN IF NOT EXISTS tier TEXT;`,
		`ALTER TABLE market_history ADD COLUMN IF NOT EXISTS item_name TEXT;`,
		`ALTER TABLE market_history ADD COLUMN IF NOT EXISTS tier TEXT;`,
		`DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'unique_history_entry') THEN
				ALTER TABLE market_history ADD CONSTRAINT unique_history_entry UNIQUE (item_id, location, quality, timestamp, aggregation);
			END IF;
		END $$;`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) projectLegacyMarketData(event storedEvent) error {
	switch event.EventType {
	case "albion.market.orders_snapshot":
		return s.projectLegacyOrders(event)
	case "albion.market.history_snapshot":
		return s.projectLegacyHistory(event)
	case "albion.market.gold_snapshot":
		return s.projectLegacyGold(event)
	default:
		return nil
	}
}

func (s *Service) projectLegacyOrders(event storedEvent) error {
	var payload lib.MarketUpload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	catalog, _ := loadCatalog()

	for _, order := range payload.Orders {
		if order == nil {
			continue
		}
		locationID := parseLegacyLocationID(firstNonEmpty(order.LocationID, event.Context.CurrentMap, event.Context.LocationID))
		if locationID == 0 {
			continue
		}

		itemName := order.ItemID
		if catalog != nil {
			if resolved := strings.TrimSpace(catalog.ItemRealNames[order.ItemID]); resolved != "" {
				itemName = resolved
			}
		}

		tier := ""
		if parsedTier := parseTier(order.ItemID); parsedTier > 0 {
			tier = fmt.Sprintf("T%d", parsedTier)
		}

		_, err := s.db.Exec(`
			INSERT INTO market_orders (
				albion_id, item_id, location, quality_level, enchantment_level, price,
				initial_amount, amount, auction_type, expires, created_at, updated_at, item_name, tier
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9, $10, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $11, $12
			)
			ON CONFLICT (albion_id) DO UPDATE SET
				item_id = EXCLUDED.item_id,
				location = EXCLUDED.location,
				quality_level = EXCLUDED.quality_level,
				enchantment_level = EXCLUDED.enchantment_level,
				price = EXCLUDED.price,
				initial_amount = EXCLUDED.initial_amount,
				amount = EXCLUDED.amount,
				auction_type = EXCLUDED.auction_type,
				expires = EXCLUDED.expires,
				updated_at = CURRENT_TIMESTAMP,
				item_name = EXCLUDED.item_name,
				tier = EXCLUDED.tier
		`, order.ID, order.ItemID, locationID, order.QualityLevel, order.EnchantmentLevel, order.Price/10000, order.Amount, order.Amount, order.AuctionType, order.Expires, itemName, tier)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) projectLegacyHistory(event storedEvent) error {
	var payload lib.MarketHistoriesUpload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	catalog, _ := loadCatalog()

	locationID := parseLegacyLocationID(firstNonEmpty(payload.LocationId, event.Context.CurrentMap, event.Context.LocationID))
	if locationID == 0 {
		return nil
	}

	itemID := strconv.Itoa(int(payload.AlbionId))
	itemName := itemID
	if catalog != nil {
		if resolvedItemID := strings.TrimSpace(catalog.IDToItemName[itemID]); resolvedItemID != "" {
			itemID = resolvedItemID
		}
		if resolvedName := strings.TrimSpace(catalog.ItemRealNames[itemID]); resolvedName != "" {
			itemName = resolvedName
		} else {
			itemName = itemID
		}
	}

	tier := ""
	if parsedTier := parseTier(itemID); parsedTier > 0 {
		tier = fmt.Sprintf("T%d", parsedTier)
	}

	aggregation := legacyAggregation(payload.Timescale)
	for _, history := range payload.Histories {
		if history == nil || history.ItemAmount == 0 {
			continue
		}

		timestamp := historyTimestampToTime(history.Timestamp)
		_, err := s.db.Exec(`
			INSERT INTO market_history (
				item_amount, silver_amount, item_id, location, quality, timestamp, aggregation, item_name, tier
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9
			)
			ON CONFLICT (item_id, location, quality, timestamp, aggregation) DO NOTHING
		`, history.ItemAmount, int64(history.SilverAmount)/10000, itemID, locationID, payload.QualityLevel, timestamp, aggregation, itemName, tier)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) projectLegacyGold(event storedEvent) error {
	var payload lib.GoldPricesUpload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}

	for index, price := range payload.Prices {
		if price <= 0 || index >= len(payload.TimeStamps) {
			continue
		}
		timestamp := goldTimestampToTime(payload.TimeStamps[index])
		_, err := s.db.Exec(`
			INSERT INTO gold_prices (timestamp, price)
			VALUES ($1, $2)
			ON CONFLICT (timestamp) DO UPDATE SET price = EXCLUDED.price
		`, timestamp, price)
		if err != nil {
			return err
		}
	}

	return nil
}

func parseLegacyLocationID(value string) int {
	if strings.Contains(strings.ToLower(value), "black market") || strings.TrimSpace(value) == "30002" {
		return 30002
	}

	var digits strings.Builder
	for _, char := range value {
		if char >= '0' && char <= '9' {
			digits.WriteRune(char)
		}
	}

	if digits.Len() == 0 {
		return 0
	}

	parsed, err := strconv.Atoi(digits.String())
	if err != nil {
		return 0
	}
	return parsed
}

func legacyAggregation(scale lib.Timescale) int {
	switch scale {
	case lib.Hours:
		return 1
	case lib.Days:
		return 6
	case lib.Weeks:
		return 24
	default:
		return int(scale)
	}
}

func historyTimestampToTime(value uint64) time.Time {
	unixSeconds := int64(value/10000000) - 62135596800
	if unixSeconds < 0 {
		unixSeconds = 0
	}
	return time.Unix(unixSeconds, 0).UTC()
}

func goldTimestampToTime(value int64) time.Time {
	if value > 10000000000000000 {
		return time.Unix(0, (value-621355968000000000)*100).UTC()
	}
	return time.Unix(0, value*1000000).UTC()
}
