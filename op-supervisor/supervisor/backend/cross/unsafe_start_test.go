package cross

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestCrossUnsafeHazards(t *testing.T) {
	t.Run("empty execMsgs", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		// when there are no execMsgs,
		// no work is done, and no error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.NoError(t, err)
		require.Empty(t, hazards)
	})
	t.Run("CanExecuteAt returns false", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{
			canExecuteAtfn: func() (bool, error) {
				return false, nil
			},
		}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 1, map[uint32]*types.ExecutingMessage{0: {}}, nil
		}
		// when there is one execMsg, and CanExecuteAt returns false,
		// no work is done and an error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.ErrorIs(t, err, types.ErrConflict)
		require.Empty(t, hazards)
	})
	t.Run("CanExecuteAt returns error", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{
			canExecuteAtfn: func() (bool, error) {
				return false, errors.New("some error")
			},
		}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 1, map[uint32]*types.ExecutingMessage{0: {}}, nil
		}
		// when there is one execMsg, and CanExecuteAt returns false,
		// no work is done and an error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.ErrorContains(t, err, "some error")
		require.Empty(t, hazards)
	})
	t.Run("unknown chain", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{
			chainIDFromIndexfn: func() (eth.ChainID, error) {
				return eth.ChainID{}, types.ErrUnknownChain
			},
		}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 1, map[uint32]*types.ExecutingMessage{0: {}}, nil
		}
		// when there is one execMsg, and ChainIDFromIndex returns ErrUnknownChain,
		// an error is returned as a ErrConflict
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.ErrorIs(t, err, types.ErrConflict)
		require.Empty(t, hazards)
	})
	t.Run("ChainIDFromUInt64 returns error", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{
			chainIDFromIndexfn: func() (eth.ChainID, error) {
				return eth.ChainID{}, errors.New("some error")
			},
		}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 1, map[uint32]*types.ExecutingMessage{0: {}}, nil
		}
		// when there is one execMsg, and ChainIDFromIndex returns some other error,
		// the error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.ErrorContains(t, err, "some error")
		require.Empty(t, hazards)
	})
	t.Run("CanInitiateAt returns false", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{
			canInitiateAtfn: func() (bool, error) {
				return false, nil
			},
		}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 1, map[uint32]*types.ExecutingMessage{0: {}}, nil
		}
		// when there is one execMsg, and CanInitiateAt returns false,
		// the error is returned as a ErrConflict
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.ErrorIs(t, err, types.ErrConflict)
		require.Empty(t, hazards)
	})
	t.Run("CanInitiateAt returns error", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{
			canInitiateAtfn: func() (bool, error) {
				return false, errors.New("some error")
			},
		}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 1, map[uint32]*types.ExecutingMessage{0: {}}, nil
		}
		// when there is one execMsg, and CanInitiateAt returns an error,
		// the error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.ErrorContains(t, err, "some error")
		require.Empty(t, hazards)
	})
	t.Run("timestamp is greater than candidate", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.deps = mockDependencySet{}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 10}
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 1, map[uint32]*types.ExecutingMessage{0: em1}, nil
		}
		// when there is one execMsg, and the timestamp is greater than the candidate,
		// an error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.ErrorContains(t, err, "breaks timestamp invariant")
		require.Empty(t, hazards)
	})
	t.Run("timestamp is equal, Check returns error", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.checkFn = func() (includedIn types.BlockSeal, err error) {
			return types.BlockSeal{}, fmt.Errorf("failed to open block BlockSeal(hash:0x0000000000000000000000000000000000000000000000000000000000000000, number:0, time:2): executing message ExecMsg(chainIndex: 0, block: 0, log: 0, time: 2, logHash: 0x0000000000000000000000000000000000000000000000000000000000000000) in 0x0000000000000000000000000000000000000000000000000000000000000000:0 breaks timestamp invariant")
		}
		usd.deps = mockDependencySet{}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 2}
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 1, map[uint32]*types.ExecutingMessage{0: em1}, nil
		}
		// when there is one execMsg, and the timetamp is equal to the candidate,
		// and check returns an error,
		// that error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.ErrorContains(t, err, "breaks timestamp invariant")
		require.Empty(t, hazards)
	})
	t.Run("timestamp is equal, same hazard twice", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		sampleBlockSeal := types.BlockSeal{Number: 3, Hash: common.BytesToHash([]byte{0x02})}
		usd.checkFn = func() (includedIn types.BlockSeal, err error) {
			return sampleBlockSeal, nil
		}
		usd.deps = mockDependencySet{}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 2}
		em2 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 2}
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 2, map[uint32]*types.ExecutingMessage{0: em1, 1: em2}, nil
		}
		// when there are two execMsgs, and both are equal time to the candidate,
		// and check returns the same includedIn for both
		// they load the hazards once, and return no error
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.NoError(t, err)
		require.Equal(t, hazards, map[types.ChainIndex]types.BlockSeal{types.ChainIndex(0): sampleBlockSeal})
	})
	t.Run("timestamp is equal, different hazards", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		// set the check function to return a different BlockSeal for the second call
		sampleBlockSeal := types.BlockSeal{Number: 3, Hash: common.BytesToHash([]byte{0x02})}
		sampleBlockSeal2 := types.BlockSeal{Number: 333, Hash: common.BytesToHash([]byte{0x22})}
		calls := 0
		usd.checkFn = func() (includedIn types.BlockSeal, err error) {
			defer func() { calls++ }()
			if calls == 0 {
				return sampleBlockSeal, nil
			}
			return sampleBlockSeal2, nil
		}
		usd.deps = mockDependencySet{}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 2}
		em2 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 2}
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 2, map[uint32]*types.ExecutingMessage{0: em1, 1: em2}, nil
		}
		// when there are two execMsgs, and both are equal time to the candidate,
		// and check returns different includedIn for the two,
		// an error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.ErrorContains(t, err, "but already depend on")
		require.Empty(t, hazards)
	})
	t.Run("timestamp is less, check returns error", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		usd.checkFn = func() (includedIn types.BlockSeal, err error) {
			return types.BlockSeal{}, fmt.Errorf("failed to open block BlockSeal(hash:0x0000000000000000000000000000000000000000000000000000000000000000, number:0, time:2): executing message ExecMsg(chainIndex: 0, block: 0, log: 0, time: 1, logHash: 0x0000000000000000000000000000000000000000000000000000000000000000) in 0x0000000000000000000000000000000000000000000000000000000000000000:0 breaks timestamp invariant")
		}
		usd.deps = mockDependencySet{}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 1}
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 1, map[uint32]*types.ExecutingMessage{0: em1}, nil
		}
		// when there is one execMsg, and the timestamp is less than the candidate,
		// and check returns an error,
		// that error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.ErrorContains(t, err, "breaks timestamp invariant")
		require.Empty(t, hazards)
	})
	t.Run("timestamp is less, IsCrossUnsafe returns error", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		sampleBlockSeal := types.BlockSeal{Number: 3, Hash: common.BytesToHash([]byte{0x02})}
		usd.checkFn = func() (includedIn types.BlockSeal, err error) {
			return sampleBlockSeal, nil
		}
		usd.isCrossUnsafeFn = func() error {
			return errors.New("some error")
		}
		usd.deps = mockDependencySet{}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 1}
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 1, map[uint32]*types.ExecutingMessage{0: em1}, nil
		}
		// when there is one execMsg, and the timestamp is less than the candidate,
		// and IsCrossUnsafe returns an error,
		// that error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.ErrorContains(t, err, "some error")
		require.Empty(t, hazards)
	})
	t.Run("timestamp is less, IsCrossUnsafe", func(t *testing.T) {
		usd := &mockUnsafeStartDeps{}
		sampleBlockSeal := types.BlockSeal{Number: 3, Hash: common.BytesToHash([]byte{0x02})}
		usd.checkFn = func() (includedIn types.BlockSeal, err error) {
			return sampleBlockSeal, nil
		}
		usd.isCrossUnsafeFn = func() error {
			return nil
		}
		usd.deps = mockDependencySet{}
		chainID := eth.ChainIDFromUInt64(0)
		candidate := types.BlockSeal{Timestamp: 2}
		usd.candidate = candidate
		em1 := &types.ExecutingMessage{Chain: types.ChainIndex(0), Timestamp: 0}
		usd.openBlockFn = func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			return eth.BlockRef{}, 1, map[uint32]*types.ExecutingMessage{0: em1}, nil
		}
		// when there is one execMsg, and the timestamp is less than the candidate,
		// and IsCrossUnsafe returns no error,
		// no error is returned
		hazards, err := CrossUnsafeHazards(usd, chainID, candidate)
		require.NoError(t, err)
		require.Empty(t, hazards)
	})
}

