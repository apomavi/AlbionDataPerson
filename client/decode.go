package client

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ao-data/albiondata-client/lib"
	"github.com/ao-data/albiondata-client/log"
	"github.com/mitchellh/mapstructure"
)

// --- GLOBAL İSTİHBARAT DEĞİŞKENLERİ ---
var (
	MyCharacterName string = "SEN (Local)"
	MyGuildName     string = "Loncasız"
	CurrentMap      string = "Bilinmeyen Konum"
)

// Konum ID'sini okunaklı isme çeviren akıllı fonksiyon
func parseLocation(rawLoc string) string {
	// 1. Bilinen şehirlerden biri mi?
	if name, exists := LocationNames[rawLoc]; exists {
		return fmt.Sprintf("%s (ID: %s)", name, rawLoc)
	}
	// 2. Oyuncu adası veya lonca adası mı?
	if strings.Contains(rawLoc, "@ISLAND@") {
		return fmt.Sprintf("Oyuncu Adası (ID: %s)", rawLoc)
	}
	// 3. Banka veya benzeri bir ev objesi mi?
	if strings.Contains(rawLoc, "BLACKBANK") || strings.Contains(rawLoc, "HomeObject") {
		return fmt.Sprintf("Banka / Sandık Alanı (ID: %s)", rawLoc)
	}
	// 4. Hiçbiri değilse (yeni bir haritaysa)
	return fmt.Sprintf("Bilinmeyen Bölge (ID: %s)", rawLoc)
}

func decodeRequest(params map[uint8]interface{}) (operation operation, err error) {
	if _, ok := params[253]; !ok {
		return nil, nil
	}
	code := params[253].(int16)

	switch OperationType(code) {
	case opGetGameServerByCluster:
		operation = &operationGetGameServerByCluster{}
	case opAuctionGetOffers:
		operation = &operationAuctionGetOffers{}
	case opAuctionGetItemAverageStats:
		operation = &operationAuctionGetItemAverageStats{}
	case opGetClusterMapInfo:
		operation = &operationGetClusterMapInfo{}
	case opGoldMarketGetAverageInfo:
		operation = &operationGoldMarketGetAverageInfo{}
	case opRealEstateGetAuctionData:
		operation = &operationRealEstateGetAuctionData{}
	case opRealEstateBidOnAuction:
		operation = &operationRealEstateBidOnAuction{}
	default:
		return nil, nil
	}

	err = decodeParams(params, operation)
	return operation, err
}

func decodeResponse(params map[uint8]interface{}) (operation operation, err error) {
	if _, ok := params[253]; !ok {
		return nil, nil
	}
	code := params[253].(int16)

	// --- 1. TİCARET TARAFLARI (155 OpCode) ---
	if code == 155 {
		tradeId := fmt.Sprintf("%v", params[6])

		p1Name := MyCharacterName
		p2Name := fmt.Sprintf("%v", params[1])
		p2Guild := fmt.Sprintf("%v", params[2])

		if activeTrades[tradeId] == nil {
			activeTrades[tradeId] = &TradeSession{
				T1Name:  p1Name,
				T1Guild: MyGuildName,
				T2Name:  p2Name,
				T2Guild: p2Guild,
				T1Items: make(map[int]int),
				T2Items: make(map[int]int),
			}
		}
		log.Infof("🔍 [TİCARET BAŞLADI] %s ↔️ %s (Oturum: %s)", p1Name, p2Name, tradeId)
	}

	switch OperationType(code) {
	case opJoin:
		operation = &operationJoinResponse{}
		// --- 2. KENDİ BİLGİLERİMİZİ YAKALAMA (Join Paketi) ---
		if name, ok := params[2]; ok {
			MyCharacterName = fmt.Sprintf("%v", name)
		}
		if guild, ok := params[57]; ok && fmt.Sprintf("%v", guild) != "[]" {
			MyGuildName = fmt.Sprintf("%v", guild)
		}
		if loc, ok := params[8]; ok && fmt.Sprintf("%v", loc) != "[]" {
			CurrentMap = fmt.Sprintf("%v", loc)
		}

		log.Infof("🟢 [SİSTEM BAĞLANDI] Oyuncu: %s | Lonca: %s | Konum: %s", MyCharacterName, MyGuildName, CurrentMap)

	case opAuctionGetOffers:
		operation = &operationAuctionGetOffersResponse{}
	case opAuctionGetRequests:
		operation = &operationAuctionGetRequestsResponse{}
	case opAuctionBuyOffer:
		operation = &operationAuctionGetRequestsResponse{}
	case opAuctionGetItemAverageStats:
		operation = &operationAuctionGetItemAverageStatsResponse{}
	case opGetMailInfos:
		operation = &operationGetMailInfosResponse{}
	case opReadMail:
		operation = &operationReadMail{}
	case opGetClusterMapInfo:
		operation = &operationGetClusterMapInfoResponse{}
	case opGoldMarketGetAverageInfo:
		operation = &operationGoldMarketGetAverageInfoResponse{}
	case opRealEstateGetAuctionData:
		operation = &operationRealEstateGetAuctionDataResponse{}
	case opRealEstateBidOnAuction:
		operation = &operationRealEstateBidOnAuctionResponse{}
	default:
		return nil, nil
	}

	err = decodeParams(params, operation)
	return operation, err
}

