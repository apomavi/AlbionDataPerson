package custom

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/ao-data/albiondata-client/log"
)

type TradeSession struct {
	T1Name   string
	T1Guild  string
	T2Name   string
	T2Guild  string
	T1Silver int64
	T2Silver int64
	T1Items  map[int]int
	T2Items  map[int]int
	Complete bool
}

type AODPHistoryResponse []struct {
	Location string `json:"location"`
	Data     []struct {
		ItemCount    float64 `json:"item_count"`
		AveragePrice float64 `json:"avg_price"`
		Timestamp    string  `json:"timestamp"`
	} `json:"data"`
}

var activeTrades = make(map[string]*TradeSession)

func parsePhotonArray(val interface{}) []int {
	var res []int
	if val == nil {
		return res
	}
	v := reflect.ValueOf(val)
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		for i := 0; i < v.Len(); i++ {
			item := v.Index(i).Interface()
			str := fmt.Sprintf("%v", item)
			var num int
			fmt.Sscanf(str, "%d", &num)
			res = append(res, num)
		}
	}
	return res
}

func parseSilver(val interface{}) int64 {
	if val == nil {
		return 0
	}
	v := reflect.ValueOf(val)
	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		if v.Len() > 0 {
			item := v.Index(0).Interface()
			str := fmt.Sprintf("%v", item)
			var num int64
			fmt.Sscanf(str, "%d", &num)
			return num / 10000
		}
		return 0
	}
	str := fmt.Sprintf("%v", val)
	var num int64
	fmt.Sscanf(str, "%d", &num)
	return num / 10000
}

func getItemName(id int) string {
	strID := fmt.Sprintf("%d", id)
	uniqueName := IdToItemName[strID]
	if uniqueName == "" {
		return fmt.Sprintf("Unknown_%d", id)
	}
	realName := ItemRealNames[uniqueName]
	if realName != "" {
		return realName
	}
	return uniqueName
}

func getRealValueFromDB(id int) int64 {
	strID := fmt.Sprintf("%d", id)
	uniqueName := IdToItemName[strID]
	if uniqueName == "" {
		return 0
	}

	var dbVolume float64
	var dbTotalValue float64
	if db != nil {
		_ = db.QueryRow(`
			SELECT COALESCE(SUM(item_amount), 0), COALESCE(SUM(silver_amount), 0)
			FROM market_history
			WHERE item_id = $1
			  AND timestamp >= NOW() - INTERVAL '14 DAYS'
			  AND location NOT IN (3003, 3004, 3005)
		`, uniqueName).Scan(&dbVolume, &dbTotalValue)
	}

	var dbAvgPrice int64
	if dbVolume > 0 {
		dbAvgPrice = int64(dbTotalValue / dbVolume)
	}

	var apiVolume float64
	var apiTotalValue float64

	httpClient := http.Client{Timeout: 3 * time.Second}
	apiURL := "https://europe.albion-online-data.com/api/v2/stats/History/" + uniqueName + "?time-scale=24&qualities=1"
	resp, apiErr := httpClient.Get(apiURL)
	if apiErr == nil {
		defer resp.Body.Close()
		var historyData AODPHistoryResponse
		if json.NewDecoder(resp.Body).Decode(&historyData) == nil {
			now := time.Now()
			fourteenDaysAgo := now.AddDate(0, 0, -14)

			for _, locData := range historyData {
				if locData.Location == "Caerleon" || locData.Location == "Black Market" || locData.Location == "3004" || locData.Location == "3003" || locData.Location == "3005" {
					continue
				}
				for _, d := range locData.Data {
					t, _ := time.Parse("2006-01-02T15:04:05", d.Timestamp)
					if t.After(fourteenDaysAgo) {
						apiVolume += d.ItemCount
						apiTotalValue += d.AveragePrice * d.ItemCount
					}
				}
			}
		}
	}

	var apiAvgPrice int64
	if apiVolume > 0 {
		apiAvgPrice = int64(apiTotalValue / apiVolume)
	}

	if apiVolume > dbVolume && apiAvgPrice > 0 {
		return apiAvgPrice
	}
	if dbAvgPrice > 0 {
		return dbAvgPrice
	}
	if apiAvgPrice > 0 {
		return apiAvgPrice
	}

	if db == nil {
		return 0
	}

	var fallbackPrice int64
	_ = db.QueryRow(`
		SELECT price
		FROM market_orders
		WHERE item_id = $1
		  AND auction_type = 'offer'
		  AND location NOT IN (3003, 3004, 3005)
		ORDER BY price ASC LIMIT 1
	`, uniqueName).Scan(&fallbackPrice)

	return fallbackPrice
}

