package redis

import (
	"context"
	"fmt"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
)

func NewClient(addr, password string, db int) *redisv9.Client {
	return redisv9.NewClient(&redisv9.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func SetPrice(ctx context.Context, rdb *redisv9.Client, productID string, price float64, ttl time.Duration) error {
	key := fmt.Sprintf("price:product:%s", productID)
	return rdb.Set(ctx, key, price, ttl).Err()
}

func GetPrice(ctx context.Context, rdb *redisv9.Client, productID string) (float64, error) {
	key := fmt.Sprintf("price:product:%s", productID)
	return rdb.Get(ctx, key).Float64()
}
