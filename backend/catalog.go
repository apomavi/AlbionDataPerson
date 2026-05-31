package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type ItemInfo struct {
	UniqueName   string `json:"id"`
	DisplayName  string `json:"name"`
	Tier         int    `json:"tier"`
	Enchant      int    `json:"enchant"`
	Category     string `json:"category"`
	SubCategory  string `json:"subCategory"`
	SubCategory2 string `json:"subCategory2"`
}

type itemCatalog struct {
	ItemNameToID  map[string]string
	IDToItemName  map[string]string
	ItemRealNames map[string]string
	ItemList      []ItemInfo
}

type aoItem struct {
	UniqueName         string            `json:"UniqueName"`
	UniqueNameAlt1     string            `json:"@uniquename"`
	UniqueNameAlt2     string            `json:"@UniqueName"`
	Index              interface{}       `json:"Index"`
	LocalizedNames     map[string]string `json:"LocalizedNames"`
	ShopCategory       string            `json:"shopCategory"`
	ShopCategoryAlt    string            `json:"@shopCategory"`
	ShopSubCategory1   string            `json:"shopSubCategory1"`
	ShopSubCategoryAlt string            `json:"@shopSubCategory1"`
	ShopSubCategory2   string            `json:"shopSubCategory2"`
	ShopSubCategory2Alt string           `json:"@shopSubCategory2"`
}

var (
	catalogOnce sync.Once
	catalogData *itemCatalog
	catalogErr  error
)

func warmCatalog() {
	if _, err := loadCatalog(); err != nil {
		logrus.Errorf("Backend catalog load failed: %v", err)
	}
}

func loadCatalog() (*itemCatalog, error) {
	catalogOnce.Do(func() {
		client := &http.Client{Timeout: 20 * time.Second}
		resp, err := client.Get("https://raw.githubusercontent.com/ao-data/ao-bin-dumps/master/formatted/items.json")
		if err != nil {
			catalogErr = err
			return
		}
		defer resp.Body.Close()

		var items []aoItem
		if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
			catalogErr = err
			return
		}

		data := &itemCatalog{
			ItemNameToID:  make(map[string]string),
			IDToItemName:  make(map[string]string),
			ItemRealNames: make(map[string]string),
			ItemList:      make([]ItemInfo, 0, len(items)),
		}

		for _, item := range items {
			uniqueName := firstNonEmpty(item.UniqueName, item.UniqueNameAlt1, item.UniqueNameAlt2)
			if uniqueName == "" {
				continue
			}

			realName := uniqueName
			if item.LocalizedNames != nil {
				if localized := strings.TrimSpace(item.LocalizedNames["EN-US"]); localized != "" {
					realName = localized
				} else {
					continue
				}
			}

			indexStr := fmt.Sprintf("%v", item.Index)
			data.ItemNameToID[uniqueName] = indexStr
			data.IDToItemName[indexStr] = uniqueName
			data.ItemRealNames[uniqueName] = realName

			shopCategory := firstNonEmpty(item.ShopCategory, item.ShopCategoryAlt)
			shopSubCategory1 := firstNonEmpty(item.ShopSubCategory1, item.ShopSubCategoryAlt)
			shopSubCategory2 := firstNonEmpty(item.ShopSubCategory2, item.ShopSubCategory2Alt)
			category, subCategory, subCategory2 := getCatalogCategory(shopCategory, shopSubCategory1, shopSubCategory2, uniqueName)

			tier := parseTier(uniqueName)
			enchant := parseEnchant(uniqueName)

			displayName := fmt.Sprintf("[T%d] %s", tier, realName)
			if enchant > 0 {
				displayName += fmt.Sprintf(" .%d", enchant)
			}

			data.ItemList = append(data.ItemList, ItemInfo{
				UniqueName:   uniqueName,
				DisplayName:  displayName,
				Tier:         tier,
				Enchant:      enchant,
				Category:     category,
				SubCategory:  subCategory,
				SubCategory2: subCategory2,
			})
		}

		catalogData = data
		logrus.Infof("Backend item catalog ready with %d items", len(data.ItemList))
	})

	return catalogData, catalogErr
}

