package cross

import (
	"fmt"
	"testing"

	"github.com/ethereum-optimism/optimism/op-service/eth"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/backend/depset"
	"github.com/ethereum-optimism/optimism/op-supervisor/supervisor/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// newTestDepSet creates a StaticConfigDependencySet for testing with predefined chain IDs and indices
func newTestDepSet() depset.DependencySet {
	deps := make(map[eth.ChainID]*depset.StaticConfigDependency)
	// Add test chains - use enough to handle deep recursion test
	for i := uint64(0); i < 100; i++ {
		chainID := eth.ChainIDFromUInt64(i)
		deps[chainID] = &depset.StaticConfigDependency{
			ChainIndex:     types.ChainIndex(i),
			ActivationTime: 0, // Allow execution at any time
			HistoryMinTime: 0, // Allow initiation at any time
		}
	}
	ds, err := depset.NewStaticConfigDependencySet(deps)
	if err != nil {
		panic(fmt.Sprintf("failed to create test dependency set: %v", err))
	}
	return ds
}

// mockHazardDeps implements HazardDeps for testing
type mockHazardDeps struct {
	containsFn    func(chain eth.ChainID, query types.ContainsQuery) (types.BlockSeal, error)
	verifyBlockFn func(chainID eth.ChainID, block eth.BlockID) error
	openBlockFn   func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error)
	deps          depset.DependencySet
}

func (m *mockHazardDeps) Contains(chain eth.ChainID, query types.ContainsQuery) (types.BlockSeal, error) {
	if m.containsFn != nil {
		return m.containsFn(chain, query)
	}
	return types.BlockSeal{}, nil
}

func (m *mockHazardDeps) VerifyBlock(chainID eth.ChainID, block eth.BlockID) error {
	if m.verifyBlockFn != nil {
		return m.verifyBlockFn(chainID, block)
	}
	return nil
}

func (m *mockHazardDeps) OpenBlock(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
	if m.openBlockFn != nil {
		return m.openBlockFn(chainID, blockNum)
	}
	return eth.BlockRef{}, 0, nil, nil
}

func (m *mockHazardDeps) DependencySet() depset.DependencySet {
	return m.deps
}

func (m *mockHazardDeps) ChainIndexFromID(id eth.ChainID) (types.ChainIndex, error) {
	return m.deps.ChainIndexFromID(id)
}

// Helper functions to make test data creation more concise
func makeBlock(number, timestamp uint64, chain types.ChainIndex, messages ...*types.ExecutingMessage) blockDef {
	return blockDef{
		number:    number,
		timestamp: timestamp,
		chain:     chain,
		hash:      common.Hash{byte(chain), byte(number)}, // Deterministic hash based on chain and number
		messages:  messages,
	}
}

func makeMessage(chain types.ChainIndex, blockNum, timestamp uint64, logIdx uint32) *types.ExecutingMessage {
	return &types.ExecutingMessage{
		Chain:     chain,
		BlockNum:  blockNum,
		Timestamp: timestamp,
		LogIdx:    logIdx,
	}
}

func makeBlockSeal(number, timestamp uint64, chain types.ChainIndex) types.BlockSeal {
	return types.BlockSeal{
		Number:    number,
		Timestamp: timestamp,
		Hash:      common.Hash{byte(chain), byte(number)}, // Match block hash generation
	}
}

// Test vectors representing different dependency scenarios
type testVector struct {
	name      string
	blocks    []blockDef
	expected  map[types.ChainIndex]types.BlockSeal
	expectErr error
}

type blockDef struct {
	number    uint64
	timestamp uint64
	hash      common.Hash
	chain     types.ChainIndex
	messages  []*types.ExecutingMessage
}

