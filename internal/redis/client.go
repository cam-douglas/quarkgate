package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	MeterStreamKey = "meter:stream"
	MeterDLQKey    = "meter:dlq"
)

type Client struct {
	rdb *goredis.Client
}

func Connect(url string) (*Client, error) {
	opt, err := goredis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	rdb := goredis.NewClient(opt)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &Client{rdb: rdb}, nil
}

func (c *Client) RDB() *goredis.Client {
	return c.rdb
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

func (c *Client) SetBalance(ctx context.Context, userID string, micro int64) error {
	return c.rdb.Set(ctx, balanceKey(userID), micro, 0).Err()
}

func (c *Client) GetBalance(ctx context.Context, userID string) (int64, bool, error) {
	v, err := c.rdb.Get(ctx, balanceKey(userID)).Int64()
	if err == goredis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}

func (c *Client) SetKeyMeta(ctx context.Context, keyHash string, meta string) error {
	return c.rdb.Set(ctx, keyMetaKey(keyHash), meta, 24*time.Hour).Err()
}

func (c *Client) DeleteKeyMeta(ctx context.Context, keyHash string) error {
	return c.rdb.Del(ctx, keyMetaKey(keyHash)).Err()
}

func (c *Client) SetKeyIDIndex(ctx context.Context, keyID, hmacHash string) error {
	return c.rdb.Set(ctx, keyIDIndexKey(keyID), hmacHash, 24*time.Hour).Err()
}

func (c *Client) GetKeyIDIndex(ctx context.Context, keyID string) (string, bool, error) {
	v, err := c.rdb.Get(ctx, keyIDIndexKey(keyID)).Result()
	if err == goredis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (c *Client) DeleteKeyIDIndex(ctx context.Context, keyID string) error {
	return c.rdb.Del(ctx, keyIDIndexKey(keyID)).Err()
}

func (c *Client) GetKeyMeta(ctx context.Context, keyHash string) (string, bool, error) {
	v, err := c.rdb.Get(ctx, keyMetaKey(keyHash)).Result()
	if err == goredis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (c *Client) SetHold(ctx context.Context, requestID string, micro int64, ttl time.Duration) error {
	return c.rdb.Set(ctx, holdKey(requestID), micro, ttl).Err()
}

func (c *Client) GetHold(ctx context.Context, requestID string) (int64, bool, error) {
	v, err := c.rdb.Get(ctx, holdKey(requestID)).Int64()
	if err == goredis.Nil {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return v, true, nil
}

func (c *Client) DeleteHold(ctx context.Context, requestID string) error {
	return c.rdb.Del(ctx, holdKey(requestID)).Err()
}

func (c *Client) AllowRate(ctx context.Context, userID string, rpm int) (bool, error) {
	key := bucketKey(userID)
	now := time.Now().Unix()
	pipe := c.rdb.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", now-60))
	pipe.ZAdd(ctx, key, goredis.Z{Score: float64(now), Member: now})
	pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, 2*time.Minute)
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}
	count := cmds[2].(*goredis.IntCmd).Val()
	return count <= int64(rpm), nil
}

func (c *Client) EmitMeterEvent(ctx context.Context, fields map[string]interface{}) error {
	return c.rdb.XAdd(ctx, &goredis.XAddArgs{
		Stream: MeterStreamKey,
		Values: fields,
	}).Err()
}

func (c *Client) CreateConsumerGroup(ctx context.Context, group string) error {
	err := c.rdb.XGroupCreateMkStream(ctx, MeterStreamKey, group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

func (c *Client) ReadMeterEvents(ctx context.Context, group, consumer string, count int64, block time.Duration) ([]goredis.XMessage, error) {
	streams, err := c.rdb.XReadGroup(ctx, &goredis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{MeterStreamKey, ">"},
		Count:    count,
		Block:    block,
	}).Result()
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, nil
	}
	return streams[0].Messages, nil
}

func (c *Client) AckMeterEvent(ctx context.Context, group, id string) error {
	return c.rdb.XAck(ctx, MeterStreamKey, group, id).Err()
}

func (c *Client) AutoClaimMeterEvents(ctx context.Context, group, consumer string, minIdle time.Duration, count int64) ([]goredis.XMessage, error) {
	messages, _, err := c.rdb.XAutoClaim(ctx, &goredis.XAutoClaimArgs{
		Stream:   MeterStreamKey,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Count:    count,
		Start:    "0-0",
	}).Result()
	if err != nil {
		return nil, err
	}
	return messages, nil
}

func (c *Client) ReadDLQ(ctx context.Context, count int64) ([]goredis.XMessage, error) {
	streams, err := c.rdb.XRead(ctx, &goredis.XReadArgs{
		Streams: []string{MeterDLQKey, "0"},
		Count:   count,
	}).Result()
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, nil
	}
	return streams[0].Messages, nil
}

func (c *Client) ReplayDLQEntry(ctx context.Context, fields map[string]interface{}) error {
	return c.EmitMeterEvent(ctx, fields)
}

func (c *Client) MessageDeliveries(ctx context.Context, group, id string) int64 {
	ext, err := c.rdb.XPendingExt(ctx, &goredis.XPendingExtArgs{
		Stream: MeterStreamKey,
		Group:  group,
		Start:  id,
		End:    id,
		Count:  1,
	}).Result()
	if err != nil || len(ext) == 0 {
		return 1
	}
	return ext[0].RetryCount
}

func (c *Client) PushDLQ(ctx context.Context, fields map[string]interface{}) error {
	return c.rdb.XAdd(ctx, &goredis.XAddArgs{
		Stream: MeterDLQKey,
		Values: fields,
	}).Err()
}

func (c *Client) SetIdempotency(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.rdb.Set(ctx, idemKey(key), value, ttl).Err()
}

func (c *Client) GetIdempotency(ctx context.Context, key string) (string, bool, error) {
	v, err := c.rdb.Get(ctx, idemKey(key)).Result()
	if err == goredis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func balanceKey(userID string) string  { return "balance:" + userID }
func holdKey(requestID string) string    { return "hold:" + requestID }
func keyMetaKey(hash string) string      { return "key:" + hash }
func keyIDIndexKey(keyID string) string   { return "keyid:" + keyID }
func bucketKey(userID string) string     { return "bucket:" + userID + ":rpm" }
func idemKey(key string) string          { return "idem:" + key }
