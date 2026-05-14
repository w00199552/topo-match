package allocator

import (
	"fmt"
	"sync"

	"github.com/w00199552/topo-match/internal/matcher"
	"github.com/w00199552/topo-match/internal/model"
)

// AllocResult holds the allocation result
type AllocResult struct {
	TestbedName string              `json:"testbed_name"`
	Mapping     map[string]string   `json:"mapping"` // logic UUID -> physical UUID
	Nodes       map[string]NodeInfo `json:"nodes"`   // physical UUID -> node info
}

// NodeInfo holds serializable node information
type NodeInfo struct {
	ObjName    string            `json:"objname"`
	DeviceType string            `json:"device_type"`
	Properties map[string]string `json:"properties"`
	Status     string            `json:"status"`
}

// Allocator manages allocation and freeing of physical environments
type Allocator struct {
	mu       sync.Mutex
	testbeds map[string]*model.Topology // testbed name -> topology
	matcher  *matcher.Matcher
}

// NewAllocator creates a new Allocator
func NewAllocator() *Allocator {
	return &Allocator{
		testbeds: make(map[string]*model.Topology),
		matcher:  matcher.NewMatcher(),
	}
}

// AddTestbed registers a testbed topology
func (a *Allocator) AddTestbed(name string, topo *model.Topology) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.testbeds[name] = topo
}

// RemoveTestbed removes a testbed
func (a *Allocator) RemoveTestbed(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.testbeds, name)
}

// Alloc tries to allocate a physical environment matching the logic topology
// across the given testbeds. Uses goroutines for concurrent matching.
func (a *Allocator) Alloc(logic *model.Topology, testbedNames []string) (*AllocResult, error) {
	a.mu.Lock()
	// Collect testbed topologies
	type tbEntry struct {
		name string
		topo *model.Topology
	}
	var entries []tbEntry
	for _, name := range testbedNames {
		name = trimSpace(name)
		topo, ok := a.testbeds[name]
		if !ok {
			a.mu.Unlock()
			return nil, fmt.Errorf("testbed %s not found", name)
		}
		entries = append(entries, tbEntry{name: name, topo: topo})
	}
	a.mu.Unlock()

	if len(entries) == 0 {
		return nil, fmt.Errorf("no testbeds provided")
	}

	// Concurrent matching across testbeds
	type matchOutput struct {
		result *matcher.MatchResult
		err    error
		index  int
	}

	ch := make(chan matchOutput, len(entries))
	for i, entry := range entries {
		go func(idx int, name string, topo *model.Topology) {
			result := a.matcher.Match(logic, topo, name)
			ch <- matchOutput{result: result, index: idx}
		}(i, entry.name, entry.topo)
	}

	// Collect results - return first successful match
	var firstResult *matcher.MatchResult
	for i := 0; i < len(entries); i++ {
		output := <-ch
		if output.result != nil && firstResult == nil {
			firstResult = output.result
		}
	}

	if firstResult == nil {
		return nil, fmt.Errorf("无空闲的测试床物理环境可用")
	}

	// Mark matched nodes as used
	a.mu.Lock()
	topo := a.testbeds[firstResult.TestbedName]
	for physUUID := range firstResult.Nodes {
		node := topo.FindNodeByUUID(physUUID)
		if node != nil {
			node.Status = "used"
		}
	}
	a.mu.Unlock()

	return buildAllocResult(firstResult, topo), nil
}

// Free releases a previously allocated physical environment
func (a *Allocator) Free(allocResult *AllocResult) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	topo, ok := a.testbeds[allocResult.TestbedName]
	if !ok {
		return fmt.Errorf("testbed %s not found", allocResult.TestbedName)
	}

	for physUUID := range allocResult.Nodes {
		node := topo.FindNodeByUUID(physUUID)
		if node != nil {
			node.Status = "idle"
		}
	}

	return nil
}

// ListTestbeds returns the names of all registered testbeds
func (a *Allocator) ListTestbeds() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	names := make([]string, 0, len(a.testbeds))
	for name := range a.testbeds {
		names = append(names, name)
	}
	return names
}

// GetTestbedStatus returns the status of all nodes in a testbed
func (a *Allocator) GetTestbedStatus(name string) (map[string]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	topo, ok := a.testbeds[name]
	if !ok {
		return nil, fmt.Errorf("testbed %s not found", name)
	}

	status := make(map[string]string)
	for _, node := range topo.AllNodes() {
		status[node.UUID] = node.Status
	}
	return status, nil
}

func buildAllocResult(mr *matcher.MatchResult, topo *model.Topology) *AllocResult {
	result := &AllocResult{
		TestbedName: mr.TestbedName,
		Mapping:     mr.Mapping,
		Nodes:       make(map[string]NodeInfo),
	}

	for physUUID, node := range mr.Nodes {
		result.Nodes[physUUID] = NodeInfo{
			ObjName:    node.ObjName,
			DeviceType: node.DeviceType,
			Properties: node.Properties,
			Status:     "used",
		}
	}

	return result
}

func trimSpace(s string) string {
	// Simple trim - avoid importing strings for just this
	result := s
	for len(result) > 0 && (result[0] == ' ' || result[0] == '\t') {
		result = result[1:]
	}
	for len(result) > 0 && (result[len(result)-1] == ' ' || result[len(result)-1] == '\t') {
		result = result[:len(result)-1]
	}
	return result
}