func TestHazardSet_Add(t *testing.T) {
	vectors := []testVector{
		{
			name: "Empty Message List",
			blocks: []blockDef{
				makeBlock(1, 1, 0),
			},
			expected: map[types.ChainIndex]types.BlockSeal{},
		},
		{
			name: "Single Dependency",
			blocks: []blockDef{
				makeBlock(1, 1, 0, makeMessage(1, 1, 1, 1)),
				makeBlock(1, 1, 1), // Referenced block from chain 1
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 1, 1),
			},
		},
		{
			name: "Multiple Messages Same Block",
			blocks: []blockDef{
				makeBlock(1, 1, 0,
					makeMessage(1, 1, 1, 1),
					makeMessage(1, 1, 1, 2),
				),
				makeBlock(1, 1, 1),
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 1, 1),
			},
		},
		{
			name: "Messages From Different Blocks Error",
			blocks: []blockDef{
				makeBlock(1, 1, 0,
					makeMessage(1, 1, 1, 1),
					makeMessage(1, 2, 1, 1),
				),
				makeBlock(1, 1, 1),
				makeBlock(2, 1, 1),
			},
			expectErr: fmt.Errorf("message %s depends on block %s but already depend on %s",
				&CrossMessage{Source: struct {
					Chain     types.ChainIndex
					BlockNum  uint64
					Timestamp uint64
				}{Chain: 1, BlockNum: 2, Timestamp: 1}},
				makeBlockSeal(2, 1, 1),
				makeBlockSeal(1, 1, 1)),
		},
		{
			name: "Dependencies Across Multiple Chains",
			blocks: []blockDef{
				makeBlock(1, 1, 0,
					makeMessage(1, 1, 1, 1),
					makeMessage(2, 1, 1, 1),
				),
				makeBlock(1, 1, 1),
				makeBlock(1, 1, 2),
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 1, 1),
				2: makeBlockSeal(1, 1, 2),
			},
		},
		{
			name: "Recursive Dependencies",
			blocks: []blockDef{
				makeBlock(1, 1, 0, makeMessage(1, 1, 1, 1)),
				makeBlock(1, 1, 1, makeMessage(2, 1, 1, 1)),
				makeBlock(1, 1, 2),
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 1, 1),
				2: makeBlockSeal(1, 1, 2),
			},
		},
		{
			name: "Recursive Dependencies - Missing Intermediate Block",
			blocks: []blockDef{
				// Chain 0 -> Chain 1 -> Chain 2, but Chain 1's block is missing
				makeBlock(1, 1, 0, makeMessage(1, 1, 1, 1)),
				// Block 1 in Chain 1 is missing
				makeBlock(1, 1, 2),
			},
			expectErr: fmt.Errorf("failed to check if message exists: block not found: %w", types.ErrFuture),
		},
		{
			name: "Recursive Dependencies - Missing Final Block",
			blocks: []blockDef{
				// Chain 0 -> Chain 1 -> Chain 2, but Chain 2's block is missing
				makeBlock(1, 1, 0, makeMessage(1, 1, 1, 1)),
				makeBlock(1, 1, 1, makeMessage(2, 1, 1, 1)),
				// Block 1 in Chain 2 is missing
			},
			expectErr: fmt.Errorf("failed to check if message exists: block not found: %w", types.ErrFuture),
		},
		{
			name: "Recursive Dependencies - Diamond Pattern",
			blocks: []blockDef{
				// Chain 0 points to both Chain 1 and Chain 2
				// Chain 1 and Chain 2 both point to Chain 3
				makeBlock(1, 1, 0,
					makeMessage(1, 1, 1, 1),
					makeMessage(2, 1, 1, 1),
				),
				makeBlock(1, 1, 1, makeMessage(3, 1, 1, 1)),
				makeBlock(1, 1, 2, makeMessage(3, 1, 1, 1)),
				makeBlock(1, 1, 3),
			},
			expected: map[types.ChainIndex]types.BlockSeal{
				1: makeBlockSeal(1, 1, 1),
				2: makeBlockSeal(1, 1, 2),
				3: makeBlockSeal(1, 1, 3),
			},
		},
		{
			name: "Recursive Dependencies - Diamond Pattern Missing Final",
			blocks: []blockDef{
				makeBlock(1, 1, 0,
					makeMessage(1, 1, 1, 1),
					makeMessage(2, 1, 1, 1),
				),
				makeBlock(1, 1, 1, makeMessage(3, 1, 1, 1)),
				makeBlock(1, 1, 2, makeMessage(3, 1, 1, 1)),
				// Block 1 in Chain 3 is missing
			},
			expectErr: fmt.Errorf("failed to check if message exists: block not found: %w", types.ErrFuture),
		},
		{
			name: "Invalid Timestamp - Future Message",
			blocks: []blockDef{
				makeBlock(1, 1, 0, makeMessage(1, 1, 2, 1)), // Message timestamp > block timestamp
				makeBlock(1, 1, 1),
			},
			expectErr: fmt.Errorf("message timestamp 2 breaks timestamp invariant with block timestamp 1"),
		},
		{
			name: "Invalid Timestamp - Zero",
			blocks: []blockDef{
				makeBlock(1, 1, 0, makeMessage(1, 1, 0, 1)),
				makeBlock(1, 1, 1),
			},
			expectErr: fmt.Errorf("failed to check if message exists: block not found: %w", types.ErrFuture),
		},
		{
			name: "Missing Block - Message References Non-existent Block",
			blocks: []blockDef{
				makeBlock(1, 1, 0, makeMessage(1, 999, 1, 1)), // Block 999 doesn't exist
			},
			expectErr: fmt.Errorf("failed to check if message exists: block not found: %w", types.ErrFuture),
		},
		{
			name: "Missing Block - Chain Break",
			blocks: []blockDef{
				makeBlock(1, 1, 0, makeMessage(1, 1, 1, 1)),
				makeBlock(1, 1, 1, makeMessage(2, 1, 1, 1)), // Message references block in chain 2 that doesn't exist
			},
			expectErr: fmt.Errorf("failed to check if message exists: block not found: %w", types.ErrFuture),
		},
		{
			name: "Invalid Block Number - Zero",
			blocks: []blockDef{
				makeBlock(1, 1, 0, makeMessage(1, 0, 1, 1)), // Invalid block number
			},
			expectErr: fmt.Errorf("failed to check if message exists: block not found: %w", types.ErrFuture),
		},
	}

	for _, tc := range vectors {
		t.Run(tc.name, func(t *testing.T) {
			deps := setupMockDeps(t, tc)
			hs, err := NewHazardSet(deps)
			require.NoError(t, err)

			// Add each block
			for i, block := range tc.blocks {
				seal := types.BlockSeal{
					Number:    block.number,
					Timestamp: block.timestamp,
					Hash:      block.hash,
				}
				chainID := eth.ChainIDFromUInt64(uint64(block.chain))

				err := hs.Add(chainID, seal)
				if tc.expectErr != nil {
					require.Error(t, err, "expected error %s, got %v", tc.expectErr, err)
					require.Equal(t, tc.expectErr.Error(), err.Error(), "expected error %s, got %v", tc.expectErr, err)
					return
				}
				require.NoError(t, err)

				if i == len(tc.blocks)-1 {
					// Verify final state
					require.Equal(t, tc.expected, hs.Dependencies())
				}
			}
		})
	}
}

