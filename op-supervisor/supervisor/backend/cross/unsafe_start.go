package cross

import (
	"errors"
	"fmt"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
)

type UnsafeStartDeps interface {
	Contains(chain eth.ChainID, query types.ContainsQuery) (includedIn types.BlockSeal, err error)

	IsCrossUnsafe(chainID eth.ChainID, block eth.BlockID) error

	DependencySet() depset.DependencySet
}

// CrossUnsafeHazards checks if the given messages all exist and pass invariants.
// It returns a hazard-set: if any intra-block messaging happened,
// these hazard blocks have to be verified.
func CrossUnsafeHazards(d UnsafeStartDeps, chainID eth.ChainID,
	candidate types.BlockSeal, execMsgs []*types.ExecutingMessage) (hazards map[types.ChainIndex]types.BlockSeal, err error) {

	// Create a HazardSet for tracking dependencies
	unsafeDeps := &unsafeDeps{UnsafeStartDeps: d}
	hs, err := NewHazardSet(unsafeDeps)
	if err != nil {
		return nil, fmt.Errorf("failed to create hazard set: %w", err)
	}

	// Warning for future: If we have sub-second distinct blocks (different block number),
	// we need to increase precision on the above timestamp invariant.
	// Otherwise a local block can depend on a future local block of the same chain,
	// simply by pulling in a block of another chain,
	// which then depends on a block of the original chain,
	// all with the same timestamp, without message cycles.

	depSet := d.DependencySet()

	if len(execMsgs) > 0 {
		if ok, err := depSet.CanExecuteAt(chainID, candidate.Timestamp); err != nil {
			return nil, fmt.Errorf("cannot check message execution of block %s (chain %s): %w", candidate, chainID, err)
		} else if !ok {
			return nil, fmt.Errorf("cannot execute messages in block %s (chain %s): %w", candidate, chainID, types.ErrConflict)
		}
	}

	// check all executing messages
	for _, msg := range execMsgs {
		initChainID, err := depSet.ChainIDFromIndex(msg.Chain)
		if err != nil {
			if errors.Is(err, types.ErrUnknownChain) {
				err = fmt.Errorf("msg %s may not execute from unknown chain %s: %w", msg, msg.Chain, types.ErrConflict)
			}
			return nil, err
		}
		if ok, err := depSet.CanInitiateAt(initChainID, msg.Timestamp); err != nil {
			return nil, fmt.Errorf("cannot check message initiation of msg %s (chain %s): %w", msg, chainID, err)
		} else if !ok {
			return nil, fmt.Errorf("cannot allow initiating message %s (chain %s): %w", msg, chainID, types.ErrConflict)
		}

		// Find the block containing this message
		includedIn, err := d.Contains(initChainID,
			types.ContainsQuery{
				Timestamp: msg.Timestamp,
				BlockNum:  msg.BlockNum,
				LogIdx:    msg.LogIdx,
				LogHash:   msg.Hash,
			})
		if err != nil {
			return nil, fmt.Errorf("executing msg %s failed check: %w", msg, err)
		}

		// Verify timestamp invariants
		if msg.Timestamp < candidate.Timestamp {
			// If timestamp is older: invariant ensures non-cyclic ordering relative to other messages.
			// Check that the block that they are included in is cross-unsafe already.
			if err := d.IsCrossUnsafe(initChainID, includedIn.ID()); err != nil {
				return nil, fmt.Errorf("msg %s included in non-cross-unsafe block %s: %w", msg, includedIn, err)
			}
			if includedIn.Timestamp != msg.Timestamp {
				return nil, fmt.Errorf("executing msg %s exists, but has different timestamp than block %s: %w", msg, includedIn, types.ErrConflict)
			}
		} else if msg.Timestamp == candidate.Timestamp {
			// If timestamp is equal: we have to inspect ordering of individual
			// log events to ensure non-cyclic cross-chain message ordering.
			// And since we may have back-and-forth messaging, we cannot wait till the initiating side is cross-unsafe.
			// Thus check that it was included in a local-unsafe block,
			// and then proceed with transitive block checks,
			// to ensure the local block we depend on is becoming cross-unsafe also.

			// Add to hazard set for tracking
			if err := hs.Add(initChainID, includedIn, nil); err != nil {
				return nil, fmt.Errorf("failed to track hazard: %w", err)
			}
		} else {
			// Timestamp invariant is broken: executing message tries to execute future block.
			// The predeploy inbox contract should not have allowed this executing message through.
			return nil, fmt.Errorf("executing message %s in %s breaks timestamp invariant", msg, candidate)
		}
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
