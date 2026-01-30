package db

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"slices"
	"strings"
	"time"
)

var rdb *redis.Client

const (
	ReceiveStreamName = "chat_receive_stream"
	GroupName         = "chat_group"
	ConsumerName      = "chat_consumer1"
	ZSetName          = "chat_zset"
)

type RankItem struct {
	Member string
	Score  float64
}

// InitRDB 初始化redis
func InitRDB() (err error) {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", //没有密码
		DB:       0,  //默认数据库0
	})

	////docker部署时的redis连接
	//rdb = redis.NewClient(&redis.Options{
	//	Addr:     "redis:6379",
	//	Password: "", //没有密码
	//	DB:       0,  //默认数据库0
	//})

	//测试一下连接
	ctx := context.Background()
	err = rdb.Ping(ctx).Err()
	if err != nil {
		err = fmt.Errorf("rdb.Ping failed,err:%w", err)
		return
	}
	fmt.Println("连接redis成功!")

	err = XGroupCreateMkStreamMsg(ReceiveStreamName, GroupName)
	if err != nil {
		err = fmt.Errorf("XGroupCreateMkStreamMsg failed,err:%w", err)
		return
	}
	return
}

// SetUser 设置键值对
func SetUser(key string, value string) error {
	ctx := context.Background()
	err := rdb.Set(ctx, key, value, 3600*time.Second).Err()
	if err != nil {
		return fmt.Errorf("rdb.Set failed,err:%w", err)
	}
	return nil
}

// GetUser 得到键值
func GetUser(key string) (string, error) {
	ctx := context.Background()
	value, err := rdb.Get(ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("rdb.Get failed,err:%w", err)
	}
	return value, nil
}

// ZAddNXMsg 为有序集合添加成员，分数默认为1
func ZAddNXMsg(member string, key string) (int, error) {
	ctx := context.Background()
	n, err := rdb.ZAddNX(ctx, key, redis.Z{Score: 1.0, Member: member}).Result()
	if err != nil {
		return int(n), fmt.Errorf("rdb.ZAddNX failed:err%w", err)
	}
	return int(n), nil
}

// ZIncrMsg 为某个成员分数加1
func ZIncrMsg(member string, key string) error {
	ctx := context.Background()
	err := rdb.ZIncrBy(ctx, key, 1.0, member).Err()
	if err != nil {
		return fmt.Errorf("rdb.ZIncrBy failed,err:%w", err)
	}
	return nil
}

// ZRevRangeMsg 遍历有序集合
func ZRevRangeMsg(key string) ([]RankItem, error) {
	ctx := context.Background()
	rank, err := rdb.ZRevRangeWithScores(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("rdb.ZRevRangeWithScores failed,err:%w", err)
	}
	res := make([]RankItem, 0, len(rank))
	for _, v := range rank {
		r := RankItem{
			Member: v.Member.(string),
			Score:  v.Score,
		}
		res = append(res, r)
	}
	return res, nil
}

// XGroupCreateMkStreamMsg 创建消费者组和流
func XGroupCreateMkStreamMsg(stream string, group string) (err error) {
	ctx := context.Background()
	err = rdb.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if err != nil {
		//消费者组已存在不算错误
		if strings.Contains(err.Error(), "BUSYGROUP") {
			err = nil
			return
		}
		err = fmt.Errorf("rdb.XGroupCreateMkStream failed,err:%w", err)
		return
	}
	return
}

// XAddMsg 消息加入流中
func XAddMsg(msg string, stream string) error {
	ctx := context.Background()
	err := rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		MaxLen: 1000,
		Values: map[string]interface{}{
			"data": msg,
		},
	}).Err()
	if err != nil {
		return fmt.Errorf("rdb.XAdd failed,err:%w", err)
	}
	return nil
}

// XReadGroupMsg 消费者从流中读消息
func XReadGroupMsg(stream, group, consumer string) (string, string, error) {
	ctx := context.Background()
	msgs, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    1,
		Block:    0,
	}).Result()
	if err != nil {
		return "", "", fmt.Errorf("rdb.XReadGroup failed,err:%w", err)
	}
	msgID := msgs[0].Messages[0].ID
	msg := msgs[0].Messages[0].Values["data"]
	return msgID, msg.(string), nil
}

// XAckMsg 确认消息，保证不被重复读
func XAckMsg(msgID string, stream string, group string) error {
	ctx := context.Background()
	err := rdb.XAck(ctx, stream, group, msgID).Err()
	if err != nil {
		return fmt.Errorf("rdb.XAck failed,err:%w", err)
	}
	return nil
}

// XRangeMsg 遍历流返回n条消息
func XRangeMsg(stream string, n int) ([]string, error) {
	res := make([]string, 0, n)
	ctx := context.Background()
	msgs, err := rdb.XRevRangeN(ctx, stream, "+", "-", int64(n)).Result()
	if err != nil {
		return nil, fmt.Errorf("rdb.XRangeN failed,err:%w", err)
	}
	for _, msg := range msgs {
		res = append(res, msg.Values["data"].(string))
	}
	slices.Reverse(res)
	return res, nil
}
