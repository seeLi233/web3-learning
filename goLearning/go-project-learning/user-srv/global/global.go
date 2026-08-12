package global

import (
	pb "github.com/go-project-learning/project/user-srv/api/pb"
	"github.com/go-redis/redis/v8"
)

var (
	UserClient pb.UserServiceClient
	RedisCli   *redis.Client // 如果需要直接访问底层 client
)
