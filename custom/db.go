package custom

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/ao-data/albiondata-client/custom/bridge"
	_ "github.com/lib/pq"
)

var db *sql.DB

func init() {
	bridge.RegisterHooks(bridge.Hooks{
		StartBackgroundServices: startCustomServices,
		OnPublicUpload:          handleCustomPublicUpload,
		OnDecodedResponse:       onDecodedResponse,
		OnDecodedEvent:          onDecodedEvent,
		OnJoinStateUpdated:      onJoinStateUpdated,
		OnGoldPrices:            onGoldPrices,
	})
}

func startCustomServices() {
	ConnectDB()
	StartWebServer()
}

func handleCustomPublicUpload(topic string, jsonData []byte, ctx bridge.UploadContext) {
	if strings.Contains(topic, "marketorders") {
		SaveToDatabase(topic, jsonData, ctx)
	}
	if strings.Contains(topic, "markethistories") {
		SaveHistoryToDatabase(topic, jsonData, ctx)
	}
}

func ConnectDB() {
	connStr := "host=localhost port=5432 user=postgres password=5447495353aA. dbname=postgres sslmode=disable"
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Veritabanina baglanilamadi: %v", err)
	}

	setupQueries := []string{
		"ALTER TABLE market_orders ADD COLUMN IF NOT EXISTS item_name TEXT;",
		"ALTER TABLE market_orders ADD COLUMN IF NOT EXISTS tier TEXT;",
		"ALTER TABLE market_history ADD COLUMN IF NOT EXISTS item_name TEXT;",
		"ALTER TABLE market_history ADD COLUMN IF NOT EXISTS tier TEXT;",
	}
	for _, q := range setupQueries {
		db.Exec(q)
	}

	db.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'unique_history_entry') THEN
				ALTER TABLE market_history ADD CONSTRAINT unique_history_entry UNIQUE (item_id, location, quality, timestamp, aggregation);
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'unique_albion_id') THEN
				ALTER TABLE market_orders ADD CONSTRAINT unique_albion_id UNIQUE (albion_id);
			END IF;
		END $$;
	`)

	fmt.Println("Veritabani (AODP uyumlu orders/history) hazir.")
}

func SaveToDatabase(topic string, jsonData []byte, ctx bridge.UploadContext) {
	if !strings.Contains(topic, "marketorders") {
		return
	}

	var payload struct {
		Orders []struct {
			Id               int64  `json:"Id"`
			ItemTypeId       string `json:"ItemTypeId"`
			QualityLevel     int    `json:"QualityLevel"`
			EnchantmentLevel int    `json:"EnchantmentLevel"`
			UnitPriceSilver  int64  `json:"UnitPriceSilver"`
			Amount           int    `json:"Amount"`
			AuctionType      string `json:"AuctionType"`
			Expires          string `json:"Expires"`
		} `json:"Orders"`
	}

	if err := json.Unmarshal(jsonData, &payload); err != nil {
		return
	}

	locID := ParseLocationID(ctx.CurrentMap)
	if locID == 0 {
		locID = ParseLocationID(ctx.LocationID)
	}
	if locID == 0 {
		fmt.Printf("Konum bulunamadi, ilan verisi atlandi. Ham konum: %s\n", ctx.CurrentMap)
		return
	}

	for _, order := range payload.Orders {
		tier := ""
		if len(order.ItemTypeId) >= 2 && order.ItemTypeId[0] == 'T' {
			tier = order.ItemTypeId[:2]
		}

		realName := ItemRealNames[order.ItemTypeId]
		if realName == "" {
			realName = order.ItemTypeId
		}

		insertQuery := `
		INSERT INTO market_orders (albion_id, item_id, location, quality_level, enchantment_level, price, initial_amount, amount, auction_type, expires, created_at, updated_at, item_name, tier)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $11, $12)
		ON CONFLICT (albion_id) DO UPDATE
		SET price = EXCLUDED.price, amount = EXCLUDED.amount, updated_at = CURRENT_TIMESTAMP, item_name = EXCLUDED.item_name, tier = EXCLUDED.tier;`

		_, err := db.Exec(insertQuery, order.Id, order.ItemTypeId, locID, order.QualityLevel, order.EnchantmentLevel, order.UnitPriceSilver/10000, order.Amount, order.Amount, order.AuctionType, order.Expires, realName, tier)
		if err != nil {
			log.Printf("Orders DB hatasi (AlbionID: %d): %v", order.Id, err)
		}
	}

	fmt.Printf("%d adet ilan market_orders tablosuna yazildi.\n", len(payload.Orders))
}

func SaveHistoryToDatabase(topic string, jsonData []byte, ctx bridge.UploadContext) {
	if !strings.Contains(topic, "markethistories") {
		return
	}

	var payload struct {
		AlbionId        int    `json:"AlbionId"`
		LocationId      string `json:"LocationId"`
		QualityLevel    int    `json:"QualityLevel"`
		Timescale       int    `json:"Timescale"`
		MarketHistories []struct {
			ItemAmount   int64 `json:"ItemAmount"`
			SilverAmount int64 `json:"SilverAmount"`
			Timestamp    int64 `json:"Timestamp"`
		} `json:"MarketHistories"`
	}

	if err := json.Unmarshal(jsonData, &payload); err != nil {
		log.Printf("History cozulmedi: %v", err)
		return
	}

	locID := ParseLocationID(ctx.CurrentMap)
	if locID == 0 {
		locID = ParseLocationID(payload.LocationId)
	}
	if locID == 0 {
		fmt.Printf("Konum bulunamadi, history verisi atlandi. Ham konum: %s\n", ctx.CurrentMap)
		return
	}

	itemIDStr := strconv.Itoa(payload.AlbionId)
	uniqueName := IdToItemName[itemIDStr]
	if uniqueName == "" {
		uniqueName = itemIDStr
	}

	tier := ""
	if len(uniqueName) >= 2 && uniqueName[0] == 'T' {
		tier = uniqueName[:2]
	}

	realName := ItemRealNames[uniqueName]
	if realName == "" {
		realName = uniqueName
	}

	var aggregation int
	switch payload.Timescale {
	case 0:
		aggregation = 1
	case 1:
		aggregation = 6
	case 2:
		aggregation = 24
	default:
		aggregation = payload.Timescale
	}

	for _, dp := range payload.MarketHistories {
		if dp.ItemAmount == 0 {
			continue
		}

		unixTimeSeconds := (dp.Timestamp / 10000000) - 62135596800
		if unixTimeSeconds < 0 {
			unixTimeSeconds = 0
		}
		timestamp := time.Unix(unixTimeSeconds, 0)

		query := `
		INSERT INTO market_history (item_amount, silver_amount, item_id, location, quality, timestamp, aggregation, item_name, tier)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (item_id, location, quality, timestamp, aggregation) DO NOTHING;`

		_, err := db.Exec(query, dp.ItemAmount, dp.SilverAmount/10000, uniqueName, locID, payload.QualityLevel, timestamp, aggregation, realName, tier)
		if err != nil {
			log.Printf("History DB hatasi: %v", err)
		}
	}

	fmt.Printf("%s icin %d adet history verisi DB'ye yazildi (agg=%d).\n", uniqueName, len(payload.MarketHistories), aggregation)
}

func ParseLocationID(loc string) int {
	if strings.Contains(strings.ToLower(loc), "black market") || loc == "30002" {
		return 30002
	}

	numeric := ""
	for _, char := range loc {
		if char >= '0' && char <= '9' {
			numeric += string(char)
		}
	}

	if numeric != "" {
		val, _ := strconv.Atoi(numeric)
		return val
	}
	return 0
}