type mockUnsafeStartDeps struct {
	deps            mockDependencySet
	checkFn         func() (includedIn types.BlockSeal, err error)
	isCrossUnsafeFn func() error
	openBlockFn     func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
	candidate       types.BlockSeal
}

func (m *mockUnsafeStartDeps) Contains(chain eth.ChainID, query types.ContainsQuery) (includedIn types.BlockSeal, err error) {
	if m.checkFn != nil {
		return m.checkFn()
	}
	// Check timestamp invariant - message timestamp must not be greater than candidate timestamp
	if query.Timestamp > m.candidate.Timestamp {
		return types.BlockSeal{}, fmt.Errorf("message timestamp %d breaks timestamp invariant with block timestamp %d", query.Timestamp, m.candidate.Timestamp)
	}
	// Return a BlockSeal with the same timestamp as the message
	return types.BlockSeal{Timestamp: query.Timestamp}, nil
}

func (m *mockUnsafeStartDeps) IsCrossUnsafe(chainID eth.ChainID, derived eth.BlockID) error {
	if m.isCrossUnsafeFn != nil {
		return m.isCrossUnsafeFn()
	}
	return nil
}

func (m *mockUnsafeStartDeps) DependencySet() depset.DependencySet {
	return m.deps
}

func (m *mockUnsafeStartDeps) OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
	if m.openBlockFn != nil {
		return m.openBlockFn(chainID, blockNum)
	}
	// Default implementation returns block with matching timestamp to avoid invariant errors
	// Return timestamp matching the candidate timestamp
	execMsgs = make(map[uint32]*types.ExecutingMessage)
	// Only add a message if we have a non-zero timestamp
	if m.candidate.Timestamp > 0 {
		execMsgs[0] = &types.ExecutingMessage{
			Chain:     0,
			BlockNum:  blockNum,
			Timestamp: m.candidate.Timestamp,
			LogIdx:    0,
		}
	}
	return eth.BlockRef{Time: m.candidate.Timestamp}, uint32(len(execMsgs)), execMsgs, nil
}
