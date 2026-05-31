package backend

import (
	"crypto/tls"
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

type flipResult struct {
	ItemID        string  `json:"item_id"`
	ItemName      string  `json:"item_name"`
	Tier          int     `json:"tier"`
	Enchant       int     `json:"enchant"`
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

type aodpPrice struct {
	ItemID           string `json:"item_id"`
	City             string `json:"city"`
	Quality          int    `json:"quality"`
	SellPriceMin     int64  `json:"sell_price_min"`
	SellPriceMinDate string `json:"sell_price_min_date"`
	BuyPriceMax      int64  `json:"buy_price_max"`
	BuyPriceMaxDate  string `json:"buy_price_max_date"`
}

type pricePoint struct {
	ItemID   string
	Enchant  int
	Quality  int
	Price    int64
	Amount   int64
	AgeMins  int
	Source   string
	Location string
}

type priceCheckOrder struct {
	Price  int64  `json:"price"`
	Amount int    `json:"amount"`
	Time   string `json:"time"`
}

type priceCheckCity struct {
	ID   int             `json:"id"`
	Name string          `json:"name"`
	Sell priceCheckOrder `json:"sell"`
	Buy  priceCheckOrder `json:"buy"`
}

type citySlot struct {
	ID      int
	Name    string
	Aliases map[int]struct{}
}

var (
	aodpCacheMutex sync.RWMutex
	aodpCacheBM    map[string]pricePoint
	aodpCacheRoyal map[string]map[string]pricePoint
	lastAODPFetch  time.Time
)

var priceCheckCityOrder = []citySlot{
	{ID: 3003, Name: "Black Market", Aliases: setOf(3003)},
	{ID: 5003, Name: "Brecilien", Aliases: setOf(5000, 5002, 5003)},
	{ID: 2000, Name: "Bridgewatch", Aliases: setOf(2000, 2003, 2004)},
	{ID: 3005, Name: "Caerleon", Aliases: setOf(1, 3005, 3006)},
	{ID: 4000, Name: "Fort Sterling", Aliases: setOf(4000, 4001, 4002)},
	{ID: 1000, Name: "Lymhurst", Aliases: setOf(1000, 1001, 1002)},
	{ID: 3000, Name: "Martlock", Aliases: setOf(3004, 3007, 3008)},
	{ID: 4, Name: "Thetford", Aliases: setOf(0, 4, 6, 7)},
}

func (s *Service) registerMarketRoutes(app *fiber.App) {
	app.Get("/api/items", s.handleItems)
	app.Get("/api/pricecheck/:item_id", s.handlePriceCheck)
	app.Get("/api/flipper", s.handleFlipper)
}

func (s *Service) handleItems(c *fiber.Ctx) error {
	catalog, err := loadCatalog()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "catalog_unavailable", "message": err.Error()})
	}
	return c.JSON(catalog.ItemList)
}

func (s *Service) handlePriceCheck(c *fiber.Ctx) error {
	itemID := strings.ToUpper(strings.TrimSpace(c.Params("item_id")))
	quality := strings.TrimSpace(c.Query("q", "1"))
	maxAgeMins := c.QueryInt("max_age_mins", 1440)
	if maxAgeMins < 1 {
		maxAgeMins = 1440
	}

	result := make([]priceCheckCity, 0, len(priceCheckCityOrder))
	cityLookup := make(map[string]*priceCheckCity, len(priceCheckCityOrder))
	for _, slot := range priceCheckCityOrder {
		entry := priceCheckCity{ID: slot.ID, Name: slot.Name}
		result = append(result, entry)
		cityLookup[slot.Name] = &result[len(result)-1]
	}

	rows, err := s.db.Query(`
		SELECT location, price, amount, updated_at, auction_type
		FROM market_orders
		WHERE item_id = $1 AND quality_level = $2
		  AND auction_type IN ('offer', 'request', '"offer"', '"request"')
	`, itemID, quality)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "query_failed", "message": err.Error()})
	}
	defer rows.Close()

	for rows.Next() {
		var rawLocation sql.NullInt64
		var price sql.NullInt64
		var amount sql.NullInt64
		var updatedAt sql.NullTime
		var auctionType sql.NullString
		if err := rows.Scan(&rawLocation, &price, &amount, &updatedAt, &auctionType); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "scan_failed", "message": err.Error()})
		}
		if !updatedAt.Valid {
			continue
		}
		ageMins := int(time.Since(updatedAt.Time).Minutes())
		if ageMins < 0 {
			ageMins = 0
		}
		if ageMins > maxAgeMins {
			continue
		}

		cityName := mapLocationIDToCityName(int(rawLocation.Int64))
		entry := cityLookup[cityName]
		if entry == nil {
			continue
		}

		order := priceCheckOrder{
			Price:  price.Int64,
			Amount: int(amount.Int64),
		}
		if updatedAt.Valid {
			order.Time = updatedAt.Time.UTC().Format(time.RFC3339)
		}

		switch strings.ToLower(strings.Trim(auctionType.String, "\" ")) {
		case "offer":
			if entry.Sell.Price == 0 || order.Price < entry.Sell.Price || (order.Price == entry.Sell.Price && order.Time > entry.Sell.Time) {
				entry.Sell = order
			}
		case "request":
			if entry.Buy.Price == 0 || order.Price > entry.Buy.Price || (order.Price == entry.Buy.Price && order.Time > entry.Buy.Time) {
				entry.Buy = order
			}
		}
	}

	return c.JSON(result)
}

