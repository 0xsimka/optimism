package cross

import (
	"errors"
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
	ChainIndexFromID(id eth.ChainID) (types.ChainIndex, error)
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

// Add adds a block to the hazard set, recursively checking for dependencies
func (h *HazardSet) Add(chainID eth.ChainID, block types.BlockSeal) error {
	// Open the block to get its executing messages
	_, _, execMsgs, err := h.deps.OpenBlock(chainID, block.Number)
	if err != nil {
		return fmt.Errorf("failed to open block %s: %w", block, err)
	}

	// Check each executing message for dependencies
	for _, msg := range execMsgs {
		// Get the chain ID for the message's source chain
		srcChainID, err := h.deps.DependencySet().ChainIDFromIndex(msg.Chain)
		if err != nil {
			if errors.Is(err, types.ErrUnknownChain) {
				return fmt.Errorf("unknown chain %d: %w", msg.Chain, types.ErrConflict)
			}
			return fmt.Errorf("failed to get chain ID for index %d: %w", msg.Chain, err)
		}

		// Check if we can execute at this chain
		canExecute, err := h.deps.DependencySet().CanExecuteAt(srcChainID, block.Timestamp)
		if err != nil {
			return fmt.Errorf("failed to check if can execute at chain %s: %w", srcChainID, err)
		}
		if !canExecute {
			return fmt.Errorf("cannot execute at chain %s: %w", srcChainID, types.ErrConflict)
		}

		// Check if we can initiate at this chain
		canInitiate, err := h.deps.DependencySet().CanInitiateAt(srcChainID, msg.Timestamp)
		if err != nil {
			return fmt.Errorf("failed to check if can initiate at chain %s: %w", srcChainID, err)
		}
		if !canInitiate {
			return fmt.Errorf("cannot initiate at chain %s: %w", srcChainID, types.ErrConflict)
		}

		// Check the timestamp invariant
		if msg.Timestamp > block.Timestamp {
			return fmt.Errorf("message timestamp %d breaks timestamp invariant with block timestamp %d", msg.Timestamp, block.Timestamp)
		}

		// Check if the message exists in a block
		query := types.ContainsQuery{
			BlockNum:  msg.BlockNum,
			LogIdx:    msg.LogIdx,
			LogHash:   msg.Hash,
			Timestamp: msg.Timestamp,
		}
		includedIn, err := h.deps.Contains(srcChainID, query)
		if err != nil {
			return fmt.Errorf("failed to check if message exists: %w", err)
		}

		// If we already have a hazard for this chain, make sure it's the same one
		if existing, ok := h.hazards[msg.Chain]; ok {
			if existing != includedIn {
				destChain, err := h.deps.ChainIndexFromID(chainID)
				if err != nil {
					return fmt.Errorf("failed to get chain index: %w", err)
				}
				crossMsg := NewCrossMessage(msg, destChain)
				return fmt.Errorf("message %s depends on block %s but already depend on %s", crossMsg, includedIn, existing)
			}
			continue
		}

		// For messages with timestamps less than the candidate block,
		// verify the block before adding it to hazards
		if msg.Timestamp < block.Timestamp {
			msgBlock := eth.BlockID{Number: msg.BlockNum}
			if err := h.deps.VerifyBlock(srcChainID, msgBlock); err == nil {
				// Block is verified, don't add it to hazards
				continue
			}
		}

		// Add the hazard
		h.hazards[msg.Chain] = includedIn
	}

	return nil
}

// Dependencies returns all blocks that must be checked
func (h *HazardSet) Dependencies() map[types.ChainIndex]types.BlockSeal {
	// Return a copy to prevent modification
	deps := make(map[types.ChainIndex]types.BlockSeal, len(h.hazards))
	for k, v := range h.hazards {
		deps[k] = v
	}
	return deps
}
