package client

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

func ConnectDB() {
	connStr := "host=localhost port=5432 user=postgres password=5447495353aA. dbname=postgres sslmode=disable"
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Veritabanına bağlanılamadı: %v", err)
	}

	// Yeni tablo yapısına uygun ekstra sütunlar (AODP ile çakışmaması için IF NOT EXISTS kullanıyoruz)
	setupQueries := []string{
		"ALTER TABLE market_orders ADD COLUMN IF NOT EXISTS item_name TEXT;",
		"ALTER TABLE market_orders ADD COLUMN IF NOT EXISTS tier TEXT;",
		"ALTER TABLE market_history ADD COLUMN IF NOT EXISTS item_name TEXT;",
		"ALTER TABLE market_history ADD COLUMN IF NOT EXISTS tier TEXT;",
	}
	for _, q := range setupQueries {
		db.Exec(q)
	}

	// Dublicate (Çift) veri yazımını engellemek için kuralları ekliyoruz
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

	fmt.Println("✅ Veritabanı (AODP Uyumlu Orders ve History Tabloları) Hazır!")
}

// 1. FONKSİYON: Aktif Market İlanları (Orders)
func SaveToDatabase(topic string, jsonData []byte, state *albionState) {
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

	// Konum 0 gelirse, harfleri silip sayıları çıkaran akıllı fonksiyonumuzu kullan!
	locIdSayı := ParseLocationID(CurrentMap)
	if locIdSayı == 0 {
		locIdSayı = ParseLocationID(state.LocationId)
	}

	// 🛡️ GÜVENLİK DUVARI: Eğer konum hala 0 ise iptal et
	if locIdSayı == 0 {
		fmt.Printf("⚠️ KONUM BULUNAMADI! (Gelen Ham Konum: %s) İlan verileri yoksayıldı.\n", CurrentMap)
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

		_, err := db.Exec(insertQuery, order.Id, order.ItemTypeId, locIdSayı, order.QualityLevel, order.EnchantmentLevel, (order.UnitPriceSilver / 10000), order.Amount, order.Amount, order.AuctionType, order.Expires, realName, tier)

		// Hata yakalayıcıyı ekledik!
		if err != nil {
			log.Printf("❌ Orders DB Hatası (AlbionID: %d): %v", order.Id, err)
		}
	}
	fmt.Printf("🎯 %d adet DETAYLI ilan verisi market_orders tablosuna yazıldı!\n", len(payload.Orders))
}

// 2. FONKSİYON: Geçmiş Satış Hacimleri (History)
func SaveHistoryToDatabase(topic string, jsonData []byte, state *albionState) {
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
		log.Printf("❌ History çözülemedi: %v", err)
		return
	}

	// Konum 0 gelirse, harfleri silip sayıları çıkaran akıllı fonksiyonumuzu kullan!
	locId := ParseLocationID(CurrentMap)
	if locId == 0 {
		locId = ParseLocationID(payload.LocationId)
	}

	// 🛡️ GÜVENLİK DUVARI: Eğer konum hala 0 ise iptal et
	if locId == 0 {
		fmt.Printf("⚠️ KONUM BULUNAMADI! (Gelen Ham Konum: %s) Geçmiş satış verisi yoksayıldı.\n", CurrentMap)
		return
	}

	itemIdStr := strconv.Itoa(payload.AlbionId)
	uniqueName := IdToItemName[itemIdStr]
	if uniqueName == "" {
		uniqueName = itemIdStr
	}

	tier := ""
	if len(uniqueName) >= 2 && uniqueName[0] == 'T' {
		tier = uniqueName[:2]
	}

	realName := ItemRealNames[uniqueName]
	if realName == "" {
		realName = uniqueName
	}

	// 🚨 BÜYÜK DÜZELTME: Albion Timescale ID'sini, AODP Aggregation Saatine Çevirme 🚨
	var trueAggregation int
	switch payload.Timescale {
	case 0:
		trueAggregation = 1 // 1 Saatlik Dilim
	case 1:
		trueAggregation = 6 // 6 Saatlik Dilim (AODP'deki o meşhur 6 rakamı)
	case 2:
		trueAggregation = 24 // Günlük Dilim (24 saat)
	default:
		trueAggregation = payload.Timescale // Bilinmeyen bir şey gelirse olduğu gibi bırak
	}

	for _, dp := range payload.MarketHistories {
		if dp.ItemAmount == 0 {
			continue
		}

		unixTimeSeconds := (dp.Timestamp / 10000000) - 62135596800
		if unixTimeSeconds < 0 {
			unixTimeSeconds = 0
		}
		gercekTarih := time.Unix(unixTimeSeconds, 0)

		query := `
		INSERT INTO market_history (item_amount, silver_amount, item_id, location, quality, timestamp, aggregation, item_name, tier)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (item_id, location, quality, timestamp, aggregation) DO NOTHING;`

		// Burada payload.Timescale yerine çevirdiğimiz trueAggregation'ı kullanıyoruz
		_, err := db.Exec(query, dp.ItemAmount, (dp.SilverAmount / 10000), uniqueName, locId, payload.QualityLevel, gercekTarih, trueAggregation, realName, tier)
		if err != nil {
			log.Printf("History DB Hatası: %v", err)
		}
	}
	fmt.Printf("📊 Eşya (%s) için %d adet SATIŞ GEÇMİŞİ (Aggregation: %d) DB'ye yazıldı!\n", uniqueName, len(payload.MarketHistories), trueAggregation)
}

// ---------------------------------------------------------
// 🧠 AKILLI KONUM FİLTRESİ (Harfleri Siler, ID'yi Kurtarır)
// ---------------------------------------------------------
func ParseLocationID(loc string) int {
	// Özel durum: Black Market (AODP'de 30002 olarak geçer)
	if strings.Contains(strings.ToLower(loc), "black market") || loc == "30002" {
		return 30002
	}

	// "BLACKBANK-0353" gibi metinlerden harfleri çöpe at, sadece "0353" rakamlarını al
	numericStr := ""
	for _, char := range loc {
		if char >= '0' && char <= '9' {
			numericStr += string(char)
		}
	}

	// Kalan rakamı tam sayıya (Integer) çevir
	if numericStr != "" {
		val, _ := strconv.Atoi(numericStr)
		return val
	}
	return 0
}
