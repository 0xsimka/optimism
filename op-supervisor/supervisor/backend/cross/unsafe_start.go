package cross

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type UnsafeStartDeps interface {
	Contains(chain eth.ChainID, query types.ContainsQuery) (includedIn types.BlockSeal, err error)

	IsCrossUnsafe(chainID eth.ChainID, block eth.BlockID) error

	DependencySet() depset.DependencySet

	OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
}

// CrossUnsafeHazards checks if the given messages all exist and pass invariants.
// It returns a hazard-set: if any intra-block messaging happened,
// these hazard blocks have to be verified.
func CrossUnsafeHazards(d UnsafeStartDeps, chainID eth.ChainID,
	candidate types.BlockSeal) (hazards map[types.ChainIndex]types.BlockSeal, err error) {

	// Create a HazardSet for tracking dependencies
	unsafeDeps := &unsafeDeps{UnsafeStartDeps: d}
	hs, err := NewHazardSet(unsafeDeps)
	if err != nil {
		return nil, fmt.Errorf("failed to create hazard set: %w", err)
	}

	// Add the candidate block to collect all dependencies
	if err := hs.Add(chainID, candidate); err != nil {
		return nil, fmt.Errorf("failed to collect hazards: %w", err)
	}

	return hs.Dependencies(), nil
}

// unsafeDeps adapts UnsafeStartDeps to HazardDeps
type unsafeDeps struct {
	UnsafeStartDeps
}

// VerifyBlock implements HazardDeps by checking cross-unsafe status
func (d *unsafeDeps) VerifyBlock(chainID eth.ChainID, block eth.BlockID) error {
	// For unsafe hazards, verify that the block is cross-unsafe
	if err := d.IsCrossUnsafe(chainID, block); err != nil {
		return fmt.Errorf("block %s not cross-unsafe: %w", block, err)
	}
	return nil
}

// ChainIndexFromID implements HazardDeps by using the dependency set
func (d *unsafeDeps) ChainIndexFromID(id eth.ChainID) (types.ChainIndex, error) {
	return d.DependencySet().ChainIndexFromID(id)
}

// OpenBlock implements HazardDeps by using the dependency set
func (d *unsafeDeps) OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
	// For unsafe hazards, verify that the block is cross-unsafe
	block := eth.BlockID{Number: blockNum}
	if err := d.IsCrossUnsafe(chainID, block); err != nil {
		return eth.BlockRef{}, 0, nil, fmt.Errorf("block %s not cross-unsafe: %w", block, err)
	}

	// Open the block to get executing messages
	ref, logCount, execMsgs, err = d.UnsafeStartDeps.OpenBlock(chainID, blockNum)
	if err != nil {
		return eth.BlockRef{}, 0, nil, err
	}

	// Verify the block again with the actual block hash
	block.Hash = ref.Hash
	if err := d.IsCrossUnsafe(chainID, block); err != nil {
		return eth.BlockRef{}, 0, nil, fmt.Errorf("block %s not cross-unsafe: %w", block, err)
	}

	// Check timestamp invariants for all messages
	for _, msg := range execMsgs {
		if msg.Timestamp > ref.Time {
			return eth.BlockRef{}, 0, nil, fmt.Errorf("executing message %s in %s breaks timestamp invariant", msg, ref)
		}
	}

	// If we get here, the block is verified to be cross-unsafe
	// Return the messages but don't add to hazard set
	return ref, logCount, execMsgs, nil
}