func decodeEvent(params map[uint8]interface{}) (event operation, err error) {
	if _, ok := params[252]; !ok {
		return nil, nil
	}
	eventType := params[252].(int16)

	switch EventType(eventType) {
	case evRedZoneWorldMapEvent:
		event = &eventRedZoneWorldMapEvent{}
	default:
		if eventType >= 176 && eventType <= 179 {
			AnalyzeTrade(eventType, params)
		}
		return nil, nil
	}

	err = decodeParams(params, event)
	return event, err
}

func decodeParams(params map[uint8]interface{}, operation operation) error {
	convertGameObjects := func(from reflect.Type, to reflect.Type, v interface{}) (interface{}, error) {
		if from == reflect.TypeOf([]int8{}) && to == reflect.TypeOf(lib.CharacterID("")) {
			return decodeCharacterID(v.([]int8)), nil
		}
		return v, nil
	}
	config := mapstructure.DecoderConfig{DecodeHook: convertGameObjects, Result: operation}
	decoder, err := mapstructure.NewDecoder(&config)
	if err != nil {
		return err
	}

	stringMap := make(map[string]interface{})
	for k, v := range params {
		stringMap[strconv.Itoa(int(k))] = v
	}
	return decoder.Decode(stringMap)
}

func decodeCharacterID(array []int8) lib.CharacterID {
	b := make([]byte, len(array))
	for k, v := range array {
		b[k] = byte(v)
	}
	b[0], b[1], b[2], b[3] = b[3], b[2], b[1], b[0]
	b[4], b[5] = b[5], b[4]
	b[6], b[7] = b[7], b[6]
	var buf [36]byte
	hex.Encode(buf[:], b[:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:], b[10:])
	return lib.CharacterID(buf[:])
}

// -------------------------------------------------------------------------
// --- ALBION TİCARET (TRADE) ANALİZ MOTORU ---
// -------------------------------------------------------------------------

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
		return fmt.Sprintf("Bilinmeyen_%d", id)
	}
	realName := ItemRealNames[uniqueName]
	if realName != "" {
		return realName
	}
	return uniqueName
}

// AODP History API JSON Yapısı
type AODPHistoryResponse []struct {
	Location string `json:"location"`
	Data     []struct {
		ItemCount    float64 `json:"item_count"`
		AveragePrice float64 `json:"avg_price"`
		Timestamp    string  `json:"timestamp"`
	} `json:"data"`
}

