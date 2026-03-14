package internal

import "math"

func CalculateDynamicPrice(basePrice float64, viewsPerMinute int, event string) (float64, string) {
	demandFactor := 1.0
	reason := "baseline"

	switch {
	case viewsPerMinute >= 1000:
		demandFactor = 1.20
		reason = "surge_high_views"
	case viewsPerMinute >= 300:
		demandFactor = 1.10
		reason = "high_views"
	case viewsPerMinute < 20:
		demandFactor = 0.95
		reason = "low_demand"
	}

	if event == "purchase" {
		demandFactor += 0.05
		reason = reason + "_purchase_signal"
	}

	price := basePrice * demandFactor
	return math.Round(price*100) / 100, reason
}
