package redis

import (
	"bluebell_microservices/common/pkg/logger"
	"bluebell_microservices/post-service/internal/dao/mysql"
	"bluebell_microservices/post-service/internal/model"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"go.uber.org/zap"
)

const (
	OneWeekInSeconds          = 7 * 24 * 3600        // 一周的秒数
	OneMonthInSeconds         = 4 * OneWeekInSeconds // 一个月的秒数
	VoteScore         float64 = 432                  // 每一票的值432分
	PostPerAge                = 20                   // 每页显示20条帖子
)

// 投票相关错误
var (
	ErrorVoteTimeExpire = errors.New("投票时间已过")
	ErrVoteRepeated     = errors.New("不允许重复投票")
)

func GetPostIDsInOrder(req *model.ParamPostList) ([]string, error) {
	// 从redis获取id
	// 1.根据用户请求中携带的order参数确定要查询的redis key
	key := KeyPostTimeZSet             // 默认是时间
	if req.Order == KeyPostScoreZSet { // 按照分数请求
		key = KeyPostScoreZSet
	}

	logger.Info("Getting post IDs from Redis",
		zap.String("key", key),
		zap.Int64("page", req.Page),
		zap.Int64("size", req.Size),
		zap.String("search", req.Search),
		zap.Int64("community_id", req.CommunityID))

	// 2.如果有搜索关键词，直接从MySQL获取匹配的帖子ID
	if req.Search != "" {
		// 从MySQL中获取匹配的帖子ID
		matchedIDs, err := mysql.GetPostIDsBySearch(req.Search, req.Page, req.Size, req.CommunityID)
		if err != nil {
			logger.Error("Failed to get matched post IDs from MySQL", zap.Error(err))
			return nil, err
		}
		return matchedIDs, nil
	}

	// 3.如果没有搜索关键词，直接从Redis获取分页后的ID
	return getIDsFormKey(key, req.Page, req.Size)
}

// getIDsFormKey 按照分数从大到小的顺序查询指定数量的元素
func getIDsFormKey(key string, page, size int64) ([]string, error) {
	start := (page - 1) * size
	end := start + size - 1

	logger.Info("Getting post IDs from Redis key",
		zap.String("key", key),
		zap.Int64("start", start),
		zap.Int64("end", end))

	// 3.ZRevRange 按照分数从大到小的顺序查询指定数量的元素
	ids, err := client.ZRevRange(key, start, end).Result()
	if err != nil {
		logger.Error("Failed to get post IDs from Redis",
			zap.String("key", key),
			zap.Error(err))
		return nil, err
	}

	logger.Info("Got post IDs from Redis",
		zap.String("key", key),
		zap.Int("count", len(ids)),
		zap.Strings("ids", ids))

	return ids, nil
}

func GetPostVoteData(ids []string) (data []int64, err error) {
	data = make([]int64, 0, len(ids))
	for _, id := range ids {
		key := KeyPostVotedZSetPrefix + id
		// 查找key中分数是1的元素数量 -> 统计每篇帖子的赞成票的数量
		v := client.ZCount(key, "1", "1").Val()
		data = append(data, v)
	}
	// 使用 pipeline一次发送多条命令减少RTT
	//pipeline := client.Pipeline()
	//for _, id := range ids {
	//	key := KeyCommunityPostSetPrefix + id
	//	pipeline.ZCount(key, "1", "1") // ZCount会返回分数在min和max范围内的成员数量
	//}
	//cmders, err := pipeline.Exec()
	//if err != nil {
	//	return nil, err
	//}
	//data = make([]int64, 0, len(cmders))
	//for _, cmder := range cmders {
	//	v := cmder.(*redis.IntCmd).Val()
	//	data = append(data, v)
	//}
	return data, nil
}

func GetCommunityPostIDsInOrder(p *model.ParamPostList) ([]string, error) {
	// 1.根据用户请求中携带的order参数确定要查询的redis key
	orderkey := KeyPostTimeZSet      // 默认是时间
	if p.Order == model.OrderScore { // 按照分数请求
		orderkey = KeyPostScoreZSet
	}

	// 社区的key
	cKey := KeyCommunityPostSetPrefix + strconv.Itoa(int(p.CommunityID))

	// 利用缓存key减少zinterstore执行的次数 缓存key
	key := orderkey + strconv.Itoa(int(p.CommunityID))
	if client.Exists(key).Val() < 1 {
		// 不存在，需要计算
		pipeline := client.Pipeline()
		pipeline.ZInterStore(key, redis.ZStore{
			Aggregate: "MAX", // 将两个zset函数聚合的时候 求最大值
		}, cKey, orderkey) // zinterstore 计算
		pipeline.Expire(key, 60*time.Second) // 设置超时时间
		_, err := pipeline.Exec()
		if err != nil {
			return nil, err
		}
	}

	// 2.如果有搜索关键词，直接从MySQL获取匹配的帖子ID
	if p.Search != "" {
		// 从MySQL中获取匹配的帖子ID
		matchedIDs, err := mysql.GetPostIDsBySearch(p.Search, p.Page, p.Size, p.CommunityID)
		if err != nil {
			logger.Error("Failed to get matched post IDs from MySQL", zap.Error(err))
			return nil, err
		}
		return matchedIDs, nil
	}

	// 3.如果没有搜索关键词，直接从Redis获取分页后的ID
	return getIDsFormKey(key, p.Page, p.Size)
}

