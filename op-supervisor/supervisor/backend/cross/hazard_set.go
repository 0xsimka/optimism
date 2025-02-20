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

// Add adds a block and its recursive dependencies to the hazard set
func (h *HazardSet) Add(chainID eth.ChainID, block types.BlockSeal, execMsgs []*types.ExecutingMessage) error {
	fmt.Printf("Adding block to hazard set: chain=%s number=%d timestamp=%d hash=0x%x\n", chainID, block.Number, block.Timestamp, block.Hash)
	depSet := h.deps.DependencySet()

	// Get chain index for this chain ID
	chainIndex, err := h.deps.ChainIndexFromID(chainID)
	if err != nil {
		return fmt.Errorf("failed to get chain index: %w", err)
	}
	fmt.Printf("  Chain index: %d\n", chainIndex)

	// Verify the block itself
	if err := h.deps.VerifyBlock(chainID, block.ID()); err != nil {
		return fmt.Errorf("block verification failed: %w", err)
	}
	fmt.Printf("  Block verified\n")

	// Check if we can execute messages at this timestamp
	if len(execMsgs) > 0 {
		if ok, err := depSet.CanExecuteAt(chainID, block.Timestamp); err != nil {
			return fmt.Errorf("cannot check message execution of block %s (chain %s): %w", block, chainID, err)
		} else if !ok {
			return fmt.Errorf("cannot execute messages in block %s (chain %s): %w", block, chainID, types.ErrConflict)
		}
		fmt.Printf("  Can execute messages at timestamp %d\n", block.Timestamp)
	}

	// Process each executing message
	for i, msg := range execMsgs {
		fmt.Printf("  Processing message[%d]: chain=%d timestamp=%d blockNum=%d logIdx=%d\n", i, msg.Chain, msg.Timestamp, msg.BlockNum, msg.LogIdx)

		// Convert to CrossMessage for clearer source/target chain data
		crossMsg := &CrossMessage{
			Source: struct {
				Chain     types.ChainIndex
				BlockNum  uint64
				Timestamp uint64
			}{
				Chain:     msg.Chain, // Source chain is where the message came from
				BlockNum:  msg.BlockNum,
				Timestamp: msg.Timestamp,
			},
			Destination: struct {
				Chain types.ChainIndex
			}{
				Chain: chainIndex, // Destination chain is where we're executing (chainIndex)
			},
			LogIdx:  msg.LogIdx,
			LogHash: msg.Hash,
		}
		fmt.Printf("    CrossMessage: from chain %d block %d to chain %d\n", crossMsg.Source.Chain, crossMsg.Source.BlockNum, crossMsg.Destination.Chain)

		initChainID, err := depSet.ChainIDFromIndex(crossMsg.Source.Chain)
		if err != nil {
			if errors.Is(err, types.ErrUnknownChain) {
				err = fmt.Errorf("msg %s may not execute from unknown chain %s: %w", crossMsg, crossMsg.Source.Chain, types.ErrConflict)
			}
			return err
		}
		fmt.Printf("    Source chain ID: %s\n", initChainID)

		// Check if we can initiate messages at this timestamp
		if ok, err := depSet.CanInitiateAt(initChainID, crossMsg.Source.Timestamp); err != nil {
			return fmt.Errorf("cannot check message initiation of msg %s (chain %s): %w", crossMsg, chainID, err)
		} else if !ok {
			return fmt.Errorf("cannot allow initiating message %s (chain %s): %w", crossMsg, chainID, types.ErrConflict)
		}
		fmt.Printf("    Can initiate messages at timestamp %d\n", crossMsg.Source.Timestamp)

		// Find the block containing this message
		includedIn, err := h.deps.Contains(initChainID, types.ContainsQuery{
			Timestamp: msg.Timestamp,
			BlockNum:  msg.BlockNum,
			LogIdx:    msg.LogIdx,
			LogHash:   msg.Hash,
		})
		if err != nil {
			return fmt.Errorf("executing msg %s failed check: %w", crossMsg, err)
		}
		fmt.Printf("    Found dependency block: number=%d timestamp=%d hash=0x%x\n", includedIn.Number, includedIn.Timestamp, includedIn.Hash)

		// Add to hazards if not already present
		// Store in source chain's index since that's where the dependency is
		sourceIndex := crossMsg.Source.Chain
		if existing, ok := h.hazards[sourceIndex]; ok {
			// Check if this block matches our existing dependency
			if existing != includedIn {
				return fmt.Errorf("message %s depends on block %s but already depend on %s", crossMsg, includedIn, existing)
			}
			fmt.Printf("    Already have dependency for chain %d\n", sourceIndex)
		} else {
			h.hazards[sourceIndex] = includedIn
			fmt.Printf("    Added dependency for chain %d: number=%d timestamp=%d hash=0x%x\n", sourceIndex, includedIn.Number, includedIn.Timestamp, includedIn.Hash)
		}
	}

	fmt.Printf("  Current hazards:\n")
	for chainIdx, seal := range h.hazards {
		fmt.Printf("    Chain[%d]: number=%d timestamp=%d hash=0x%x\n", chainIdx, seal.Number, seal.Timestamp, seal.Hash)
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