func getRealValueFromDB(id int) int64 {
	strID := fmt.Sprintf("%d", id)
	uniqueName := IdToItemName[strID]
	if uniqueName == "" {
		return 0
	}

	var dbVolume float64 = 0
	var dbTotalValue float64 = 0

	_ = db.QueryRow(`
		SELECT COALESCE(SUM(item_amount), 0), COALESCE(SUM(avg_price * item_amount), 0)
		FROM market_history 
		WHERE item_id = $1 
		  AND timestamp >= NOW() - INTERVAL '14 DAYS'
		  AND location NOT IN ('3003', '3004', 'Black Market', 'Caerleon')
	`, uniqueName).Scan(&dbVolume, &dbTotalValue)

	var dbAvgPrice int64 = 0
	if dbVolume > 0 {
		dbAvgPrice = int64(dbTotalValue / dbVolume)
	}

	var apiVolume float64 = 0
	var apiTotalValue float64 = 0

	client := http.Client{Timeout: 3 * time.Second}
	apiUrl := "https://europe.albion-online-data.com/api/v2/stats/History/" + uniqueName + "?time-scale=24&qualities=1"
	resp, apiErr := client.Get(apiUrl)

	if apiErr == nil {
		defer resp.Body.Close()
		var historyData AODPHistoryResponse
		if json.NewDecoder(resp.Body).Decode(&historyData) == nil {
			now := time.Now()
			fourteenDaysAgo := now.AddDate(0, 0, -14)

			for _, locData := range historyData {
				if locData.Location == "Caerleon" || locData.Location == "Black Market" || locData.Location == "3004" || locData.Location == "3003" {
					continue
				}
				for _, d := range locData.Data {
					t, _ := time.Parse("2006-01-02T15:04:05", d.Timestamp)
					if t.After(fourteenDaysAgo) {
						apiVolume += d.ItemCount
						apiTotalValue += (d.AveragePrice * d.ItemCount)
					}
				}
			}
		}
	}

	var apiAvgPrice int64 = 0
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

	var fallbackPrice int64 = 0
	_ = db.QueryRow(`
		SELECT price 
		FROM market_data 
		WHERE item_id = $1 
		  AND auction_type = 'offer' 
		  AND location NOT IN ('3003', '3004', 'Black Market', 'Caerleon')
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
	tradeId := fmt.Sprintf("%v", params[0])
	session, exists := activeTrades[tradeId]
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
		who := "Bir taraf"
		if _, ok := params[1]; ok {
			who = "SEN (Local)"
		}
		if _, ok := params[2]; ok {
			who = "KARŞI TARAF"
		}
		log.Infof("   ✅ %s 'KABUL ET' Butonuna Bastı.", who)

	case 176:
		if !session.Complete {
			session.Complete = true
			log.Warnf("❌ [TİCARET İPTAL EDİLDİ] Oturum: %s\n", tradeId)
			delete(activeTrades, tradeId)
		}

	case 178:
		if session.Complete {
			return
		}
		session.Complete = true
		PrintTradeReceipt(tradeId, session)
		delete(activeTrades, tradeId)
	}
}

// 📊 FİŞ KESME, ANALİZ VE VERİTABANI KAYIT RAPORU
func PrintTradeReceipt(tradeId string, session *TradeSession) {
	// Saati tam UTC formunda alıyoruz (Hem log hem DB için)
	currentTimeUTC := time.Now().UTC()
	currentTimeStr := currentTimeUTC.Format("2006-01-02 15:04:05 UTC")

	readableLocation := parseLocation(CurrentMap)

	log.Infof("\n%s", strings.Repeat("━", 60))
	log.Infof("📑 ALBİON TİCARET İSTİHBARAT RAPORU")
	log.Infof("🕒 Tarih/Saat : %s", currentTimeStr)
	log.Infof("📍 Konum      : %s", readableLocation)
	log.Infof("🔑 Oturum     : %s", tradeId)
	log.Infof("%s", strings.Repeat("━", 60))

	t1Info := fmt.Sprintf("%s [%s]", session.T1Name, session.T1Guild)
	if session.T1Guild == "" || session.T1Guild == "[]" {
		t1Info = session.T1Name
	}

	t2Info := fmt.Sprintf("%s [%s]", session.T2Name, session.T2Guild)
	if session.T2Guild == "" || session.T2Guild == "[]" {
		t2Info = session.T2Name
	}

	log.Infof("👥 TARAFLAR: %s  ↔️  %s", t1Info, t2Info)
	log.Infof("%s", strings.Repeat("-", 60))

	var v1DB, v2DB int64

	// Eşyaları API'yi veya DB'yi tekrar yormamak için geçici olarak belleğe alıyoruz
	type dbItem struct {
		Owner string
		ID    int
		Name  string
		Amt   int
		Price int64
	}
	var itemsToSave []dbItem

	// --- 1. TARAF (SEN) ---
	log.Infof("📦 %s (Senin Verdiklerin):", session.T1Name)
	if len(session.T1Items) == 0 {
		log.Infof("   - Eşya yok")
	}
	for id, amt := range session.T1Items {
		p := getRealValueFromDB(id)
		v1DB += p * int64(amt)
		name := getItemName(id)
		log.Infof("   - %dx %s | 14 Günlük Ort. Fiyat: %d 🥈", amt, name, p)

		// Veritabanı için listeye ekle
		itemsToSave = append(itemsToSave, dbItem{Owner: session.T1Name, ID: id, Name: name, Amt: amt, Price: p})
	}
	log.Infof("   💰 Gümüş: %d 🥈", session.T1Silver)
	log.Infof("   📈 Toplam Ortalama Değer: %d 🥈", v1DB+session.T1Silver)
	log.Infof("%s", strings.Repeat("-", 60))

	// --- 2. TARAF (KARŞI TARAF) ---
	log.Infof("📦 %s (Onun Verdikleri):", session.T2Name)
	if len(session.T2Items) == 0 {
		log.Infof("   - Eşya yok")
	}
	for id, amt := range session.T2Items {
		p := getRealValueFromDB(id)
		v2DB += p * int64(amt)
		name := getItemName(id)
		log.Infof("   - %dx %s | 14 Günlük Ort. Fiyat: %d 🥈", amt, name, p)

		// Veritabanı için listeye ekle
		itemsToSave = append(itemsToSave, dbItem{Owner: session.T2Name, ID: id, Name: name, Amt: amt, Price: p})
	}
	log.Infof("   💰 Gümüş: %d 🥈", session.T2Silver)
	log.Infof("   📈 Toplam Ortalama Değer: %d 🥈", v2DB+session.T2Silver)

	// --- ANALİZ VE SONUÇ ---
	log.Infof("%s", strings.Repeat("━", 60))

	myTotalIn := v2DB + session.T2Silver
	myTotalOut := v1DB + session.T1Silver
	profit := myTotalIn - myTotalOut

	if profit > 0 {
		log.Infof("🚀 NET SONUÇ: %d 🥈 KÂR ETTİN!", profit)
	} else if profit < 0 {
		log.Infof("🚨 NET SONUÇ: %d 🥈 ZARAR ETTİN!", -profit)
	} else {
		log.Infof("⚖️  NET SONUÇ: Ticaret tamamen EŞİT (0) değerde bitti.")
	}

	// =========================================================================
	// 💾 VERİTABANI KAYIT İŞLEMİ (POSTGRESQL)
	// =========================================================================

	var dbTradeID int
	err := db.QueryRow(`
		INSERT INTO trades (
			session_id, trade_date, location,
			player1_name, player1_guild, player2_name, player2_guild,
			player1_silver, player2_silver, player1_total_value, player2_total_value, net_profit
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		) RETURNING id
	`, tradeId, currentTimeUTC, readableLocation,
		session.T1Name, session.T1Guild, session.T2Name, session.T2Guild,
		session.T1Silver, session.T2Silver, v1DB, v2DB, profit).Scan(&dbTradeID)

	if err != nil {
		log.Errorf("❌ Veritabanı Kayıt Hatası (Ana Tablo): %v", err)
	} else {
		// Ana kayıt başarılıysa, eşyaları alt tabloya yaz
		for _, item := range itemsToSave {
			totalPrice := item.Price * int64(item.Amt)
			_, itemErr := db.Exec(`
				INSERT INTO trade_items (trade_id, owner_name, item_id, item_name, amount, unit_price, total_price)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, dbTradeID, item.Owner, item.ID, item.Name, item.Amt, item.Price, totalPrice)

			if itemErr != nil {
				log.Errorf("❌ Veritabanı Kayıt Hatası (Eşya %s): %v", item.Name, itemErr)
			}
		}
		log.Infof("💾 [DB] Ticaret başarıyla veritabanına kaydedildi! (Kayıt ID: %d)", dbTradeID)
	}
	// =========================================================================

	log.Infof("%s\n", strings.Repeat("━", 60))
}