func getCatalogCategory(shopCategory string, subCategory1 string, subCategory2 string, uniqueName string) (string, string, string) {
	shopCategory = strings.ToLower(shopCategory)
	subCategory1 = strings.ToLower(subCategory1)
	subCategory2 = strings.ToLower(subCategory2)
	upperName := strings.ToUpper(uniqueName)

	if isToolItem(upperName) {
		return "Gathering Equipment", inferGatheringSubCategory(upperName, subCategory1, subCategory2), ""
	}

	switch shopCategory {
	case "melee", "magic", "ranged":
		sub := firstNonEmpty(formatCategoryValue(subCategory1), inferWeaponSubCategory(upperName))
		return "Weapons", sub, formatCategoryValue(subCategory2)
	case "armor":
		switch subCategory1 {
		case "head":
			return "Armor", "Head Armor", formatCategoryValue(subCategory2)
		case "chest":
			return "Armor", "Chest Armor", formatCategoryValue(subCategory2)
		case "shoes":
			return "Armor", "Foot Armor", formatCategoryValue(subCategory2)
		default:
			sub := firstNonEmpty(formatCategoryValue(subCategory1), inferArmorSubCategory(upperName))
			return "Armor", sub, formatCategoryValue(subCategory2)
		}
	case "accessories":
		sub := firstNonEmpty(formatAccessoryGroup(subCategory1, upperName), inferAccessorySubCategory(upperName))
		return "Accessories", sub, formatCategoryValue(subCategory2)
	case "offhand":
		sub := firstNonEmpty(inferOffhandSubCategory(subCategory1, upperName), "Off-Hand")
		return "Accessories", "Off-Hands", sub
	case "mounts":
		return "Mounts", firstNonEmpty(formatCategoryValue(subCategory1), inferMountSubCategory(upperName)), formatCategoryValue(subCategory2)
	case "gatherergear":
		return "Gathering Equipment", firstNonEmpty(formatCategoryValue(subCategory1), inferGatheringSubCategory(upperName, subCategory1, subCategory2)), formatCategoryValue(subCategory2)
	case "consumables":
		return "Consumables", firstNonEmpty(formatCategoryValue(subCategory1), inferConsumableSubCategory(upperName)), formatCategoryValue(subCategory2)
	case "materials", "crafting", "cityresources":
		return "Resources", firstNonEmpty(formatCategoryValue(subCategory1), inferResourceSubCategory(upperName)), formatCategoryValue(subCategory2)
	case "artifacts":
		return "Artifacts", firstNonEmpty(formatCategoryValue(subCategory1), inferArtifactSubCategory(upperName)), formatCategoryValue(subCategory2)
	case "farmables", "products":
		return "Farming", firstNonEmpty(formatCategoryValue(subCategory1), inferFarmingSubCategory(upperName)), formatCategoryValue(subCategory2)
	case "furniture":
		return "Furniture", formatCategoryValue(subCategory1), formatCategoryValue(subCategory2)
	case "vanity":
		return "Vanity", formatCategoryValue(subCategory1), formatCategoryValue(subCategory2)
	default:
		return inferCategoryFromUniqueName(uniqueName)
	}
}

func inferCategoryFromUniqueName(uniqueName string) (string, string, string) {
	switch {
	case isToolItem(uniqueName):
		return "Gathering Equipment", inferGatheringSubCategory(uniqueName, "", ""), ""
	case strings.Contains(uniqueName, "_MOUNT"):
		return "Mounts", inferMountSubCategory(uniqueName), ""
	case isConsumableItem(uniqueName):
		return "Consumables", inferConsumableSubCategory(uniqueName), ""
	case isResourceItem(uniqueName):
		return "Resources", inferResourceSubCategory(uniqueName), ""
	case strings.Contains(uniqueName, "_MAIN_"), strings.Contains(uniqueName, "_2H_"):
		return "Weapons", inferWeaponSubCategory(uniqueName), ""
	case strings.Contains(uniqueName, "_HEAD_"):
		return "Armor", "Head Armor", ""
	case strings.Contains(uniqueName, "_ARMOR_"):
		return "Armor", "Chest Armor", ""
	case strings.Contains(uniqueName, "_SHOES_"):
		return "Armor", "Foot Armor", ""
	case strings.Contains(uniqueName, "_BAG"):
		return "Accessories", "Bags", ""
	case strings.Contains(uniqueName, "_CAPE"):
		return "Accessories", "Capes", inferCapeSubCategory(uniqueName)
	case strings.Contains(uniqueName, "_OFF_"):
		return "Accessories", "Off-Hands", inferOffhandSubCategory("", uniqueName)
	case strings.Contains(uniqueName, "_RING"), strings.Contains(uniqueName, "_NECKLACE"), strings.Contains(uniqueName, "_TRINKET"):
		return "Accessories", "Jewelry", ""
	default:
		return "Other", "", ""
	}
}

func formatSubCategory(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + strings.ToLower(value[1:])
}

func formatCategoryValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "_", " ")
	parts := strings.Fields(strings.ToLower(value))
	for i, part := range parts {
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func isToolItem(uniqueName string) bool {
	return strings.Contains(uniqueName, "_TOOL_") || strings.Contains(uniqueName, "GATHERER") || strings.Contains(uniqueName, "FISHING")
}

func isConsumableItem(uniqueName string) bool {
	return strings.Contains(uniqueName, "_POTION") ||
		strings.Contains(uniqueName, "_MEAL") ||
		strings.Contains(uniqueName, "OMELETTE") ||
		strings.Contains(uniqueName, "SANDWICH") ||
		strings.Contains(uniqueName, "SOUP") ||
		strings.Contains(uniqueName, "STEW") ||
		strings.Contains(uniqueName, "SALAD") ||
		strings.Contains(uniqueName, "PIE")
}

func isResourceItem(uniqueName string) bool {
	return strings.Contains(uniqueName, "_ORE") ||
		strings.Contains(uniqueName, "_ROCK") ||
		strings.Contains(uniqueName, "_WOOD") ||
		strings.Contains(uniqueName, "_PLANKS") ||
		strings.Contains(uniqueName, "_CLOTH") ||
		strings.Contains(uniqueName, "_FIBER") ||
		strings.Contains(uniqueName, "_HIDE") ||
		strings.Contains(uniqueName, "_LEATHER") ||
		strings.Contains(uniqueName, "_METALBAR") ||
		strings.Contains(uniqueName, "_STONEBLOCK")
}

func inferWeaponSubCategory(uniqueName string) string {
	switch {
	case strings.Contains(uniqueName, "SWORD"):
		return "Sword"
	case strings.Contains(uniqueName, "AXE"):
		return "Axe"
	case strings.Contains(uniqueName, "MACE"):
		return "Mace"
	case strings.Contains(uniqueName, "HAMMER"):
		return "Hammer"
	case strings.Contains(uniqueName, "DAGGER"):
		return "Dagger"
	case strings.Contains(uniqueName, "SPEAR"):
		return "Spear"
	case strings.Contains(uniqueName, "BOW"):
		return "Bow"
	case strings.Contains(uniqueName, "CROSSBOW"):
		return "Crossbow"
	case strings.Contains(uniqueName, "FIRE"):
		return "Fire Staff"
	case strings.Contains(uniqueName, "FROST"):
		return "Frost Staff"
	case strings.Contains(uniqueName, "ARCANE"):
		return "Arcane Staff"
	case strings.Contains(uniqueName, "HOLY"):
		return "Holy Staff"
	case strings.Contains(uniqueName, "NATURE"):
		return "Nature Staff"
	case strings.Contains(uniqueName, "CURSE"):
		return "Cursed Staff"
	case strings.Contains(uniqueName, "QUARTERSTAFF"):
		return "Quarterstaff"
	case strings.Contains(uniqueName, "DUALSWORD"):
		return "Sword"
	default:
		return ""
	}
}

func inferArmorSubCategory(uniqueName string) string {
	switch {
	case strings.Contains(uniqueName, "_HEAD_"):
		return "Head Armor"
	case strings.Contains(uniqueName, "_ARMOR_"):
		return "Chest Armor"
	case strings.Contains(uniqueName, "_SHOES_"):
		return "Foot Armor"
	default:
		return ""
	}
}

func inferAccessorySubCategory(uniqueName string) string {
	switch {
	case strings.Contains(uniqueName, "_BAG"):
		return "Bags"
	case strings.Contains(uniqueName, "_CAPE"):
		return "Capes"
	case strings.Contains(uniqueName, "_OFF_"):
		return "Off-Hands"
	case strings.Contains(uniqueName, "_RING"), strings.Contains(uniqueName, "_NECKLACE"), strings.Contains(uniqueName, "_TRINKET"):
		return "Jewelry"
	default:
		return ""
	}
}

func formatAccessoryGroup(subCategory1 string, uniqueName string) string {
	switch subCategory1 {
	case "cape":
		return "Capes"
	case "bag":
		return "Bags"
	case "ring", "necklace", "trinket":
		return "Jewelry"
	default:
		return inferAccessorySubCategory(uniqueName)
	}
}

func inferCapeSubCategory(uniqueName string) string {
	switch {
	case strings.Contains(uniqueName, "UNDEAD"):
		return "Undead"
	case strings.Contains(uniqueName, "KEEPER"):
		return "Keeper"
	case strings.Contains(uniqueName, "MORGANA"):
		return "Morgana"
	case strings.Contains(uniqueName, "DEMON"):
		return "Demon"
	case strings.Contains(uniqueName, "HERETIC"):
		return "Heretic"
	case strings.Contains(uniqueName, "AVALON"):
		return "Avalonian"
	case strings.Contains(uniqueName, "FW_"):
		return "Faction"
	default:
		return "Cape"
	}
}

func inferOffhandSubCategory(subCategory1 string, uniqueName string) string {
	switch subCategory1 {
	case "shield":
		return "Shield"
	case "book":
		return "Tome"
	case "horn":
		return "Torch"
	}
	switch {
	case strings.Contains(uniqueName, "SHIELD"):
		return "Shield"
	case strings.Contains(uniqueName, "BOOK"):
		return "Tome"
	case strings.Contains(uniqueName, "TORCH"), strings.Contains(uniqueName, "HORN"):
		return "Torch"
	default:
		return "Off-Hand"
	}
}

func inferGatheringSubCategory(uniqueName string, subCategory1 string, subCategory2 string) string {
	switch {
	case strings.Contains(uniqueName, "AXE"):
		return "Axe"
	case strings.Contains(uniqueName, "PICK"):
		return "Pickaxe"
	case strings.Contains(uniqueName, "KNIFE"):
		return "Skinning Knife"
	case strings.Contains(uniqueName, "SICKLE"):
		return "Sickle"
	case strings.Contains(uniqueName, "HAMMER"):
		return "Stone Hammer"
	case strings.Contains(uniqueName, "ROD"):
		return "Fishing Rod"
	}
	return firstNonEmpty(formatCategoryValue(subCategory2), formatCategoryValue(subCategory1))
}

func inferMountSubCategory(uniqueName string) string {
	switch {
	case strings.Contains(uniqueName, "OX"):
		return "Ox"
	case strings.Contains(uniqueName, "HORSE"):
		return "Horse"
	case strings.Contains(uniqueName, "MULE"):
		return "Mule"
	case strings.Contains(uniqueName, "DIREBOAR"):
		return "Direboar"
	case strings.Contains(uniqueName, "SWAMPDRAGON"):
		return "Swamp Dragon"
	default:
		return ""
	}
}

func inferConsumableSubCategory(uniqueName string) string {
	switch {
	case strings.Contains(uniqueName, "POTION"):
		return "Potions"
	case strings.Contains(uniqueName, "OMELETTE"), strings.Contains(uniqueName, "STEW"), strings.Contains(uniqueName, "SOUP"), strings.Contains(uniqueName, "SALAD"), strings.Contains(uniqueName, "PIE"), strings.Contains(uniqueName, "SANDWICH"), strings.Contains(uniqueName, "MEAL"):
		return "Food"
	default:
		return ""
	}
}

func inferResourceSubCategory(uniqueName string) string {
	switch {
	case strings.Contains(uniqueName, "_ORE"), strings.Contains(uniqueName, "_METALBAR"):
		return "Ore & Metal"
	case strings.Contains(uniqueName, "_WOOD"), strings.Contains(uniqueName, "_PLANKS"):
		return "Wood & Planks"
	case strings.Contains(uniqueName, "_FIBER"), strings.Contains(uniqueName, "_CLOTH"):
		return "Fiber & Cloth"
	case strings.Contains(uniqueName, "_HIDE"), strings.Contains(uniqueName, "_LEATHER"):
		return "Hide & Leather"
	case strings.Contains(uniqueName, "_ROCK"), strings.Contains(uniqueName, "_STONEBLOCK"):
		return "Stone"
	default:
		return ""
	}
}

func inferArtifactSubCategory(uniqueName string) string {
	switch {
	case strings.Contains(uniqueName, "RUNE"):
		return "Runes"
	case strings.Contains(uniqueName, "SOUL"):
		return "Souls"
	case strings.Contains(uniqueName, "RELIC"):
		return "Relics"
	case strings.Contains(uniqueName, "AVALON"):
		return "Avalonian"
	default:
		return ""
	}
}

func inferFarmingSubCategory(uniqueName string) string {
	switch {
	case strings.Contains(uniqueName, "SEED"):
		return "Seeds"
	case strings.Contains(uniqueName, "ANIMAL"), strings.Contains(uniqueName, "BABY"):
		return "Animals"
	default:
		return ""
	}
}

func parseTier(uniqueName string) int {
	if len(uniqueName) >= 2 && uniqueName[0] == 'T' && uniqueName[1] >= '1' && uniqueName[1] <= '8' {
		return int(uniqueName[1] - '0')
	}
	return 0
}

func parseEnchant(uniqueName string) int {
	if idx := strings.Index(uniqueName, "@"); idx != -1 && idx+1 < len(uniqueName) {
		return int(uniqueName[idx+1] - '0')
	}
	return 0
}
