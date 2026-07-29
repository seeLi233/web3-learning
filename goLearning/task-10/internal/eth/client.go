package eth

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Client 封装 HTTP + WebSocket 两个连接
type Client struct {
	httpClient *ethclient.Client
	wsClient   *ethclient.Client
}

// NewClient 创建客户端（HTTP 必填，WS 可选）
func NewClient(httpRPC, wsRPC string) (*Client, error) {
	httpClient, err := ethclient.Dial(httpRPC)
	if err != nil {
		return nil, fmt.Errorf("HTTP 连接失败: %w", err)
	}

	var wsClient *ethclient.Client
	if wsRPC != "" {
		wsClient, err = ethclient.Dial(wsRPC)
		if err != nil {
			log.Printf("⚠️  WebSocket 连接失败 (将使用轮询模式): %v", err)
			wsClient = nil
		}
	}

	return &Client{
		httpClient: httpClient,
		wsClient:   wsClient,
	}, nil
}

func (c *Client) Close() {
	if c.httpClient != nil {
		c.httpClient.Close()
	}
	if c.wsClient != nil {
		c.wsClient.Close()
	}
}

// GetLatestBlock 获取最新区块号
func (c *Client) GetLatestBlock(ctx context.Context) (uint64, error) {
	header, err := c.httpClient.HeaderByNumber(ctx, nil)
	if err != nil {
		return 0, err
	}
	return header.Number.Uint64(), nil
}

// GetBlockTimestamp 获取区块的时间戳
func (c *Client) GetBlockTimestamp(ctx context.Context, blockNum uint64) (uint64, error) {
	header, err := c.httpClient.HeaderByNumber(ctx, new(big.Int).SetUint64(blockNum))
	if err != nil {
		return 0, err
	}
	return header.Time, nil
}

// HasWebSocket 是否支持 WebSocket 订阅
func (c *Client) HasWebSocket() bool {
	return c.wsClient != nil
}

// FilterTransferLogs 历史回溯：查询指定区块范围的 Transfer 事件
func (c *Client) FilterTransferLogs(ctx context.Context, contractAddr common.Address, fromBlock, toBlock uint64) ([]types.Log, error) {
	transferTopic := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

	query := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(fromBlock),
		ToBlock:   new(big.Int).SetUint64(toBlock),
		Addresses: []common.Address{contractAddr},
		Topics: [][]common.Hash{{
			transferTopic,
		}},
	}
	return c.httpClient.FilterLogs(ctx, query)
}

// SubscribeTransferLogs WebSocket 实时订阅 Transfer 事件
func (c *Client) SubscribeTransferLogs(ctx context.Context, contractAddr common.Address) (chan types.Log, ethereum.Subscription, error) {
	if c.wsClient == nil {
		return nil, nil, fmt.Errorf("WebSocket 不可用")
	}

	transferTopic := crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddr},
		Topics: [][]common.Hash{{
			transferTopic,
		}},
	}

	logsCh := make(chan types.Log, 100)
	sub, err := c.wsClient.SubscribeFilterLogs(ctx, query, logsCh)
	if err != nil {
		return nil, nil, err
	}

	return logsCh, sub, nil
}

// WaitForConfirmations 等待区块获得 N 个确认（避免 reorg 影响）
func (c *Client) WaitForConfirmations(ctx context.Context, blockNum uint64, confirmations int) error {
	for {
		latest, err := c.GetLatestBlock(ctx)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		if latest >= blockNum+uint64(confirmations) {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
}
