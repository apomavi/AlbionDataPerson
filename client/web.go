package client

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

var ItemNameToId = make(map[string]string)
var IdToItemName = make(map[string]string)
var ItemRealNames = make(map[string]string)

type ItemInfo struct {
	UniqueName  string `json:"id"`
	DisplayName string `json:"name"`
	Tier        int    `json:"tier"`
	Enchant     int    `json:"enchant"`
	Category    string `json:"category"`
	SubCategory string `json:"subCategory"`
}

var ItemList = make([]ItemInfo, 0)

func formatSubCategory(s string) string {
	if len(s) == 0 {
		return ""
	}
	return strings.ToUpper(string(s[0])) + strings.ToLower(s[1:])
}

func getAfmCategory(shopCat, subCat, uniqueName string) (string, string) {
	shopCat = strings.ToLower(shopCat)
	subCat = strings.ToLower(subCat)
	uName := strings.ToUpper(uniqueName)

	switch shopCat {
	case "melee", "magic", "ranged":
		sub := "Other"
		if subCat != "" {
			sub = formatSubCategory(subCat)
		}
		return "Weapons", sub
	case "armor":
		if subCat == "head" {
			return "Head Armor", ""
		}
		if subCat == "chest" {
			return "Chest Armor", ""
		}
		if subCat == "shoes" {
			return "Foot Armor", ""
		}
		return "Armor", ""
	case "accessories":
		if subCat == "cape" {
			if strings.Contains(uName, "UNDEAD") {
				return "Capes", "Undead"
			}
			if strings.Contains(uName, "KEEPER") {
				return "Capes", "Keeper"
			}
			if strings.Contains(uName, "MORGANA") {
				return "Capes", "Morgana"
			}
			if strings.Contains(uName, "DEMON") {
				return "Capes", "Demon"
			}
			if strings.Contains(uName, "HERETIC") {
				return "Capes", "Heretic"
			}
			if strings.Contains(uName, "AVALON") {
				return "Capes", "Avalonian"
			}
			if strings.Contains(uName, "FW_") {
				return "Capes", "Faction"
			}
			return "Capes", "Cape"
		}
		if subCat == "bag" {
			return "Bags", ""
		}
		return "Accessories", ""
	case "offhand":
		sub := "Other"
		if subCat == "shield" {
			sub = "Shield"
		}
		if subCat == "book" {
			sub = "Tome"
		}
		if subCat == "horn" {
			sub = "Torch"
		}
		return "Off-Hands", sub
	case "mounts":
		return "Mount", ""
	case "gatherergear":
		return "Gathering Equipment", ""
	case "consumables":
		return "Consumables", ""
	case "materials", "crafting", "cityresources":
		return "Crafting", ""
	case "artifacts":
		return "Artifacts", ""
	case "farmables", "products":
		return "Farming", ""
	case "furniture":
		return "Furniture", ""
	case "vanity":
		return "Vanity", ""
	}
	return "Other", ""
}

// BÜYÜK DÜZELTME: Asla çökmeyen Struct yapısı ve @ tuzağını aşan özel etiketler!
type AOItem struct {
	UniqueName         string            `json:"UniqueName"`
	Index              interface{}       `json:"Index"`
	LocalizedNames     map[string]string `json:"LocalizedNames"`
	ShopCategory       string            `json:"shopCategory"`
	ShopCategoryAlt    string            `json:"@shopCategory"`
	ShopSubCategory1   string            `json:"shopSubCategory1"`
	ShopSubCategoryAlt string            `json:"@shopSubCategory1"`
}

