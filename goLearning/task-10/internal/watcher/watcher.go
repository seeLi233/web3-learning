package watcher

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"task10/internal/db"
	"task10/internal/eth"
	"task10/internal/model"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// EventWatcher 区块链事件监听器
// 职责：历史回溯 + 实时订阅 + Redis 去重
type EventWatcher struct {
	ethClient     *eth.Client
	redis         *redis.Client
	pg            *gorm.DB // ← 新增：PostgreSQL
	contractAddr  common.Address
	lastBlock     uint64 // 已处理到哪个区块
	confirmations int    // 等几个确认后才处理事件
}

// New 创建事件监听器
func New(ethClient *eth.Client, rdb *redis.Client, pg *gorm.DB, contractAddr common.Address) *EventWatcher {
	return &EventWatcher{
		ethClient:     ethClient,
		redis:         rdb,
		pg:            pg,
		contractAddr:  contractAddr,
		lastBlock:     0,
		confirmations: 2, // 等 2 个确认，避免 reorg
	}
}

// Start 启动事件监听（入口函数）
func (w *EventWatcher) Start(ctx context.Context) error {
	fmt.Println("🚀 事件监听器启动中...")

	// ====== Phase 1: 历史回溯 ======
	fmt.Println("\n📥 Phase 1: 历史回溯")
	latestBlock, err := w.ethClient.GetLatestBlock(ctx)
	if err != nil {
		return fmt.Errorf("获取最新区块失败: %w", err)
	}

	// 从 Redis 读取上次处理到的区块号
	w.lastBlock = w.getCheckpoint()
	// 首次启动（断点为 0）时，只回溯最近 1000 个区块，不从头扫
	if w.lastBlock == 0 && latestBlock > 1000 {
		w.lastBlock = latestBlock - 1000
		fmt.Printf("   💡 首次启动，从最近 1000 个区块开始\n")
	}
	fmt.Printf("   已处理到区块: %d\n", w.lastBlock)
	fmt.Printf("   最新区块:     %d\n", latestBlock)

	if w.lastBlock < latestBlock {
		// 回溯时等 2 个确认（保险起见）
		toBlock := latestBlock - uint64(w.confirmations)
		if toBlock > w.lastBlock {
			fromBlock := w.lastBlock + 1
			gap := toBlock - fromBlock + 1
			fmt.Printf("   ⏳ 回溯 %d 个区块 [%d → %d]...\n", gap, fromBlock, toBlock)

			count, err := w.syncRange(ctx, fromBlock, toBlock)
			if err != nil {
				log.Printf("   ⚠️  回溯出错: %v", err)
			} else {
				fmt.Printf("   ✅ 回溯完成，处理了 %d 条事件\n", count)
			}
		}
		w.lastBlock = toBlock
		w.saveCheckpoint(w.lastBlock)
	}

	// ====== Phase 2: 实时监听 ======
	fmt.Println("\n📡 Phase 2: 实时监听")

	if w.ethClient.HasWebSocket() {
		fmt.Println("   使用 WebSocket 订阅模式")
		return w.subscriptionLoop(ctx)
	}

	fmt.Println("   WebSocket 不可用，使用轮询模式")
	return w.pollingLoop(ctx)
}

// ==================== 历史回溯 ====================

// syncRange 同步一个区块范围的事件（分批查询，每批最多 2000 个区块）
func (w *EventWatcher) syncRange(ctx context.Context, from, to uint64) (int, error) {
	batchSize := uint64(2000)
	total := 0

	for start := from; start <= to; start += batchSize {
		end := start + batchSize - 1
		if end > to {
			end = to
		}

		logs, err := w.ethClient.FilterTransferLogs(ctx, w.contractAddr, start, end)
		if err != nil {
			return total, fmt.Errorf("查询区块 [%d-%d] 日志失败: %w", start, end, err)
		}

		for _, vLog := range logs {
			if w.handleEvent(vLog) {
				total++
			}
		}

		// RPC 限流
		if end-start > 500 {
			time.Sleep(200 * time.Millisecond)
		}
	}

	return total, nil
}

// ==================== 实时订阅 (WebSocket) ====================

func (w *EventWatcher) subscriptionLoop(ctx context.Context) error {
	logsCh, sub, err := w.ethClient.SubscribeTransferLogs(ctx, w.contractAddr)
	if err != nil {
		return fmt.Errorf("订阅失败: %w", err)
	}
	defer sub.Unsubscribe()

	fmt.Println("   ✅ 已订阅 Transfer 事件，等待事件中...")
	fmt.Println("   (按 Ctrl+C 退出)")

	// 定时检查轮询（兜底：每 30 秒对账一次，防止订阅漏事件）
	pollTicker := time.NewTicker(30 * time.Second)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n👋 监听器已停止")
			return ctx.Err()

		// 收到新事件
		case vLog := <-logsCh:
			// 只处理确认区块的事件
			latest, _ := w.ethClient.GetLatestBlock(ctx)
			if vLog.BlockNumber+uint64(w.confirmations) <= latest {
				w.handleEvent(vLog)
			}
			// 如果区块还不够老，先跳过（后续对账会补上）

		// WebSocket 断线 → 重连
		case err := <-sub.Err():
			log.Printf("   🔌 订阅断开: %v，5 秒后重连...", err)
			time.Sleep(5 * time.Second)
			logsCh, sub, err = w.ethClient.SubscribeTransferLogs(ctx, w.contractAddr)
			if err != nil {
				log.Printf("   ❌ 重连失败: %v，切换为轮询模式", err)
				return w.pollingLoop(ctx)
			}
			defer sub.Unsubscribe()
			log.Println("   ✅ 已重新连接")

		// 每 30 秒对账
		case <-pollTicker.C:
			w.reconcile(ctx)
		}
	}
}

