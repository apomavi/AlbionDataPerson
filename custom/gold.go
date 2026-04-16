package custom

import (
	"fmt"
	"sort"
	"time"

	"github.com/ao-data/albiondata-client/log"
)

type GoldDataPoint struct {
	Timestamp time.Time
	Price     int
}

func onGoldPrices(prices []int, timestamps []int64) {
	if db == nil {
		return
	}

	var validDataPoints []GoldDataPoint
	for i, price := range prices {
		if price <= 0 || i >= len(timestamps) {
			continue
		}

		timestampInt := timestamps[i]
		var timeObj time.Time
		if timestampInt > 10000000000000000 {
			timeObj = time.Unix(0, (timestampInt-621355968000000000)*100).UTC()
		} else {
			timeObj = time.Unix(0, timestampInt*1000000).UTC()
		}

		validDataPoints = append(validDataPoints, GoldDataPoint{
			Timestamp: timeObj,
			Price:     price,
		})
	}

	sort.Slice(validDataPoints, func(i, j int) bool {
		return validDataPoints[i].Timestamp.Before(validDataPoints[j].Timestamp)
	})

	savedCount := 0
	for _, dataPoint := range validDataPoints {
		query := `
			INSERT INTO public.gold_prices ("timestamp", price)
			VALUES ($1, $2)
			ON CONFLICT ("timestamp") DO UPDATE SET price = EXCLUDED.price
		`

		_, err := db.Exec(query, dataPoint.Timestamp, dataPoint.Price)
		if err != nil {
			log.Errorf("Gold price could not be saved: %v", err)
			continue
		}
		savedCount++
	}

	if savedCount > 0 {
		fmt.Printf("Gold market data saved: %d points.\n", savedCount)
	}
}