func (s *Service) handleFlipper(c *fiber.Ctx) error {
	useAODP := strings.EqualFold(strings.TrimSpace(c.Query("use_aodp", "false")), "true")
	cityFilter := strings.TrimSpace(c.Query("city_filter", "Hepsi"))

	catalog, err := loadCatalog()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "catalog_unavailable", "message": err.Error()})
	}

	bmPool := make(map[string]pricePoint)
	royalPool := make(map[string]map[string]pricePoint)

	if useAODP {
		if err := populateAODPPricePools(catalog, bmPool, royalPool); err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "aodp_fetch_failed", "message": err.Error()})
		}
	}

	rows, err := s.db.Query(`
		SELECT item_id, quality_level, location, price, amount, auction_type,
		       COALESCE(EXTRACT(EPOCH FROM (NOW() AT TIME ZONE 'UTC' - updated_at))/60, 999) AS age
		FROM market_orders
		WHERE auction_type IN ('request', 'offer', '"request"', '"offer"')
		  AND amount > 0
	`)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "query_failed", "message": err.Error()})
	}
	defer rows.Close()

	for rows.Next() {
		var itemID sql.NullString
		var quality sql.NullInt32
		var location sql.NullInt32
		var price sql.NullInt64
		var amount sql.NullInt64
		var auctionType sql.NullString
		var age sql.NullFloat64

		if err := rows.Scan(&itemID, &quality, &location, &price, &amount, &auctionType, &age); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "scan_failed", "message": err.Error()})
		}
		if !itemID.Valid || !price.Valid {
			continue
		}

		cleanItemID := strings.Trim(itemID.String, "\" ")
		cleanAuctionType := strings.ToLower(strings.Trim(auctionType.String, "\" "))
		qualityValue := int(quality.Int32)
		locationID := int(location.Int32)
		locationName := mapLocationIDToCityName(locationID)
		pointAge := int(age.Float64)
		if pointAge < 0 {
			pointAge = 0
		}

		key := fmt.Sprintf("%s_%d", cleanItemID, qualityValue)
		point := pricePoint{
			ItemID:   cleanItemID,
			Enchant:  parseEnchant(cleanItemID),
			Quality:  qualityValue,
			Price:    price.Int64,
			Amount:   amount.Int64,
			AgeMins:  pointAge,
			Source:   "LOCAL",
			Location: locationName,
		}

		if cleanAuctionType == "request" && locationName == "Black Market" {
			existing, exists := bmPool[key]
			if !exists || point.AgeMins <= existing.AgeMins {
				bmPool[key] = point
			}
			continue
		}

		if cleanAuctionType == "offer" {
			if royalPool[key] == nil {
				royalPool[key] = make(map[string]pricePoint)
			}
			existing, exists := royalPool[key][locationName]
			if !exists || point.AgeMins <= existing.AgeMins {
				royalPool[key][locationName] = point
			}
		}
	}

	results := make([]flipResult, 0)
	for key, bm := range bmPool {
		for locationName, royal := range royalPool[key] {
			if cityFilter != "" && cityFilter != "Hepsi" && locationName != cityFilter {
				continue
			}
			if royal.Price <= 0 {
				continue
			}

			profitPremium := int64(float64(bm.Price)*0.96) - royal.Price
			profitNormal := int64(float64(bm.Price)*0.92) - royal.Price
			if profitPremium <= 0 {
				continue
			}

			itemName := catalog.ItemRealNames[bm.ItemID]
			if itemName == "" {
				itemName = bm.ItemID
			}

			results = append(results, flipResult{
				ItemID:        bm.ItemID,
				ItemName:      itemName,
				Tier:          parseTier(bm.ItemID),
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

	sort.Slice(results, func(i int, j int) bool {
		return results[i].ProfitPremium > results[j].ProfitPremium
	})

	return c.JSON(results)
}

func populateAODPPricePools(catalog *itemCatalog, bmPool map[string]pricePoint, royalPool map[string]map[string]pricePoint) error {
	aodpCacheMutex.Lock()
	defer aodpCacheMutex.Unlock()

	if aodpCacheBM == nil || time.Since(lastAODPFetch) > 3*time.Minute {
		tempBM := make(map[string]pricePoint)
		tempRoyal := make(map[string]map[string]pricePoint)

		targetItems := make([]string, 0)
		for _, item := range catalog.ItemList {
			if item.Tier < 4 {
				continue
			}
			switch item.Category {
			case "Weapons", "Head Armor", "Chest Armor", "Foot Armor", "Armor", "Capes", "Bags", "Accessories", "Off-Hands":
				targetItems = append(targetItems, item.UniqueName)
			}
		}

		chunks := buildItemChunks(targetItems, 8000, "https://europe.albion-online-data.com/api/v2/stats/Prices/", "?locations=Black%20Market,Lymhurst,Martlock,Bridgewatch,Fort%20Sterling,Thetford")

		transport := &http.Transport{
			TLSNextProto: make(map[string]func(string, *tls.Conn) http.RoundTripper),
		}
		client := &http.Client{Transport: transport, Timeout: 20 * time.Second}

		var wg sync.WaitGroup
		var mu sync.Mutex
		sem := make(chan struct{}, 3)
		var fetchErr error

		for _, chunk := range chunks {
			chunk := chunk
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				itemsValue := strings.Join(chunk, ",")
				url := "https://europe.albion-online-data.com/api/v2/stats/Prices/" + itemsValue + "?locations=Black%20Market,Lymhurst,Martlock,Bridgewatch,Fort%20Sterling,Thetford"
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					mu.Lock()
					if fetchErr == nil {
						fetchErr = err
					}
					mu.Unlock()
					return
				}
				req.Header.Set("User-Agent", "AlbionPersonalBackend/1.0")

				resp, err := client.Do(req)
				if err != nil {
					mu.Lock()
					if fetchErr == nil {
						fetchErr = err
					}
					mu.Unlock()
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					mu.Lock()
					if fetchErr == nil {
						fetchErr = fmt.Errorf("unexpected AODP status: %d", resp.StatusCode)
					}
					mu.Unlock()
					return
				}

				var payload []aodpPrice
				if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
					mu.Lock()
					if fetchErr == nil {
						fetchErr = err
					}
					mu.Unlock()
					return
				}

				mu.Lock()
				defer mu.Unlock()
				for _, item := range payload {
					key := fmt.Sprintf("%s_%d", item.ItemID, item.Quality)
					if item.City == "Black Market" && item.BuyPriceMax > 0 {
						tempBM[key] = pricePoint{
							ItemID:   item.ItemID,
							Enchant:  parseEnchant(item.ItemID),
							Quality:  item.Quality,
							Price:    item.BuyPriceMax,
							Amount:   1,
							AgeMins:  parseAODPAge(item.BuyPriceMaxDate),
							Source:   "AODP",
							Location: "Black Market",
						}
						continue
					}
					if item.City == "Black Market" || item.City == "Caerleon" || item.SellPriceMin <= 0 {
						continue
					}
					if tempRoyal[key] == nil {
						tempRoyal[key] = make(map[string]pricePoint)
					}
					tempRoyal[key][item.City] = pricePoint{
						ItemID:   item.ItemID,
						Enchant:  parseEnchant(item.ItemID),
						Quality:  item.Quality,
						Price:    item.SellPriceMin,
						Amount:   1,
						AgeMins:  parseAODPAge(item.SellPriceMinDate),
						Source:   "AODP",
						Location: item.City,
					}
				}
			}()
			time.Sleep(200 * time.Millisecond)
		}

		wg.Wait()
		if fetchErr != nil {
			return fetchErr
		}

		aodpCacheBM = tempBM
		aodpCacheRoyal = tempRoyal
		lastAODPFetch = time.Now()
	}

	for key, value := range aodpCacheBM {
		bmPool[key] = value
	}
	for key, locationMap := range aodpCacheRoyal {
		if royalPool[key] == nil {
			royalPool[key] = make(map[string]pricePoint)
		}
		for location, point := range locationMap {
			royalPool[key][location] = point
		}
	}

	return nil
}

