package cross

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
)

// CrossMessage represents a message being executed from one chain to another.
type CrossMessage struct {
	// Source represents the chain and block where the message originated
	Source struct {
		Chain     types.ChainIndex
		BlockNum  uint64
		Timestamp uint64
	}
	// Destination represents the chain where the message will be executed
	Destination struct {
		Chain types.ChainIndex
	}
	// Message details
	LogIdx  uint32
	LogHash common.Hash
}

// NewCrossMessage creates a new CrossMessage from an ExecutingMessage
func NewCrossMessage(msg *types.ExecutingMessage, destChain types.ChainIndex) *CrossMessage {
	return &CrossMessage{
		Source: struct {
			Chain     types.ChainIndex
			BlockNum  uint64
			Timestamp uint64
		}{
			Chain:     msg.Chain,
			BlockNum:  msg.BlockNum,
			Timestamp: msg.Timestamp,
		},
		Destination: struct {
			Chain types.ChainIndex
		}{
			Chain: destChain,
		},
		LogIdx:  msg.LogIdx,
		LogHash: msg.Hash,
	}
}

// String returns a human-readable representation of the message
func (m *CrossMessage) String() string {
	return fmt.Sprintf("CrossMessage(from chain %d block %d to chain %d)", m.Source.Chain, m.Source.BlockNum, m.Destination.Chain)
}

// ContainsQuery returns a query to check if this message exists in a block
func (m *CrossMessage) ContainsQuery() types.ContainsQuery {
	return types.ContainsQuery{
		Timestamp: m.Source.Timestamp,
		BlockNum:  m.Source.BlockNum,
		LogIdx:    m.LogIdx,
		LogHash:   m.LogHash,
	}
}

// ToExecutingMessage converts this CrossMessage back to an ExecutingMessage
func (m *CrossMessage) ToExecutingMessage() *types.ExecutingMessage {
	return &types.ExecutingMessage{
		Chain:     m.Source.Chain,
		Timestamp: m.Source.Timestamp,
		BlockNum:  m.Source.BlockNum,
		LogIdx:    m.LogIdx,
		Hash:      m.LogHash,
	}
}
