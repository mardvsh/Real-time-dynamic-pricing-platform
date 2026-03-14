package internal

import "testing"

func TestCalculateDynamicPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		basePrice float64
		views     int
		event     string
		wantPrice float64
	}{
		{name: "high views", basePrice: 500, views: 1200, event: "view_product", wantPrice: 600},
		{name: "medium views", basePrice: 500, views: 500, event: "view_product", wantPrice: 550},
		{name: "low demand", basePrice: 500, views: 10, event: "view_product", wantPrice: 475},
		{name: "purchase signal", basePrice: 500, views: 500, event: "purchase", wantPrice: 575},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, _ := CalculateDynamicPrice(tt.basePrice, tt.views, tt.event)
			if got != tt.wantPrice {
				t.Fatalf("price mismatch: got %.2f want %.2f", got, tt.wantPrice)
			}
		})
	}
}