// setupMockDeps creates a mock dependency set for testing
func setupMockDeps(t *testing.T, tc testVector) *mockHazardDeps {
	t.Helper()

	// Create a map of all blocks for quick lookup
	blockMap := make(map[blockKey]blockDef)
	for _, block := range tc.blocks {
		key := blockKey{
			chain:     block.chain,
			number:    block.number,
			timestamp: block.timestamp,
		}
		blockMap[key] = block
	}

	deps := &mockHazardDeps{
		deps: newTestDepSet(),
		// Only implement the functions we actually use
		containsFn: func(chain eth.ChainID, query types.ContainsQuery) (types.BlockSeal, error) {
			chainIndex := types.ChainIndex(eth.EvilChainIDToUInt64(chain))
			key := blockKey{
				chain:     chainIndex,
				number:    query.BlockNum,
				timestamp: query.Timestamp,
			}
			t.Logf("Contains called with chain=%d, blockNum=%d, timestamp=%d", chainIndex, query.BlockNum, query.Timestamp)
			if block, ok := blockMap[key]; ok {
				t.Logf("Found block: chain=%d, number=%d, timestamp=%d", block.chain, block.number, block.timestamp)
				return types.BlockSeal{
					Number:    block.number,
					Timestamp: block.timestamp,
					Hash:      block.hash,
				}, nil
			}
			t.Logf("Block not found for key: chain=%d, number=%d, timestamp=%d", key.chain, key.number, key.timestamp)
			return types.BlockSeal{}, fmt.Errorf("block not found: %w", types.ErrFuture)
		},
		verifyBlockFn: func(chainID eth.ChainID, block eth.BlockID) error {
			chainIndex := types.ChainIndex(eth.EvilChainIDToUInt64(chainID))
			key := blockKey{
				chain:     chainIndex,
				number:    block.Number,
				timestamp: 0,
			}
			t.Logf("VerifyBlock called with chain=%d, blockNum=%d", chainIndex, block.Number)
			found := false
			for k, v := range blockMap {
				if k.chain == key.chain && k.number == key.number {
					if v.hash == block.Hash {
						found = true
						t.Logf("Found matching block with hash=%s", v.hash)
						break
					}
					t.Logf("Found block but hash mismatch: expected=%s, got=%s", block.Hash, v.hash)
				}
			}
			if !found {
				t.Logf("Block not found for chain=%d, number=%d", chainIndex, block.Number)
				return fmt.Errorf("block not found: %w", types.ErrConflict)
			}
			return nil
		},
		openBlockFn: func(chainID eth.ChainID, blockNum uint64) (ref eth.BlockRef, logCount uint32, execMsgs map[uint32]*types.ExecutingMessage, err error) {
			chainIndex := types.ChainIndex(eth.EvilChainIDToUInt64(chainID))
			key := blockKey{
				chain:     chainIndex,
				number:    blockNum,
				timestamp: 0,
			}
			t.Logf("OpenBlock called with chain=%d, blockNum=%d", chainIndex, blockNum)
			for k, v := range blockMap {
				if k.chain == key.chain && k.number == key.number {
					t.Logf("Found block with %d messages", len(v.messages))
					msgMap := make(map[uint32]*types.ExecutingMessage)
					for i, msg := range v.messages {
						msgMap[uint32(i)] = msg
						t.Logf("Message %d: chain=%d, blockNum=%d, timestamp=%d", i, msg.Chain, msg.BlockNum, msg.Timestamp)
					}
					return eth.BlockRef{
						Hash:   v.hash,
						Number: v.number,
						Time:   v.timestamp,
					}, uint32(len(v.messages)), msgMap, nil
				}
			}
			t.Logf("Block not found for chain=%d, number=%d", chainIndex, blockNum)
			return eth.BlockRef{}, 0, nil, fmt.Errorf("block not found: %w", types.ErrConflict)
		},
	}

	return deps
}

// blockKey is used for efficient block lookup in tests
type blockKey struct {
	chain     types.ChainIndex
	number    uint64
	timestamp uint64
}
