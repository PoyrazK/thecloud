// Package node implements storage node services.
package node

import (
	"encoding/json"
	"sort"
)

// defaultVCWindow is the sliding window size for pruning old VC entries.
// Entries older than (maxCounter - window) are removed after each increment.
const defaultVCWindow = 10

// VectorClock tracks causal ordering across nodes.
// Key: nodeID, Value: counter for that node.
type VectorClock map[string]uint64

// NewVectorClock creates an empty VC.
func NewVectorClock() VectorClock {
	return make(VectorClock)
}

// Increment increments the counter for the given node and returns the new value.
func (vc VectorClock) Increment(nodeID string) uint64 {
	vc[nodeID]++
	return vc[nodeID]
}

// maxCounter returns the maximum counter value in the VC.
func (vc VectorClock) maxCounter() uint64 {
	var max uint64
	for _, c := range vc {
		if c > max {
			max = c
		}
	}
	return max
}

// prune removes entries that are older than windowSize from the max.
func (vc VectorClock) prune(windowSize uint64) {
	if windowSize == 0 {
		return
	}
	maxVal := vc.maxCounter()
	if maxVal <= windowSize {
		return // Nothing to prune when maxVal <= windowSize
	}
	threshold := maxVal - windowSize
	for nodeID, c := range vc {
		if c < threshold {
			delete(vc, nodeID)
		}
	}
}

// Compare compares two vector clocks and returns:
//   -1 if a < b  (a happened-before b, b dominates a)
//    1 if a > b  (a happened-after b, a dominates b)
//    0 if a == b (identical state)
//    2 if concurrent (neither dominates the other)
func Compare(a, b VectorClock) int {
	allALessOrEqual := true
	allBLessOrEqual := true
	strictlyLess := false

	// Check all keys in a
	for nodeID, counterA := range a {
		counterB, exists := b[nodeID]
		if !exists {
			if counterA > 0 {
				allALessOrEqual = false
			}
		} else {
			if counterA < counterB {
				allALessOrEqual = false
			}
			if counterA > counterB {
				allBLessOrEqual = false
				strictlyLess = true
			}
		}
	}

	// Check all keys in b that aren't in a
	for nodeID, counterB := range b {
		if _, exists := a[nodeID]; !exists {
			if counterB > 0 {
				allBLessOrEqual = false
			}
		}
	}

	if allALessOrEqual && allBLessOrEqual && !strictlyLess {
		return 0 // Equal
	}
	if allALessOrEqual && (strictlyLess || !allBLessOrEqual) {
		return -1 // a < b (a happened-before b)
	}
	if allBLessOrEqual {
		return 1 // a > b (a happened-after b)
	}
	return 2 // Concurrent
}

// IsNewerThan returns true if vc is strictly newer than other (vc > other).
func (vc VectorClock) IsNewerThan(other VectorClock) bool {
	return Compare(vc, other) == 1
}

// IsOlderThan returns true if vc is strictly older than other (vc < other).
func (vc VectorClock) IsOlderThan(other VectorClock) bool {
	return Compare(vc, other) == -1
}

// IsConcurrentWith returns true if vc and other are concurrent.
func (vc VectorClock) IsConcurrentWith(other VectorClock) bool {
	return Compare(vc, other) == 2
}

// Merge takes the maximum of each counter from other.
func (vc VectorClock) Merge(other VectorClock) {
	for nodeID, counter := range other {
		if counter > vc[nodeID] {
			vc[nodeID] = counter
		}
	}
}

// Sum returns the sum of all counters (deterministic tiebreaker).
func (vc VectorClock) Sum() uint64 {
	var sum uint64
	for _, c := range vc {
		sum += c
	}
	return sum
}

// SortedNodeIDs returns node IDs sorted lexicographically (deterministic tiebreaker).
func (vc VectorClock) SortedNodeIDs() []string {
	ids := make([]string, 0, len(vc))
	for id := range vc {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Serialize encodes the VC as JSON bytes.
func (vc VectorClock) Serialize() ([]byte, error) {
	return json.Marshal(map[string]uint64(vc))
}

// DeserializeVC decodes a VC from JSON bytes.
func DeserializeVC(data []byte) (VectorClock, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var vc VectorClock
	if err := json.Unmarshal(data, &vc); err != nil {
		return nil, err
	}
	return vc, nil
}