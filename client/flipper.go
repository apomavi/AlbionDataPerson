package client

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

type FlipResult struct {
	ItemID        string  `json:"item_id"`
	ItemName      string  `json:"item_name"`
	Tier          int     `json:"tier"`
	Enchant       int     `json:"enchant"` // 🚨 DÜZELTME: Eklemeyi unuttuğumuz satır
	Quality       int     `json:"quality"`
	BuyFromLoc    string  `json:"buy_from_location"`
	SellToLoc     string  `json:"sell_to_location"`
	BuyPrice      int64   `json:"buy_price"`
	SellPrice     int64   `json:"sell_price"`
	BuyAmount     int64   `json:"buy_amount"`
	SellAmount    int64   `json:"sell_amount"`
	ProfitPremium int64   `json:"profit_premium"`
	ProfitNormal  int64   `json:"profit_normal"`
	ProfitPct     float64 `json:"profit_percentage"`
	BuyAgeMins    int     `json:"buy_age_mins"`
	SellAgeMins   int     `json:"sell_age_mins"`
	BuySource     string  `json:"buy_source"`
	SellSource    string  `json:"sell_source"`
}

type AODPPrice struct {
	ItemID           string `json:"item_id"`
	City             string `json:"city"`
	Quality          int    `json:"quality"`
	SellPriceMin     int64  `json:"sell_price_min"`
	SellPriceMinDate string `json:"sell_price_min_date"`
	BuyPriceMax      int64  `json:"buy_price_max"`
	BuyPriceMaxDate  string `json:"buy_price_max_date"`
}

type PricePoint struct {
	ItemID   string
	Enchant  int
	Quality  int
	Price    int64
	Amount   int64
	AgeMins  int
	Source   string
	Location string
}

var (
	aodpCacheBM    map[string]PricePoint
	aodpCacheRoyal map[string]map[string]PricePoint
	lastAODPFetch  time.Time
	aodpCacheMutex sync.RWMutex
)

