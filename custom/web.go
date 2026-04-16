package custom

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

// ZIRHLI YAPI: Hem eski hem yeni etiketleri tanır
type AOItem struct {
	UniqueName         string            `json:"UniqueName"`
	UniqueNameAlt1     string            `json:"@uniquename"`
	UniqueNameAlt2     string            `json:"@UniqueName"`
	Index              interface{}       `json:"Index"`
	LocalizedNames     map[string]string `json:"LocalizedNames"`
	ShopCategory       string            `json:"shopCategory"`
	ShopCategoryAlt    string            `json:"@shopCategory"`
	ShopSubCategory1   string            `json:"shopSubCategory1"`
	ShopSubCategoryAlt string            `json:"@shopSubCategory1"`
}

func LoadDictionary() {
	fmt.Println("📚 AFM Tarzı Zırhlı Eşya Sözlüğü İndiriliyor...")

	// Sağlam ve orijinal Github adresi (JSON hatası vermeyen adres)
	resp, err := http.Get("https://raw.githubusercontent.com/ao-data/ao-bin-dumps/master/formatted/items.json")
	if err != nil {
		fmt.Println("❌ Sözlük İndirme Hatası:", err)
		return
	}
	defer resp.Body.Close()

	var items []AOItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err == nil {
		for _, item := range items {

			uName := item.UniqueName
			if uName == "" {
				uName = item.UniqueNameAlt1
			}
			if uName == "" {
				uName = item.UniqueNameAlt2
			}
			if uName == "" {
				continue
			}

			// 🚨 KRİTİK FİLTRE: Gerçek oyuncu eşyası mı? (İngilizce ismi var mı?)
			realName := uName
			if item.LocalizedNames != nil && item.LocalizedNames["EN-US"] != "" {
				realName = item.LocalizedNames["EN-US"]
			} else {
				continue // İsmi yoksa bu bir yetenek veya kod parçasıdır, sil gitsin!
			}

			idxStr := fmt.Sprintf("%v", item.Index)
			ItemNameToId[uName] = idxStr
			IdToItemName[idxStr] = uName
			ItemRealNames[uName] = realName

			shopCat := item.ShopCategory
			if shopCat == "" {
				shopCat = item.ShopCategoryAlt
			}
			subCat := item.ShopSubCategory1
			if subCat == "" {
				subCat = item.ShopSubCategoryAlt
			}

			cat, sub := getAfmCategory(shopCat, subCat, uName)

			// 🛡️ ZIRHLI KATEGORİ TAHMİN EDİCİ 🛡️
			// Eğer dışarıdan kategori verisi silinmişse veya boş gelirse, eşyanın kodundan biz buluruz!
			if cat == "Other" || cat == "" {
				if strings.Contains(uName, "_MAIN_") || strings.Contains(uName, "_2H_") {
					cat = "Weapons"
				} else if strings.Contains(uName, "_HEAD_") {
					cat = "Head Armor"
				} else if strings.Contains(uName, "_ARMOR_") {
					cat = "Chest Armor"
				} else if strings.Contains(uName, "_SHOES_") {
					cat = "Foot Armor"
				} else if strings.Contains(uName, "_BAG") {
					cat = "Bags"
				} else if strings.Contains(uName, "_CAPE") {
					cat = "Capes"
				} else if strings.Contains(uName, "_OFF_") {
					cat = "Off-Hands"
				} else if strings.Contains(uName, "_RING") || strings.Contains(uName, "_NECKLACE") || strings.Contains(uName, "_TRINKET") {
					cat = "Accessories"
				}
			}

			tier := 0
			if len(uName) >= 2 && uName[0] == 'T' && uName[1] >= '1' && uName[1] <= '8' {
				tier = int(uName[1] - '0')
			}

			enchant := 0
			if idx := strings.Index(uName, "@"); idx != -1 && idx+1 < len(uName) {
				enchant = int(uName[idx+1] - '0')
			}

			displayName := fmt.Sprintf("[T%d] %s", tier, realName)
			if enchant > 0 {
				displayName += fmt.Sprintf(" .%d", enchant)
			}

			ItemList = append(ItemList, ItemInfo{
				UniqueName:  uName,
				DisplayName: displayName,
				Tier:        tier,
				Enchant:     enchant,
				Category:    cat,
				SubCategory: sub,
			})
		}
		fmt.Printf("✅ Sözlük Hazır! Tam %d adet GEÇERLİ eşya yüklendi ve Flipper'a bağlandı.\n", len(ItemList))
	} else {
		fmt.Println("❌ JSON HATA:", err)
	}
}

func StartWebServer() {
	LoadDictionary()
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Use(cors.New())

	SetupFlipperRoutes(app)

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
