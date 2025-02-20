package cross

import (
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type SafeStartDeps interface {
	Contains(chain eth.ChainID, query types.ContainsQuery) (includedIn types.BlockSeal, err error)

	CrossDerivedToSource(chainID eth.ChainID, derived eth.BlockID) (source types.BlockSeal, err error)

	DependencySet() depset.DependencySet

	OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
}

// CrossSafeHazards checks if the given messages all exist and pass invariants.
// It returns a hazard-set: if any intra-block messaging happened,
// these hazard blocks have to be verified.
func CrossSafeHazards(d SafeStartDeps, chainID eth.ChainID, inL1Source eth.BlockID,
	candidate types.BlockSeal) (hazards map[types.ChainIndex]types.BlockSeal, err error) {

	// Create a HazardSet for tracking dependencies
	safeDeps := &safeDeps{
		SafeStartDeps: d,
		l1Source:      inL1Source,
	}
	hs, err := NewHazardSet(safeDeps)
	if err != nil {
		return nil, fmt.Errorf("failed to create hazard set: %w", err)
	}

	// Add the candidate block to collect all dependencies
	if err := hs.Add(chainID, candidate); err != nil {
		return nil, fmt.Errorf("failed to collect hazards: %w", err)
	}

	return hs.Dependencies(), nil
}

// safeDeps adapts SafeStartDeps to HazardDeps
type safeDeps struct {
	SafeStartDeps
	l1Source eth.BlockID
}

// VerifyBlock implements HazardDeps by checking cross-safe derivation
func (d *safeDeps) VerifyBlock(chainID eth.ChainID, block eth.BlockID) error {
	// For safe hazards, verify that the block is derived from a source within scope
	source, err := d.CrossDerivedToSource(chainID, block)
	if err != nil {
		return fmt.Errorf("block %s not cross-safe: %w", block, err)
	}
	if source.Number > d.l1Source.Number {
		return fmt.Errorf("block %s derived from %s which is not in cross-safe scope %s: %w",
			block, source, d.l1Source, types.ErrOutOfScope)
	}
	return nil
}

// ChainIndexFromID implements HazardDeps by using the dependency set
func (d *safeDeps) ChainIndexFromID(id eth.ChainID) (types.ChainIndex, error) {
	return d.DependencySet().ChainIndexFromID(id)
}

// OpenBlock implements HazardDeps by using the dependency set
func (d *safeDeps) OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
	// For safe hazards, verify that the block is derived from a source within scope
	block := eth.BlockID{Number: blockNum}
	source, err := d.CrossDerivedToSource(chainID, block)
	if err != nil {
		return eth.BlockRef{}, 0, nil, fmt.Errorf("block %s not cross-safe: %w", block, err)
	}
	if source.Number > d.l1Source.Number {
		return eth.BlockRef{}, 0, nil, fmt.Errorf("block %s derived from %s which is not in cross-safe scope %s: %w",
			block, source, d.l1Source, types.ErrOutOfScope)
	}

	// Open the block to get executing messages
	ref, logCount, execMsgs, err = d.SafeStartDeps.OpenBlock(chainID, blockNum)
	if err != nil {
		return eth.BlockRef{}, 0, nil, err
	}

	// Verify the block again with the actual block hash
	block.Hash = ref.Hash
	source, err = d.CrossDerivedToSource(chainID, block)
	if err != nil {
		return eth.BlockRef{}, 0, nil, fmt.Errorf("block %s not cross-safe: %w", block, err)
	}
	if source.Number > d.l1Source.Number {
		return eth.BlockRef{}, 0, nil, fmt.Errorf("block %s derived from %s which is not in cross-safe scope %s: %w",
			block, source, d.l1Source, types.ErrOutOfScope)
	}

	// If we get here, the block is verified to be within scope
	// Return the messages but don't add to hazard set
	return ref, logCount, execMsgs, nil
}