func buildItemChunks(items []string, maxURLLength int, baseURL string, suffix string) [][]string {
	maxItemsLen := maxURLLength - len(baseURL) - len(suffix)
	chunks := make([][]string, 0)
	current := make([]string, 0)
	currentLen := 0

	for _, item := range items {
		itemLen := len(item)
		if currentLen > 0 {
			itemLen++
		}
		if currentLen+itemLen > maxItemsLen && len(current) > 0 {
			chunks = append(chunks, current)
			current = []string{item}
			currentLen = len(item)
			continue
		}
		current = append(current, item)
		currentLen += itemLen
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}

	return chunks
}

func parseAODPAge(value string) int {
	if strings.TrimSpace(value) == "" {
		return 999
	}
	parsed, err := time.Parse("2006-01-02T15:04:05", value)
	if err != nil {
		return 999
	}
	age := int(time.Since(parsed).Minutes())
	if age < 0 || age > 10000 {
		return 999
	}
	return age
}

func mapLocationIDToCityName(locationID int) string {
	for _, slot := range priceCheckCityOrder {
		if _, ok := slot.Aliases[locationID]; ok {
			return slot.Name
		}
	}
	return strconv.Itoa(locationID)
}

func setOf(values ...int) map[int]struct{} {
	result := make(map[int]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