// ==================== 轮询模式 (WebSocket 不可用时的降级方案) ====================
func (w *EventWatcher) pollingLoop(ctx context.Context) error {
	fmt.Println("   🔁 轮询模式（每 12 秒查询一次，匹配以太坊出块时间）")
	fmt.Println("   (按 Ctrl+C 退出)")

	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n👋 监听器已停止")
			return ctx.Err()
		case <-ticker.C:
			w.reconcile(ctx)
		}
	}
}

// ==================== 对账（历史回溯 + 实时之间的桥梁） ====================

func (w *EventWatcher) reconcile(ctx context.Context) {
	latest, err := w.ethClient.GetLatestBlock(ctx)
	if err != nil {
		log.Printf("   ⚠️  获取最新区块失败: %v", err)
		return
	}

	toBlock := latest - uint64(w.confirmations)
	if toBlock <= w.lastBlock {
		return // 没有新区块
	}

	fromBlock := w.lastBlock + 1
	count, err := w.syncRange(ctx, fromBlock, toBlock)
	if err != nil {
		log.Printf("   ⚠️  对账失败 [%d→%d]: %v", fromBlock, toBlock, err)
		return
	}

	if count > 0 {
		log.Printf("   📦 对账 [%d→%d]: %d 条事件", fromBlock, toBlock, count)
	}

	w.lastBlock = latest
	w.saveCheckpoint(w.lastBlock)
}

// ==================== 单条事件处理 ====================

// handleEvent 处理一条原始日志（解析 + 去重 + 打印）
// 返回: true=新事件, false=重复/已处理
func (w *EventWatcher) handleEvent(vLog types.Log) bool {
	// 1. 解析日志
	event := model.ParseTransferLog(vLog)

	// 2. Redis SETNX 去重（24 小时过期）
	dedupKey := event.DedupKey()
	ok, err := w.redis.SetNX(context.Background(), dedupKey, "1", 24*time.Hour).Result()
	if err != nil {
		log.Printf("   ⚠️  Redis 操作失败: %v", err)
		// return false // Redis 挂了不阻塞，继续走 DB 层
	}
	if !ok {
		return false // 已处理过
	}

	// PostgreSQL UPSERT 兜底去重 + 持久化
	dbEvent := db.FromTransferEvent(event)
	saved, err := db.SaveEvent(w.pg, dbEvent)
	if err != nil {
		log.Printf("   ⚠️  DB 保存失败: %v", err)
		// 保存失败时，删除 Redis key（下次重试）
		w.redis.Del(context.Background(), dedupKey)
		return false
	}

	if !saved {
		// DB 里已存在（ON CONFLICT DO NOTHING），说明之前 Redis 的 key 过期/丢失了
		// 补上 Redis key
		w.redis.Set(context.Background(), dedupKey, "1", 24*time.Hour)
		return false
	}

	// 3. 打印事件
	w.printEvent(event)

	return true
}

// ==================== 断点续传 ====================
func (w *EventWatcher) getCheckpoint() uint64 {
	// 优先从数据库获取（最可靠）
	if w.pg != nil {
		block, err := db.GetLastProcessedBlock(w.pg)
		if err == nil && block > 0 {
			return block
		}
	}

	val, err := w.redis.Get(context.Background(), "watcher:last_block").Result()
	if err == redis.Nil {
		return 0 // 首次启动
	}
	if err != nil {
		log.Printf("   ⚠️  读取断点失败: %v，从头开始", err)
		return 0
	}
	return parseUint64(val)
}

func (w *EventWatcher) saveCheckpoint(block uint64) {
	err := w.redis.Set(context.Background(), "watcher:last_block", fmt.Sprintf("%d", block), 0).Err()
	if err != nil {
		log.Printf("   ⚠️  保存断点失败: %v", err)
	}
}

// ==================== 打印 ====================

func (w *EventWatcher) printEvent(e *model.TransferEvent) {
	from := shorten(e.From)
	to := shorten(e.To)

	// 格式化 value（假设 decimals=18，我们用 6 位精度显示）
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	human := new(big.Float).Quo(
		new(big.Float).SetInt(e.Value),
		new(big.Float).SetInt(divisor),
	)

	fmt.Printf("🔔 Transfer | %s → %s | %s | blk=%d | tx=%s\n",
		from, to, human.Text('f', 2), e.BlockNumber, shorten(e.TxHash))
}

// ==================== 小工具 ====================

func shorten(s string) string {
	if len(s) <= 10 {
		return s
	}
	return s[:6] + "..." + s[len(s)-4:]
}

func parseUint64(s string) uint64 {
	var n uint64
	fmt.Sscanf(s, "%d", &n)
	return n
}
