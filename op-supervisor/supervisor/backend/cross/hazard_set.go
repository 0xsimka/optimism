package cross

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

// HazardDeps abstracts the differences between safe/unsafe implementations
type HazardDeps interface {
	// Contains checks if a message exists in a block
	Contains(chain eth.ChainID, query types.ContainsQuery) (types.BlockSeal, error)
	// DependencySet provides access to chain dependencies
	DependencySet() depset.DependencySet
	// VerifyBlock verifies a block according to safe/unsafe rules
	VerifyBlock(chainID eth.ChainID, block eth.BlockID) error
	// ChainIndexFromID converts a ChainID to a ChainIndex
	// ChainIndexFromID(id eth.ChainID) (types.ChainIndex, error)
	// ChainIDFromIndex converts a ChainIndex to a ChainID
	// ChainIDFromIndex(index types.ChainIndex) (eth.ChainID, error)
	// OpenBlock opens a block to access its executing messages
	OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
}

// HazardSet tracks blocks that must be checked before a candidate can be promoted
type HazardSet struct {
	// hazards maps chain indices to their block seals
	hazards map[types.ChainIndex]types.BlockSeal
	// deps provides access to dependency verification
	deps HazardDeps
}

// NewHazardSet creates a new HazardSet with the given dependencies
func NewHazardSet(deps HazardDeps) (*HazardSet, error) {
	if deps == nil {
		return nil, fmt.Errorf("hazard dependencies cannot be nil")
	}
	return &HazardSet{
		hazards: make(map[types.ChainIndex]types.BlockSeal),
		deps:    deps,
	}, nil
}

// blockToProcess represents a block that needs to be processed for hazards
type blockToProcess struct {
	chainID eth.ChainID
	block   types.BlockSeal
}

// Add adds a block to the hazard set and recursively adds any blocks that it depends on.
// If a block has already been added, it will be skipped.
// If a block has already been added with a different hash, an error will be returned.
func (h *HazardSet) Add(chainID eth.ChainID, block types.BlockSeal) error {
	depSet := h.deps.DependencySet()

	// Initialize stack with the first block
	stack := []blockToProcess{{chainID: chainID, block: block}}

	// Process blocks until the stack is empty
	for len(stack) > 0 {
		// Pop the next block to process
		next := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Convert chain ID to index
		chainIndex, err := depSet.ChainIndexFromID(next.chainID)
		if err != nil {
			return fmt.Errorf("failed to get chain index: %w", err)
		}
		fmt.Println("ChainIndexFromID", chainIndex)

		// Open the block to get its messages
		ref, _, execMsgs, err := h.deps.OpenBlock(next.chainID, next.block.Number)
		if err != nil {
			return fmt.Errorf("failed to open block: %w", err)
		}

		// Verify the block matches what we expect
		if ref.Hash != next.block.Hash {
			return fmt.Errorf("block hash mismatch: expected %s, got %s", next.block.Hash, ref.Hash)
		}

		// Process each message in the block
		for _, msg := range execMsgs {
			fmt.Println("Processing message", msg)
			// Skip messages targeting the source chain
			// if msg.Chain == chainIndex {
			// 	fmt.Println("Skipping message", msg)
			// 	continue
			// }

			msgChainID, err := depSet.ChainIDFromIndex(msg.Chain)
			if err != nil {
				return fmt.Errorf("failed to get chain ID: %w", err)
			}
			fmt.Println("Message chain ID", msgChainID)

			// Check if the message can be initiated at this time
			canInit, err := depSet.CanInitiateAt(msgChainID, msg.Timestamp)
			if err != nil {
				return fmt.Errorf("message %s cannot be initiated: %w", NewCrossMessage(msg, chainIndex), err)
			}
			if !canInit {
				return fmt.Errorf("message %s cannot be initiated: %w", NewCrossMessage(msg, chainIndex), types.ErrConflict)
			}

			// Check if the message can be executed at this time
			canExec, err := depSet.CanExecuteAt(msgChainID, msg.Timestamp)
			if err != nil {
				return fmt.Errorf("message %s cannot be executed: %w", NewCrossMessage(msg, chainIndex), err)
			}
			if !canExec {
				return fmt.Errorf("message %s cannot be executed: %w", NewCrossMessage(msg, chainIndex), types.ErrConflict)
			}

			// Check if the message exists in its chain
			seal, err := h.deps.Contains(msgChainID, types.ContainsQuery{
				BlockNum:  msg.BlockNum,
				Timestamp: msg.Timestamp,
			})
			if err != nil {
				return fmt.Errorf("failed to check if message exists: %w", err)
			}

			// If we already have a different block for this chain, return an error
			if existingSeal, ok := h.hazards[msg.Chain]; ok {
				if existingSeal != seal {
					return fmt.Errorf("message %s depends on block %s but already depend on %s",
						NewCrossMessage(msg, chainIndex),
						seal,
						existingSeal)
				}
				continue
			}

			// Check if the block is already cross-safe/unsafe
			// err = h.deps.VerifyBlock(msgChainID, eth.BlockID{
			// 	Hash:   seal.Hash,
			// 	Number: seal.Number,
			// })

			// If the block is not cross-safe/unsafe, add it to hazards and process its messages
			// if err != nil {
			// Add the block to hazards
			h.hazards[msg.Chain] = seal

			// Add the block to the stack for processing
			stack = append(stack, blockToProcess{
				chainID: msgChainID,
				block:   seal,
			})
			// }
		}
	}

	return nil
}

// Dependencies returns a copy of the hazard set's dependencies
func (h *HazardSet) Dependencies() map[types.ChainIndex]types.BlockSeal {
	deps := make(map[types.ChainIndex]types.BlockSeal, len(h.hazards))
	for k, v := range h.hazards {
		deps[k] = v
	}
	return deps
}
