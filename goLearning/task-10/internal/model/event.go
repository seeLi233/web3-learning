package model

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// TransferEvent 解析后的 Transfer 事件
type TransferEvent struct {
	TxHash      string
	BlockNumber uint64
	LogIndex    uint
	Contract    string
	From        string
	To          string
	Value       *big.Int
	RawData     string
}

// DedupKey 生成去重 key: "event:processed:{txHash}:{logIndex}"
func (e *TransferEvent) DedupKey() string {
	return "event:processed:" + e.TxHash + ":" + uintToStr(e.LogIndex)
}

func uintToStr(n uint) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// ParseTransferLog 从原始 Log 解析出 TransferEvent
// Transfer(address indexed from, address indexed to, uint256 value)
func ParseTransferLog(vLog types.Log) *TransferEvent {
	var from, to common.Address
	if len(vLog.Topics) > 1 {
		// topic[1] 是 indexed from，存在后 20 字节
		from = common.BytesToAddress(vLog.Topics[1].Bytes())
	}
	if len(vLog.Topics) > 2 {
		// topic[2] 是 indexed to，存在后 20 字节
		to = common.BytesToAddress(vLog.Topics[2].Bytes())
	}

	// data 区存的是非 indexed 的 value (uint256)
	value := new(big.Int).SetBytes(vLog.Data)

	return &TransferEvent{
		TxHash:      vLog.TxHash.Hex(),
		BlockNumber: vLog.BlockNumber,
		LogIndex:    vLog.Index,
		Contract:    vLog.Address.Hex(),
		From:        from.Hex(),
		To:          to.Hex(),
		Value:       value,
		RawData:     common.Bytes2Hex(vLog.Data),
	}
}
