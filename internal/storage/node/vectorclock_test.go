package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompare(t *testing.T) {
	tests := []struct {
		name     string
		a        VectorClock
		b        VectorClock
		expected int
	}{
		{
			name:     "equal empty",
			a:        VectorClock{},
			b:        VectorClock{},
			expected: 0,
		},
		{
			name:     "equal with same counters",
			a:        VectorClock{"node1": 1, "node2": 2},
			b:        VectorClock{"node1": 1, "node2": 2},
			expected: 0,
		},
		// Compare returns 1 when a > b (a dominates / happened after b)
		{
			name:     "a dominates b (a happened after b)",
			a:        VectorClock{"node1": 2},
			b:        VectorClock{"node1": 1},
			expected: 1,
		},
		{
			name:     "a dominates b with multiple entries",
			a:        VectorClock{"node1": 2, "node2": 1},
			b:        VectorClock{"node1": 1, "node2": 1},
			expected: 1,
		},
		// Compare returns -1 when a < b (a happened before b)
		{
			name:     "a happened before b",
			a:        VectorClock{"node1": 1},
			b:        VectorClock{"node1": 2},
			expected: -1,
		},
		{
			name:     "concurrent — neither dominates",
			a:        VectorClock{"node1": 1, "node2": 0},
			b:        VectorClock{"node1": 0, "node2": 1},
			expected: 2,
		},
		{
			name:     "concurrent — both have extra keys the other doesn't",
			a:        VectorClock{"node1": 1, "node3": 5},
			b:        VectorClock{"node2": 1, "node4": 5},
			expected: 2,
		},
		{
			name:     "a has key b doesn't with counter > 0 — a dominates",
			a:        VectorClock{"node1": 1},
			b:        VectorClock{},
			expected: 1,
		},
		{
			name:     "a has zero entry same as b's missing key",
			a:        VectorClock{"node1": 0},
			b:        VectorClock{},
			expected: 0,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result := Compare(tc.a, tc.b)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCompare_TiebreakerSum(t *testing.T) {
	// When concurrent, Sum determines winner in processReadResults
	vcA := VectorClock{"node1": 1, "node2": 0} // Sum = 1
	vcB := VectorClock{"node1": 0, "node2": 1} // Sum = 1 — equal Sum!
	// With equal Sum, lexicographic nodeID decides: "node2" > "node1" so vcB wins

	cmp := Compare(vcA, vcB)
	assert.Equal(t, 2, cmp, "should be concurrent")

	// Sum tiebreaker
	assert.Equal(t, uint64(1), vcA.Sum())
	assert.Equal(t, uint64(1), vcB.Sum())

	// Lexicographic tiebreaker: "node2" > "node1"
	assert.Greater(t, "node2", "node1")
}

func TestVectorClock_Sum(t *testing.T) {
	vc := VectorClock{"node1": 5, "node2": 3, "node3": 1}
	assert.Equal(t, uint64(9), vc.Sum())

	empty := VectorClock{}
	assert.Equal(t, uint64(0), empty.Sum())
}

func TestVectorClock_IsNewerThan(t *testing.T) {
	newer := VectorClock{"node1": 2, "node2": 1}
	older := VectorClock{"node1": 1, "node2": 1}

	assert.True(t, newer.IsNewerThan(older))
	assert.False(t, older.IsNewerThan(newer))
	assert.False(t, newer.IsNewerThan(newer))
}

func TestVectorClock_IsConcurrentWith(t *testing.T) {
	vcA := VectorClock{"node1": 1}
	vcB := VectorClock{"node2": 1}

	assert.True(t, vcA.IsConcurrentWith(vcB))
	assert.False(t, vcA.IsConcurrentWith(vcA))
}

func TestVectorClock_Merge(t *testing.T) {
	a := VectorClock{"node1": 5, "node2": 1}
	b := VectorClock{"node1": 3, "node3": 2}

	a.Merge(b)

	assert.Equal(t, uint64(5), a["node1"], "max preserved")
	assert.Equal(t, uint64(1), a["node2"], "unchanged")
	assert.Equal(t, uint64(2), a["node3"], "added from b")
}

func TestVectorClock_Prune(t *testing.T) {
	vc := VectorClock{"node1": 15, "node2": 8, "node3": 3}

	// Window = 10. maxVal = 15. threshold = 15 - 10 = 5.
	// node3=3 < 5 → pruned. node1=15 ≥ 5 → kept. node2=8 ≥ 5 → kept.
	vc.prune()

	_, hasNode1 := vc["node1"]
	_, hasNode2 := vc["node2"]
	_, hasNode3 := vc["node3"]

	assert.True(t, hasNode1, "node1 should be kept")
	assert.True(t, hasNode2, "node2 should be kept")
	assert.False(t, hasNode3, "node3 should be pruned")
}

func TestVectorClock_Prune_NoOpWhenMaxLessThanWindow(t *testing.T) {
	vc := VectorClock{"node1": 5, "node2": 3}

	// maxVal=5, window=10. 5 <= 10 → no pruning
	vc.prune()

	assert.Len(t, vc, 2)
}

func TestVectorClock_SortedNodeIDs(t *testing.T) {
	vc := VectorClock{"node3": 1, "node1": 2, "node2": 3}
	ids := vc.SortedNodeIDs()

	assert.Equal(t, []string{"node1", "node2", "node3"}, ids)
}

func TestVectorClock_Serialize(t *testing.T) {
	vc := VectorClock{"node1": 5, "node2": 3}
	data, err := vc.Serialize()
	require.NoError(t, err)

	vc2, err := DeserializeVC(data)
	require.NoError(t, err)
	assert.Equal(t, vc, vc2)
}

func TestDeserializeVC_Empty(t *testing.T) {
	vc, err := DeserializeVC(nil)
	require.NoError(t, err)
	assert.Nil(t, vc)

	vc, err = DeserializeVC([]byte{})
	require.NoError(t, err)
	assert.Nil(t, vc)
}
