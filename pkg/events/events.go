package events

import "time"

type UserEvent struct {
	UserID    string    `json:"user_id" bson:"user_id"`
	Event     string    `json:"event" bson:"event"`
	ProductID string    `json:"product_id" bson:"product_id"`
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`
}

type PriceUpdateEvent struct {
	ProductID string    `json:"product_id" bson:"product_id"`
	OldPrice  float64   `json:"old_price" bson:"old_price"`
	NewPrice  float64   `json:"new_price" bson:"new_price"`
	Reason    string    `json:"reason" bson:"reason"`
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`
}

type AnalyticsEvent struct {
	Type      string         `json:"type" bson:"type"`
	Payload   map[string]any `json:"payload" bson:"payload"`
	Timestamp time.Time      `json:"timestamp" bson:"timestamp"`
}
