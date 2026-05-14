package matcher

import (
	"fmt"

	"github.com/w00199552/topo-match/internal/model"
)

// MatchResult holds the result of a successful match
type MatchResult struct {
	TestbedName string                // which testbed matched
	Mapping     map[string]string     // logic UUID -> physical UUID
	Nodes       map[string]*model.Node // physical UUID -> Node (matched nodes)
}

// Matcher performs subtree isomorphism matching
type Matcher struct{}

// NewMatcher creates a new Matcher
func NewMatcher() *Matcher {
	return &Matcher{}
}

// Match tries to find a matching subtree in the testbed for the given logic topology.
// Supports multiple root devices: all logic root devices must be matched.
// Returns nil if no match found.
func (m *Matcher) Match(logic *model.Topology, testbed *model.Topology, testbedName string) *MatchResult {
	if len(logic.RootDevices) == 0 {
		return nil
	}

	// For single root device, use simple matching
	if len(logic.RootDevices) == 1 {
		return m.matchSingleRoot(logic.RootDevices[0], logic, testbed, testbedName)
	}

	// For multiple root devices, try to match all of them
	return m.matchMultipleRoots(logic, testbed, testbedName)
}

// matchSingleRoot matches a single logic root device against the testbed
func (m *Matcher) matchSingleRoot(logicRoot *model.Node, logic *model.Topology, testbed *model.Topology, testbedName string) *MatchResult {
	allTestbedNodes := testbed.AllNodes()

	for _, candidate := range allTestbedNodes {
		mapping := make(map[string]string)
		used := make(map[string]bool)

		if m.matchNode(logicRoot, candidate, mapping, used) {
			if m.checkLinks(logic, testbed, mapping) {
				return m.buildResult(testbedName, mapping, testbed)
			}
		}
	}

	return nil
}

// matchMultipleRoots tries to match all logic root devices against the testbed
func (m *Matcher) matchMultipleRoots(logic *model.Topology, testbed *model.Topology, testbedName string) *MatchResult {
	allTestbedNodes := testbed.AllNodes()
	globalMapping := make(map[string]string)
	globalUsed := make(map[string]bool)

	if m.backtrackRoots(logic, 0, allTestbedNodes, globalMapping, globalUsed, testbed) {
		if m.checkLinks(logic, testbed, globalMapping) {
			return m.buildResult(testbedName, globalMapping, testbed)
		}
	}
	return nil
}

func (m *Matcher) backtrackRoots(
	logic *model.Topology,
	rootIdx int,
	candidates []*model.Node,
	globalMapping map[string]string,
	globalUsed map[string]bool,
	testbed *model.Topology,
) bool {
	if rootIdx == len(logic.RootDevices) {
		return true
	}

	savedMapping := copyMap(globalMapping)
	savedUsed := copyBoolMap(globalUsed)

	for _, candidate := range candidates {
		if globalUsed[candidate.UUID] {
			continue
		}

		if m.matchNode(logic.RootDevices[rootIdx], candidate, globalMapping, globalUsed) {
			if m.backtrackRoots(logic, rootIdx+1, candidates, globalMapping, globalUsed, testbed) {
				return true
			}
			// Backtrack
			restoreMap(globalMapping, savedMapping)
			restoreBoolMap(globalUsed, savedUsed)
		}
	}

	return false
}

// buildResult constructs a MatchResult from a mapping
func (m *Matcher) buildResult(testbedName string, mapping map[string]string, testbed *model.Topology) *MatchResult {
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

// matchNode tries to match a logic node against a physical candidate node.
func (m *Matcher) matchNode(logicNode *model.Node, physNode *model.Node, mapping map[string]string, used map[string]bool) bool {
	if logicNode.DeviceType != physNode.DeviceType {
		return false
	}

	if physNode.Status == "used" && !physNode.Share {
		return false
	}
	if physNode.Status != "idle" && physNode.Status != "used" {
		return false
	}

	if used[physNode.UUID] {
		return false
	}

	for key, logicVal := range logicNode.Properties {
		if logicVal != "" {
			physVal, ok := physNode.Properties[key]
			if !ok || physVal != logicVal {
				return false
			}
		}
	}

	mapping[logicNode.UUID] = physNode.UUID
	used[physNode.UUID] = true

	if len(logicNode.Children) != len(physNode.Children) {
		delete(mapping, logicNode.UUID)
		delete(used, physNode.UUID)
		return false
	}

	if len(logicNode.Children) == 0 {
		return true
	}

	return m.matchChildren(logicNode.Children, physNode.Children, mapping, used)
}

func (m *Matcher) matchChildren(logicChildren []*model.Node, physChildren []*model.Node, mapping map[string]string, used map[string]bool) bool {
	usedPhys := make(map[int]bool)
	return m.backtrackChildren(logicChildren, physChildren, 0, usedPhys, mapping, used)
}

func (m *Matcher) backtrackChildren(
	logicChildren []*model.Node,
	physChildren []*model.Node,
	idx int,
	usedPhys map[int]bool,
	mapping map[string]string,
	used map[string]bool,
) bool {
	if idx == len(logicChildren) {
		return true
	}

	savedMapping := copyMap(mapping)
	savedUsed := copyBoolMap(used)

	for j := 0; j < len(physChildren); j++ {
		if usedPhys[j] {
			continue
		}

		if m.matchNode(logicChildren[idx], physChildren[j], mapping, used) {
			usedPhys[j] = true
			if m.backtrackChildren(logicChildren, physChildren, idx+1, usedPhys, mapping, used) {
				return true
			}
			usedPhys[j] = false
			restoreMap(mapping, savedMapping)
			restoreBoolMap(used, savedUsed)
		}
	}

	return false
}

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

		found := false
		for _, physLink := range testbed.Links {
			if physLink.SourceUUID == physSourceUUID && physLink.TargetUUID == physTargetUUID {
				if physLink.Dir == logicLink.Dir {
					found = true
					break
				}
			}
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