func extractItemsFromParams(pID interface{}, pAmt interface{}) map[int]int {
	items := make(map[int]int)
	if pID == nil {
		return items
	}
	ids := parsePhotonArray(pID)
	var amts []int
	if pAmt != nil {
		amts = parsePhotonArray(pAmt)
	}
	for i, id := range ids {
		amt := 1
		if i < len(amts) {
			amt = amts[i]
		}
		items[id] += amt
	}
	return items
}

func AnalyzeTrade(eventType int16, params map[uint8]interface{}) {
	tradeID := fmt.Sprintf("%v", params[0])
	session, exists := activeTrades[tradeID]
	if !exists {
		return
	}

	switch eventType {
	case 177:
		if session.Complete {
			return
		}
		if s1, ok := params[2]; ok {
			session.T1Silver = parseSilver(s1)
		}
		if p8, ok := params[8]; ok {
			session.T1Items = extractItemsFromParams(p8, params[15])
		}
		if s2, ok := params[4]; ok {
			session.T2Silver = parseSilver(s2)
		}
		if p18, ok := params[18]; ok {
			session.T2Items = extractItemsFromParams(p18, params[25])
		}

	case 179:
		if session.Complete {
			return
		}
		who := "One side"
		if _, ok := params[1]; ok {
			who = "LOCAL"
		}
		if _, ok := params[2]; ok {
			who = "REMOTE"
		}
		log.Infof("%s clicked accept.", who)

	case 176:
		if !session.Complete {
			session.Complete = true
			log.Warnf("[TRADE CANCELLED] Session: %s", tradeID)
			delete(activeTrades, tradeID)
		}

	case 178:
		if session.Complete {
			return
		}
		session.Complete = true
		PrintTradeReceipt(tradeID, session)
		delete(activeTrades, tradeID)
	}
}

