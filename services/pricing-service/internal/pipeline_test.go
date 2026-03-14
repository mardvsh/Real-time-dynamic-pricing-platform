package internal

import "testing"

func TestPricingPipelineScenario(t *testing.T) {
	t.Parallel()

	base := 1300.0
	price1, reason1 := CalculateDynamicPrice(base, 50, "view_product")
	if price1 != 1300.0 || reason1 != "baseline" {
		t.Fatalf("unexpected baseline result: price=%.2f reason=%s", price1, reason1)
	}

	price2, reason2 := CalculateDynamicPrice(base, 1200, "view_product")
	if price2 != 1560.0 || reason2 != "surge_high_views" {
		t.Fatalf("unexpected surge result: price=%.2f reason=%s", price2, reason2)
	}

	price3, reason3 := CalculateDynamicPrice(base, 1200, "purchase")
	if price3 != 1625.0 || reason3 != "surge_high_views_purchase_signal" {
		t.Fatalf("unexpected purchase surge result: price=%.2f reason=%s", price3, reason3)
	}
}
