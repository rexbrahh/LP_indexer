package cache

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "strconv"
    "time"

    "github.com/redis/go-redis/v9"

    "github.com/rexbrahh/lp-indexer/api/http/types"
)

// ErrDisabled indicates the cache layer is disabled via configuration.
var ErrDisabled = errors.New("redis cache disabled")

// Config represents Redis client configuration options.
type Config struct {
    Enabled  bool
    Addr     string
    Password string
    DB       int
    TTL      time.Duration
}

// LoadConfigFromEnv constructs a Config from environment variables.
//
// Recognized variables:
//   * API_REDIS_ADDR (required to enable the cache)
//   * API_REDIS_PASSWORD (optional)
//   * API_REDIS_DB (defaults to 0)
//   * API_REDIS_TTL (parseable duration, defaults to 5m)
func LoadConfigFromEnv() (Config, error) {
    addr := os.Getenv("API_REDIS_ADDR")
    if addr == "" {
        return Config{Enabled: false, TTL: 5 * time.Minute}, nil
    }

    password := os.Getenv("API_REDIS_PASSWORD")

    db := 0
    if rawDB := os.Getenv("API_REDIS_DB"); rawDB != "" {
        parsed, err := strconv.Atoi(rawDB)
        if err != nil {
            return Config{}, fmt.Errorf("invalid API_REDIS_DB: %w", err)
        }
        db = parsed
    }

    ttl := 5 * time.Minute
    if rawTTL := os.Getenv("API_REDIS_TTL"); rawTTL != "" {
        parsed, err := time.ParseDuration(rawTTL)
        if err != nil {
            return Config{}, fmt.Errorf("invalid API_REDIS_TTL: %w", err)
        }
        ttl = parsed
    }

    return Config{
        Enabled:  true,
        Addr:     addr,
        Password: password,
        DB:       db,
        TTL:      ttl,
    }, nil
}

// Cache wraps a Redis client to simplify candle caching.
type Cache struct {
    client *redis.Client
    cfg    Config
}

// New creates a new Cache from the provided configuration.
func New(cfg Config) (*Cache, error) {
    if !cfg.Enabled {
        return &Cache{cfg: cfg}, nil
    }

    opts := &redis.Options{
        Addr:     cfg.Addr,
        Password: cfg.Password,
        DB:       cfg.DB,
    }

    client := redis.NewClient(opts)

    return &Cache{
        client: client,
        cfg:    cfg,
    }, nil
}

func (c *Cache) key(poolID, timeframe string) string {
    return fmt.Sprintf("pool:%s:candles:%s", poolID, timeframe)
}

// GetCandles retrieves cached candles for a pool and timeframe.
func (c *Cache) GetCandles(ctx context.Context, poolID, timeframe string) ([]types.Candle, error) {
    if c == nil || c.client == nil {
        return nil, ErrDisabled
    }

    payload, err := c.client.Get(ctx, c.key(poolID, timeframe)).Result()
    if errors.Is(err, redis.Nil) {
        return nil, types.ErrNotFound
    }
    if err != nil {
        return nil, err
    }

    var candles []types.Candle
    if err := json.Unmarshal([]byte(payload), &candles); err != nil {
        return nil, err
    }

    return candles, nil
}

// SetCandles stores candles for a pool and timeframe.
func (c *Cache) SetCandles(ctx context.Context, poolID, timeframe string, candles []types.Candle) error {
    if c == nil || c.client == nil {
        return ErrDisabled
    }

    payload, err := json.Marshal(candles)
    if err != nil {
        return err
    }

    return c.client.Set(ctx, c.key(poolID, timeframe), payload, c.cfg.TTL).Err()
}
