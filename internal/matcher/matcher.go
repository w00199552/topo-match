package matcher

import (
	"fmt"

	"github.com/w00199552/topo-match/internal/model"
)

// MatchResult holds the result of a successful match
type MatchResult struct {
	TestbedName string               // which testbed matched
	Mapping     map[string]string    // logic UUID -> physical UUID
	Nodes       map[string]*model.Node // physical UUID -> Node (matched nodes)
}

// Matcher performs subtree isomorphism matching
type Matcher struct{}

// NewMatcher creates a new Matcher
func NewMatcher() *Matcher {
	return &Matcher{}
}

// Match tries to find a matching subtree in the testbed for the given logic topology.
// Returns nil if no match found.
func (m *Matcher) Match(logic *model.Topology, testbed *model.Topology, testbedName string) *MatchResult {
	if len(logic.RootDevices) == 0 {
		return nil
	}

	// Collect all nodes in testbed as potential root candidates
	allTestbedNodes := testbed.AllNodes()

	// Try matching starting from each testbed node
	for _, candidate := range allTestbedNodes {
		mapping := make(map[string]string) // logicUUID -> physicalUUID
		used := make(map[string]bool)      // physicalUUID -> used in this match

		if m.matchNode(logic.RootDevices[0], candidate, mapping, used) {
			// Check link constraints
			if m.checkLinks(logic, testbed, mapping) {
				// Build result
				result := &MatchResult{
					TestbedName: testbedName,
					Mapping:     mapping,
					Nodes:       make(map[string]*model.Node),
				}
				for _, physUUID := range mapping {
					node := testbed.FindNodeByUUID(physUUID)
					if node != nil {
						result.Nodes[physUUID] = node
					}
				}
				return result
			}
		}
	}

	return nil
}

// matchNode tries to match a logic node against a physical candidate node.
// It recursively matches children (unordered).
func (m *Matcher) matchNode(logicNode *model.Node, physNode *model.Node, mapping map[string]string, used map[string]bool) bool {
	// Check device_type
	if logicNode.DeviceType != physNode.DeviceType {
		return false
	}

	// Check status (must be idle)
	if physNode.Status != "idle" {
		return false
	}

	// Check if physical node already used in this match
	if used[physNode.UUID] {
		return false
	}

	// Check properties (logic property non-empty -> must match exactly)
	for key, logicVal := range logicNode.Properties {
		if logicVal != "" {
			physVal, ok := physNode.Properties[key]
			if !ok || physVal != logicVal {
				return false
			}
		}
	}

	// Check objname (if logic node has one, must match)
	if logicNode.ObjName != "" && physNode.ObjName != "" {
		if logicNode.ObjName != physNode.ObjName {
			// objname is just a label, not a hard constraint unless we decide otherwise
			// For now, objname does NOT need to match (it's just a display name)
		}
	}

	// Record mapping
	mapping[logicNode.UUID] = physNode.UUID
	used[physNode.UUID] = true

	// Check child count
	if len(logicNode.Children) != len(physNode.Children) {
		delete(mapping, logicNode.UUID)
		delete(used, physNode.UUID)
		return false
	}

	if len(logicNode.Children) == 0 {
		return true
	}

	// Try to match children (unordered) using backtracking
	return m.matchChildren(logicNode.Children, physNode.Children, mapping, used)
}

// matchChildren tries to find a bijection between logic children and physical children.
// Since children are unordered, we try all permutations via backtracking.
func (m *Matcher) matchChildren(logicChildren []*model.Node, physChildren []*model.Node, mapping map[string]string, used map[string]bool) bool {
	n := len(logicChildren)
	assigned := make([]int, 0, n) // which phys child index is assigned to each logic child
	usedPhys := make(map[int]bool)

	return m.backtrackChildren(logicChildren, physChildren, 0, assigned, usedPhys, mapping, used)
}

func (m *Matcher) backtrackChildren(
	logicChildren []*model.Node,
	physChildren []*model.Node,
	idx int,
	assigned []int,
	usedPhys map[int]bool,
	mapping map[string]string,
	used map[string]bool,
) bool {
	if idx == len(logicChildren) {
		return true
	}

	// Save mapping state for backtracking
	savedMapping := copyMap(mapping)
	savedUsed := copyBoolMap(used)

	for j := 0; j < len(physChildren); j++ {
		if usedPhys[j] {
			continue
		}

		// Try matching logicChildren[idx] with physChildren[j]
		if m.matchNode(logicChildren[idx], physChildren[j], mapping, used) {
			usedPhys[j] = true
			if m.backtrackChildren(logicChildren, physChildren, idx+1, assigned, usedPhys, mapping, used) {
				return true
			}
			// Backtrack
			usedPhys[j] = false
			// Restore mapping and used
			restoreMap(mapping, savedMapping)
			restoreBoolMap(used, savedUsed)
		}
	}

	return false
}

// checkLinks verifies that all links in the logic topology have corresponding links in the testbed
func (m *Matcher) checkLinks(logic *model.Topology, testbed *model.Topology, mapping map[string]string) bool {
	for _, logicLink := range logic.Links {
		physSourceUUID, ok := mapping[logicLink.SourceUUID]
		if !ok {
			return false
		}
		physTargetUUID, ok := mapping[logicLink.TargetUUID]
		if !ok {
			return false
		}

		// Find matching link in testbed
		found := false
		for _, physLink := range testbed.Links {
			// Check forward direction
			if physLink.SourceUUID == physSourceUUID && physLink.TargetUUID == physTargetUUID {
				if physLink.Dir == logicLink.Dir {
					found = true
					break
				}
			}
			// For bidirectional links, also check reverse
			if logicLink.Dir == model.LinkDirTwoWay && physLink.Dir == model.LinkDirTwoWay {
				if physLink.SourceUUID == physTargetUUID && physLink.TargetUUID == physSourceUUID {
					found = true
					break
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func copyMap(m map[string]string) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func restoreMap(m map[string]string, saved map[string]string) {
	for k := range m {
		if _, ok := saved[k]; !ok {
			delete(m, k)
		}
	}
	for k, v := range saved {
		m[k] = v
	}
}

func copyBoolMap(m map[string]bool) map[string]bool {
	result := make(map[string]bool, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func restoreBoolMap(m map[string]bool, saved map[string]bool) {
	for k := range m {
		if _, ok := saved[k]; !ok {
			delete(m, k)
		}
	}
	for k, v := range saved {
		m[k] = v
	}
}

// FormatMapping returns a human-readable string of the mapping
func (r *MatchResult) FormatMapping(logic *model.Topology, testbed *model.Topology) string {
	s := fmt.Sprintf("Matched on testbed: %s\n", r.TestbedName)
	s += "Logic Node -> Physical Node:\n"
	for logicUUID, physUUID := range r.Mapping {
		logicNode := logic.FindNodeByUUID(logicUUID)
		physNode := testbed.FindNodeByUUID(physUUID)
		if logicNode != nil && physNode != nil {
			s += fmt.Sprintf("  %s(%s) -> %s(%s)\n", logicNode.ObjName, logicNode.DeviceType, physNode.ObjName, physNode.DeviceType)
		}
	}
	return s
}