func LoadDictionary() {
	fmt.Println("📚 AFM Tarzı Kusursuz Eşya Sözlüğü İndiriliyor...")
	resp, err := http.Get("https://raw.githubusercontent.com/ao-data/ao-bin-dumps/master/formatted/items.json")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var items []AOItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err == nil {
		for _, item := range items {
			if item.UniqueName == "" {
				continue
			}

			idxStr := fmt.Sprintf("%v", item.Index)
			ItemNameToId[item.UniqueName] = idxStr
			IdToItemName[idxStr] = item.UniqueName

			realName := item.UniqueName
			if item.LocalizedNames != nil && item.LocalizedNames["EN-US"] != "" {
				realName = item.LocalizedNames["EN-US"]
			}
			ItemRealNames[item.UniqueName] = realName

			// @ Tuzağını okuma
			shopCat := item.ShopCategory
			if shopCat == "" {
				shopCat = item.ShopCategoryAlt
			}
			subCat := item.ShopSubCategory1
			if subCat == "" {
				subCat = item.ShopSubCategoryAlt
			}
			if shopCat == "" {
				continue
			}

			cat, sub := getAfmCategory(shopCat, subCat, item.UniqueName)

			tier := 0
			if len(item.UniqueName) >= 2 && item.UniqueName[0] == 'T' && item.UniqueName[1] >= '1' && item.UniqueName[1] <= '8' {
				tier = int(item.UniqueName[1] - '0')
			}

			enchant := 0
			if idx := strings.Index(item.UniqueName, "@"); idx != -1 && idx+1 < len(item.UniqueName) {
				enchant = int(item.UniqueName[idx+1] - '0')
			}

			displayName := fmt.Sprintf("[T%d] %s", tier, realName)
			if enchant > 0 {
				displayName += fmt.Sprintf(" .%d", enchant)
			}

			ItemList = append(ItemList, ItemInfo{
				UniqueName:  item.UniqueName,
				DisplayName: displayName,
				Tier:        tier,
				Enchant:     enchant,
				Category:    cat,
				SubCategory: sub,
			})
		}
		fmt.Printf("✅ Sözlük Hazır! Tam %d adet eşya yüklendi ve ağaca yerleştirildi.\n", len(ItemList))
	} else {
		fmt.Println("❌ JSON HATA:", err)
	}
}

func StartWebServer() {
	LoadDictionary()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(cors.New())

	app.Get("/api/items", func(c *fiber.Ctx) error { return c.JSON(ItemList) })

	app.Get("/api/pricecheck/:item_id", func(c *fiber.Ctx) error {
		itemIdStr := strings.ToUpper(c.Params("item_id"))
		quality := c.Query("q", "1")

		type Order struct {
			Price  int64  `json:"price"`
			Amount int    `json:"amount"`
			Time   string `json:"time"`
		}
		type CityData struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
			Sell Order  `json:"sell"`
			Buy  Order  `json:"buy"`
		}

		cities := map[int]*CityData{
			30002: {Id: 30002, Name: "Black Market"}, 3003: {Id: 3003, Name: "Brecilien"}, 2000: {Id: 2000, Name: "Bridgewatch"},
			30004: {Id: 30004, Name: "Caerleon"}, 4000: {Id: 4000, Name: "Fort Sterling"}, 1000: {Id: 1000, Name: "Lymhurst"},
			3005: {Id: 3005, Name: "Martlock"}, 4: {Id: 4, Name: "Thetford"},
		}

		sellRows, _ := db.Query(`SELECT DISTINCT ON (location) location, price, amount, updated_at FROM market_orders WHERE item_id = $1 AND quality_level = $2 AND auction_type = 'offer' ORDER BY location, price ASC`, itemIdStr, quality)
		for sellRows.Next() {
			var loc, amount int
			var price int64
			var time string
			sellRows.Scan(&loc, &price, &amount, &time)
			if city, ok := cities[loc]; ok {
				city.Sell = Order{Price: price, Amount: amount, Time: time}
			}
		}
		sellRows.Close()

		buyRows, _ := db.Query(`SELECT DISTINCT ON (location) location, price, amount, updated_at FROM market_orders WHERE item_id = $1 AND quality_level = $2 AND auction_type = 'request' ORDER BY location, price DESC`, itemIdStr, quality)
		for buyRows.Next() {
			var loc, amount int
			var price int64
			var time string
			buyRows.Scan(&loc, &price, &amount, &time)
			if city, ok := cities[loc]; ok {
				city.Buy = Order{Price: price, Amount: amount, Time: time}
			}
		}
		buyRows.Close()

		var result []CityData
		orderKeys := []int{30002, 3003, 2000, 30004, 4000, 1000, 3005, 4}
		for _, locId := range orderKeys {
			result = append(result, *cities[locId])
		}
		return c.JSON(result)
	})

	app.Static("/", "./public")
	log.Println("🌐 AFM Klonu Başlatıldı: http://localhost:8081")
	log.Fatal(app.Listen(":8081"))
}