func PrintTradeReceipt(tradeID string, session *TradeSession) {
	currentTimeUTC := time.Now().UTC()
	currentTimeStr := currentTimeUTC.Format("2006-01-02 15:04:05 UTC")
	readableLocation := parseLocation(CurrentMap)

	log.Infof("\n%s", strings.Repeat("=", 60))
	log.Infof("ALBION TRADE REPORT")
	log.Infof("Time     : %s", currentTimeStr)
	log.Infof("Location : %s", readableLocation)
	log.Infof("Session  : %s", tradeID)
	log.Infof("%s", strings.Repeat("=", 60))

	t1Info := fmt.Sprintf("%s [%s]", session.T1Name, session.T1Guild)
	if session.T1Guild == "" || session.T1Guild == "[]" {
		t1Info = session.T1Name
	}

	t2Info := fmt.Sprintf("%s [%s]", session.T2Name, session.T2Guild)
	if session.T2Guild == "" || session.T2Guild == "[]" {
		t2Info = session.T2Name
	}

	log.Infof("Parties: %s  <->  %s", t1Info, t2Info)
	log.Infof("%s", strings.Repeat("-", 60))

	var v1DB, v2DB int64
	localItems := make([]collectorTradeItem, 0, len(session.T1Items))
	remoteItems := make([]collectorTradeItem, 0, len(session.T2Items))

	type dbItem struct {
		Owner string
		ID    int
		Name  string
		Amt   int
		Price int64
	}
	var itemsToSave []dbItem

	log.Infof("%s (Your items):", session.T1Name)
	if len(session.T1Items) == 0 {
		log.Infof("  - No items")
	}
	for id, amt := range session.T1Items {
		p := getRealValueFromDB(id)
		v1DB += p * int64(amt)
		name := getItemName(id)
		log.Infof("  - %dx %s | 14d avg: %d", amt, name, p)
		itemsToSave = append(itemsToSave, dbItem{Owner: session.T1Name, ID: id, Name: name, Amt: amt, Price: p})
		localItems = append(localItems, collectorTradeItem{
			ItemID:     id,
			ItemName:   name,
			Amount:     amt,
			UnitPrice:  p,
			TotalPrice: p * int64(amt),
		})
	}
	log.Infof("  Silver: %d", session.T1Silver)
	log.Infof("  Total : %d", v1DB+session.T1Silver)
	log.Infof("%s", strings.Repeat("-", 60))

	log.Infof("%s (Their items):", session.T2Name)
	if len(session.T2Items) == 0 {
		log.Infof("  - No items")
	}
	for id, amt := range session.T2Items {
		p := getRealValueFromDB(id)
		v2DB += p * int64(amt)
		name := getItemName(id)
		log.Infof("  - %dx %s | 14d avg: %d", amt, name, p)
		itemsToSave = append(itemsToSave, dbItem{Owner: session.T2Name, ID: id, Name: name, Amt: amt, Price: p})
		remoteItems = append(remoteItems, collectorTradeItem{
			ItemID:     id,
			ItemName:   name,
			Amount:     amt,
			UnitPrice:  p,
			TotalPrice: p * int64(amt),
		})
	}
	log.Infof("  Silver: %d", session.T2Silver)
	log.Infof("  Total : %d", v2DB+session.T2Silver)

	log.Infof("%s", strings.Repeat("=", 60))

	myTotalIn := v2DB + session.T2Silver
	myTotalOut := v1DB + session.T1Silver
	profit := myTotalIn - myTotalOut

	switch {
	case profit > 0:
		log.Infof("NET RESULT: PROFIT %d", profit)
	case profit < 0:
		log.Infof("NET RESULT: LOSS %d", -profit)
	default:
		log.Infof("NET RESULT: EVEN")
	}

	emitCollectorTradeCompleted(
		collectorTradeCompletedPayload{
			SessionID:   tradeID,
			Location:    readableLocation,
			CompletedAt: currentTimeUTC,
			LocalParty: collectorTradeParty{
				Name:      session.T1Name,
				GuildName: session.T1Guild,
				Silver:    session.T1Silver,
				Total:     v1DB + session.T1Silver,
				Items:     localItems,
			},
			RemoteParty: collectorTradeParty{
				Name:      session.T2Name,
				GuildName: session.T2Guild,
				Silver:    session.T2Silver,
				Total:     v2DB + session.T2Silver,
				Items:     remoteItems,
			},
			NetProfit: profit,
		},
		collectorActor{
			CharacterName: session.T1Name,
		},
		collectorContext{
			LocationID: CurrentMap,
			CurrentMap: readableLocation,
			GuildName:  session.T1Guild,
		},
	)

	if db == nil {
		log.Infof("%s\n", strings.Repeat("=", 60))
		return
	}

	var dbTradeID int
	err := db.QueryRow(`
		INSERT INTO trades (
			session_id, trade_date, location,
			player1_name, player1_guild, player2_name, player2_guild,
			player1_silver, player2_silver, player1_total_value, player2_total_value, net_profit
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		) RETURNING id
	`, tradeID, currentTimeUTC, readableLocation,
		session.T1Name, session.T1Guild, session.T2Name, session.T2Guild,
		session.T1Silver, session.T2Silver, v1DB, v2DB, profit).Scan(&dbTradeID)

	if err != nil {
		log.Errorf("Trade DB save error: %v", err)
		log.Infof("%s\n", strings.Repeat("=", 60))
		return
	}

	for _, item := range itemsToSave {
		totalPrice := item.Price * int64(item.Amt)
		_, itemErr := db.Exec(`
			INSERT INTO trade_items (trade_id, owner_name, item_id, item_name, amount, unit_price, total_price)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, dbTradeID, item.Owner, item.ID, item.Name, item.Amt, item.Price, totalPrice)
		if itemErr != nil {
			log.Errorf("Trade item DB save error (%s): %v", item.Name, itemErr)
		}
	}

	log.Infof("[DB] Trade saved successfully. Record ID: %d", dbTradeID)
	log.Infof("%s\n", strings.Repeat("=", 60))
}
