package database

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type DB struct {
	Postgres  *pgxpool.Pool
	Timescale *pgxpool.Pool
	Redis     *redis.Client
}

func Connect(ctx context.Context, postgresURL, timescaleURL, redisAddr, redisPassword string) (*DB, error) {
	// PostgreSQL
	authPool, err := pgxpool.New(ctx, postgresURL)
	if err != nil {
		return nil, err
	}

	// TimescaleDB
	telemetryPool, err := pgxpool.New(ctx, timescaleURL)
	if err != nil {
		authPool.Close()
		return nil, err
	}

	// Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       0,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("Redis unavailable: %v (continuing with degraded functionality)", err)
		rdb = nil
	}

	return &DB{
		Postgres:  authPool,
		Timescale: telemetryPool,
		Redis:     rdb,
	}, nil
}

func (db *DB) Close() {
	if db.Postgres != nil {
		db.Postgres.Close()
	}
	if db.Timescale != nil {
		db.Timescale.Close()
	}
	if db.Redis != nil {
		db.Redis.Close()
	}
}