func SetupFlipperRoutes(app *fiber.App) {
	app.Get("/api/flipper", func(c *fiber.Ctx) error {
		useAODP := c.Query("use_aodp", "false")

		bmPool := make(map[string]PricePoint)
		royalPool := make(map[string]map[string]PricePoint)

		// =====================================================================
		// 1. AŞAMA: AODP KÜRESEL VERİLERİNİ HAVUZA EKLE
		// =====================================================================
		if useAODP == "true" {
			aodpCacheMutex.Lock()
			if aodpCacheBM == nil || time.Since(lastAODPFetch) > 3*time.Minute {
				fmt.Println("🌐 AODP API'den taze veri çekiliyor...")

				tempCacheBM := make(map[string]PricePoint)
				tempCacheRoyal := make(map[string]map[string]PricePoint)

				var targetItems []string
				for _, item := range ItemList {
					if item.Tier >= 4 {
						cat := item.Category
						if cat == "Weapons" || cat == "Head Armor" || cat == "Chest Armor" || cat == "Foot Armor" || cat == "Armor" || cat == "Capes" || cat == "Bags" || cat == "Accessories" || cat == "Off-Hands" {
							targetItems = append(targetItems, item.UniqueName)
						}
					}
				}

				var wg sync.WaitGroup
				var mu sync.Mutex
				sem := make(chan struct{}, 5)

				chunkSize := 120
				for i := 0; i < len(targetItems); i += chunkSize {
					end := i + chunkSize
					if end > len(targetItems) {
						end = len(targetItems)
					}
					chunk := targetItems[i:end]

					wg.Add(1)
					go func(itemChunk []string) {
						defer wg.Done()
						sem <- struct{}{}
						defer func() { <-sem }()

						itemsStr := strings.Join(itemChunk, ",")
						apiUrl := "https://europe.albion-online-data.com/api/v2/stats/Prices/" + itemsStr + "?locations=Black%20Market,Lymhurst,Martlock,Bridgewatch,Fort%20Sterling,Thetford"

						client := http.Client{Timeout: 15 * time.Second}
						resp, err := client.Get(apiUrl)
						if err != nil {
							return
						}
						defer resp.Body.Close()

						var aodpData []AODPPrice
						if json.NewDecoder(resp.Body).Decode(&aodpData) == nil {
							mu.Lock()
							for _, d := range aodpData {
								key := fmt.Sprintf("%s_%d", d.ItemID, d.Quality)
								enchant := 0
								if idx := strings.Index(d.ItemID, "@"); idx != -1 && idx+1 < len(d.ItemID) {
									enchant, _ = strconv.Atoi(string(d.ItemID[idx+1:]))
								}

								if d.City == "Black Market" && d.BuyPriceMax > 0 {
									sellDate, _ := time.Parse("2006-01-02T15:04:05", d.BuyPriceMaxDate)
									age := int(time.Since(sellDate).Minutes())
									if age < 0 || age > 10000 {
										age = 999
									}
									tempCacheBM[key] = PricePoint{ItemID: d.ItemID, Enchant: enchant, Quality: d.Quality, Price: d.BuyPriceMax, Amount: 1, AgeMins: age, Source: "AODP 🌐", Location: "Black Market"}
								} else if d.City != "Black Market" && d.City != "Caerleon" && d.SellPriceMin > 0 {
									buyDate, _ := time.Parse("2006-01-02T15:04:05", d.SellPriceMinDate)
									age := int(time.Since(buyDate).Minutes())
									if age < 0 || age > 10000 {
										age = 999
									}

									if tempCacheRoyal[key] == nil {
										tempCacheRoyal[key] = make(map[string]PricePoint)
									}
									tempCacheRoyal[key][d.City] = PricePoint{ItemID: d.ItemID, Enchant: enchant, Quality: d.Quality, Price: d.SellPriceMin, Amount: 1, AgeMins: age, Source: "AODP 🌐", Location: d.City}
								}
							}
							mu.Unlock()
						}
					}(chunk)
				}
				wg.Wait()
				aodpCacheBM = tempCacheBM
				aodpCacheRoyal = tempCacheRoyal
				lastAODPFetch = time.Now()
			}

			for k, v := range aodpCacheBM {
				bmPool[k] = v
			}
			for k, vMap := range aodpCacheRoyal {
				if royalPool[k] == nil {
					royalPool[k] = make(map[string]PricePoint)
				}
				for loc, p := range vMap {
					royalPool[k][loc] = p
				}
			}
			aodpCacheMutex.Unlock()
		}

		// =====================================================================
		// 2. AŞAMA: LOCAL VERİLERLE HAVUZU GÜNCELLE (ZIRHLI MİKTAR KORUMASI)
		// =====================================================================
		query := `
			SELECT 
				item_id, quality_level, location, price, amount, auction_type,
				COALESCE(EXTRACT(EPOCH FROM (NOW() AT TIME ZONE 'UTC' - updated_at))/60, 999) as age
			FROM market_orders
			WHERE auction_type IN ('request', 'offer', '"request"', '"offer"')
		`
		rows, err := db.Query(query)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var itemID sql.NullString
				var quality sql.NullInt32
				var locInt sql.NullInt32
				var price sql.NullInt64
				var amount sql.NullInt64
				var aucType sql.NullString
				var ageFloat sql.NullFloat64

				if err := rows.Scan(&itemID, &quality, &locInt, &price, &amount, &aucType, &ageFloat); err != nil {
					continue
				}
				if !itemID.Valid || !price.Valid {
					continue
				}

				cleanItemID := strings.Trim(itemID.String, "\" ")
				cleanAuc := strings.ToLower(strings.Trim(aucType.String, "\" "))

				age := int(ageFloat.Float64)
				if age < 0 {
					age = 0
				}

				enchant := 0
				if idx := strings.Index(cleanItemID, "@"); idx != -1 && idx+1 < len(cleanItemID) {
					enchant, _ = strconv.Atoi(string(cleanItemID[idx+1:]))
				}

				locID := int(locInt.Int32)
				locStr := strconv.Itoa(locID)
				locName := LocationNames[locStr]
				if locName == "" {
					locName = locStr
				}

				key := fmt.Sprintf("%s_%d", cleanItemID, int(quality.Int32))

				// 🛡️ LOCAL KALKANI: Eğer bizim verimiz 120 dakikadan yeniyse, AODP'yi KESİN EZ!
				if cleanAuc == "request" && locID == 3003 {
					existing, exists := bmPool[key]
					if !exists || age <= 120 || price.Int64 > existing.Price {
						bmPool[key] = PricePoint{ItemID: cleanItemID, Enchant: enchant, Quality: int(quality.Int32), Price: price.Int64, Amount: amount.Int64, AgeMins: age, Source: "LOCAL 🛡️", Location: "Black Market"}
					}
				} else if cleanAuc == "offer" {
					if royalPool[key] == nil {
						royalPool[key] = make(map[string]PricePoint)
					}
					existing, exists := royalPool[key][locName]
					if !exists || age <= 120 || price.Int64 < existing.Price {
						royalPool[key][locName] = PricePoint{ItemID: cleanItemID, Enchant: enchant, Quality: int(quality.Int32), Price: price.Int64, Amount: amount.Int64, AgeMins: age, Source: "LOCAL 🛡️", Location: locName}
					}
				}
			}
		}

		// =====================================================================
		// 3. AŞAMA: KÂRLARI HESAPLA
		// =====================================================================
		var finalResults []FlipResult
		for key, bm := range bmPool {
			for _, royal := range royalPool[key] {

				profitPremium := int64(float64(bm.Price)*0.96) - royal.Price
				profitNormal := int64(float64(bm.Price)*0.92) - royal.Price

				if profitPremium > 0 {
					itemID := bm.ItemID
					tier := 0
					if len(itemID) >= 2 && itemID[0] == 'T' {
						tier = int(itemID[1] - '0')
					}
					realName := ItemRealNames[itemID]
					if realName == "" {
						realName = itemID
					}

					finalResults = append(finalResults, FlipResult{
						ItemID:        itemID,
						ItemName:      realName,
						Tier:          tier,
						Enchant:       bm.Enchant,
						Quality:       bm.Quality,
						BuyFromLoc:    royal.Location,
						SellToLoc:     bm.Location,
						BuyPrice:      royal.Price,
						SellPrice:     bm.Price,
						BuyAmount:     royal.Amount,
						SellAmount:    bm.Amount,
						ProfitPremium: profitPremium,
						ProfitNormal:  profitNormal,
						ProfitPct:     (float64(profitPremium) / float64(royal.Price)) * 100,
						BuyAgeMins:    royal.AgeMins,
						SellAgeMins:   bm.AgeMins,
						BuySource:     royal.Source,
						SellSource:    bm.Source,
					})
				}
			}
		}

		sort.Slice(finalResults, func(i, j int) bool {
			return finalResults[i].ProfitPremium > finalResults[j].ProfitPremium
		})

		return c.JSON(finalResults)
	})
}
