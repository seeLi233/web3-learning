package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"task10/internal/config"
	"task10/internal/db"
	"task10/internal/eth"
	"task10/internal/watcher"

	"github.com/ethereum/go-ethereum/common"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()

	// 校验必填配置
	if cfg.RPCURL == "" {
		log.Fatal("❌ 请设置环境变量 SEPOLIA_RPC_URL")
	}

	// 1. 连接以太坊
	log.Println("🔗 连接以太坊节点...")
	ethClient, err := eth.NewClient(cfg.RPCURL, cfg.WsRPCURL)
	if err != nil {
		log.Fatalf("❌ 以太坊连接失败: %v", err)
	}
	defer ethClient.Close()
	log.Println("✅ 以太坊连接成功")

	// 2. 连接 Redis
	log.Println("🔗 连接 Redis...")
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
	})
	defer rdb.Close()

	// 测试 Redis 连接
	_, err = rdb.Ping(context.Background()).Result()
	if err != nil {
		log.Fatalf("❌ Redis 连接失败: %v（请先启动 redis-server）", err)
	}
	log.Println("✅ Redis 连接成功")

	// 3. 连接 PostgreSQL
	log.Println("🔗 连接 PostgreSQL...")
	pg, err := db.NewPostgres(cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPass, cfg.DBName)
	if err != nil {
		log.Fatalf("❌ PostgreSQL 连接失败: %v", err)
	}
	defer func() {
		sqlDB, _ := pg.DB()
		sqlDB.Close()
	}()
	log.Println("✅ PostgreSQL 连接成功")

	// 4. 创建事件监听器
	contractAddr := common.HexToAddress(cfg.ContractAddr)
	log.Printf("🎯 监听合约: %s\n", contractAddr.Hex())

	w := watcher.New(ethClient, rdb, pg, contractAddr)

	// 5. 优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("收到信号 %v，正在关闭...", sig)
		cancel()
	}()

	// 6. 启动！
	if err := w.Start(ctx); err != nil {
		if err == context.Canceled {
			log.Println("✅ 监听器正常退出")
		} else {
			log.Fatalf("❌ 监听器异常退出: %v", err)
		}
	}
}