func CreatePost(postID, authorID uint64, title, content string, communityID uint64) error {
	now := float64(time.Now().Unix())
	votedKey := KeyPostVotedZSetPrefix + strconv.Itoa(int(postID))
	communityKey := KeyCommunityPostSetPrefix + strconv.Itoa(int(communityID))

	logger.Info("Creating post in Redis",
		zap.Uint64("postID", postID),
		zap.Uint64("authorID", authorID),
		zap.Uint64("communityID", communityID))

	postInfo := map[string]interface{}{
		"title":    title,
		"content":  content,
		"post_id":  postID,
		"user_id":  authorID, // 修改键名，确保与其他地方一致
		"time":     now,
		"votes":    1,
		"comments": 0,
	}

	// 事务操作
	pipeline := client.TxPipeline()
	// 投票 zSet
	pipeline.ZAdd(votedKey, redis.Z{ // 作者默认投赞成票
		Score:  1,
		Member: authorID,
	})
	pipeline.Expire(votedKey, time.Second*OneMonthInSeconds*6) // 过期时间：6个月
	// 文章 hash
	pipeline.HMSet(KeyPostInfoHashPrefix+strconv.Itoa(int(postID)), postInfo)
	// 添加到分数 ZSet
	pipeline.ZAdd(KeyPostScoreZSet, redis.Z{
		Score:  now + VoteScore,
		Member: postID,
	})
	// 添加到时间 ZSet
	pipeline.ZAdd(KeyPostTimeZSet, redis.Z{
		Score:  now,
		Member: postID,
	})
	// 添加到对应版块 把帖子添加到社区 set
	pipeline.SAdd(communityKey, postID)
	_, err := pipeline.Exec()
	if err != nil {
		logger.Error("Failed to execute Redis pipeline",
			zap.Uint64("postID", postID),
			zap.Error(err))
		return err
	}

	logger.Info("Successfully created post in Redis",
		zap.Uint64("postID", postID),
		zap.Uint64("authorID", authorID),
		zap.Float64("time", now),
		zap.String("title", title))

	return nil
}

func GetPostVoteNum(id int64) (int64, error) {
	key := KeyPostVotedZSetPrefix + strconv.Itoa(int(id))
	voteNum, err := client.ZCount(key, "1", "1").Result()
	if err != nil {
		logger.Error("Failed to get post vote number from Redis",
			zap.String("key", key),
			zap.Error(err))
		return 0, err
	}
	return voteNum, nil
}

// SetVoteStatus 设置投票状态
func SetVoteStatus(postID, userID int64, status int64, expiration time.Duration) error {
	redisClient := Client()
	voteStatusKey := fmt.Sprintf("bluebell-plus:vote:status:%d:%d", postID, userID)
	return redisClient.Set(voteStatusKey, status, expiration).Err()
}

// CreatePostVote 创建帖子投票记录
func CreatePostVote(postID, userID, direction int64) error {
	// 1.判断投票限制
	// 去redis取帖子发布时间
	postIDStr := strconv.FormatInt(postID, 10)
	userIDStr := strconv.FormatInt(userID, 10)
	postTime := client.ZScore(KeyPostTimeZSet, postIDStr).Val()
	if float64(time.Now().Unix())-postTime > OneWeekInSeconds { // 超过一个星期就不允许投票了
		// 不允许投票了
		return ErrorVoteTimeExpire
	}

	// 2、获取用户之前的投票记录
	key := KeyPostVotedZSetPrefix + postIDStr
	ov := client.ZScore(key, userIDStr).Val()

	// direction为当前票值
	v := float64(direction)

	// 如果这一次投票的值和之前保存的值一致，就提示不允许重复投票
	if v == ov {
		return ErrVoteRepeated
	}

	// 3、计算投票方向和分数变化
	var op float64
	if v > ov {
		op = 1
	} else {
		op = -1
	}
	diffAbs := math.Abs(ov - v) // 计算两次投票的差值

	// 4、使用事务进行投票更新
	pipeline := client.TxPipeline()

	// 4.1、更新帖子分数
	incrementScore := VoteScore * diffAbs * op // 计算分数变化
	_, err := pipeline.ZIncrBy(KeyPostScoreZSet, incrementScore, postIDStr).Result()
	if err != nil {
		return err
	}

	// 4.2、记录用户为该帖子的投票数据
	if v == 0 {
		// 取消投票，从集合中删除记录
		_, err = client.ZRem(key, userIDStr).Result()
		if err != nil {
			return err
		}
	} else {
		// 记录投票信息
		pipeline.ZAdd(key, redis.Z{
			Score:  v, // 赞成票(1)或反对票(-1)
			Member: userIDStr,
		})
	}

	// 4.3、更新帖子的投票总数
	// 允许投票数为负数，直接增减即可
	pipeline.HIncrBy(KeyPostInfoHashPrefix+postIDStr, "votes", int64(op))

	// 5、执行事务
	_, err = pipeline.Exec()
	return err
}

// LockKey 生成锁的键
func LockKey(postID, userID int64) string {
	return fmt.Sprintf("bluebell-plus:vote:lock:%d:%d", postID, userID)
}

// AcquireLock 获取分布式锁
func AcquireLock(postID, userID int64, expiration time.Duration) (bool, error) {
	redisClient := Client()
	lockKey := LockKey(postID, userID)

	// 使用SETNX命令获取锁
	success, err := redisClient.SetNX(lockKey, "1", expiration).Result()
	if err != nil {
		return false, err
	}

	return success, nil
}

// ReleaseLock 释放分布式锁
func ReleaseLock(postID, userID int64) error {
	redisClient := Client()
	lockKey := LockKey(postID, userID)

	// 删除锁
	return redisClient.Del(lockKey).Err()
}
