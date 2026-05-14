package allocator

import (
	"testing"

	"github.com/w00199552/topo-match/internal/model"
)

func buildSimpleLogic() *model.Topology {
	cna1 := &model.Node{UUID: "l3", ObjName: "cna1", DeviceType: "cna", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	cna2 := &model.Node{UUID: "l4", ObjName: "cna2", DeviceType: "cna", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	cluster1 := &model.Node{UUID: "l2", ObjName: "cluster1", DeviceType: "cluster", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{cna1, cna2}}
	fc1 := &model.Node{UUID: "l1", ObjName: "fc1", DeviceType: "fc", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{cluster1}}
	return &model.Topology{
		RootDevices: []*model.Node{fc1},
		Links:       []*model.Link{{SourceUUID: "l3", TargetUUID: "l4", Dir: model.LinkDirOneWay}},
	}
}

func buildSimpleTestbed() *model.Topology {
	cna10 := &model.Node{UUID: "p3", ObjName: "cna10", DeviceType: "cna", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	cna20 := &model.Node{UUID: "p4", ObjName: "cna20", DeviceType: "cna", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	cluster10 := &model.Node{UUID: "p2", ObjName: "cluster10", DeviceType: "cluster", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{cna10, cna20}}
	fc10 := &model.Node{UUID: "p1", ObjName: "fc10", DeviceType: "fc", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{cluster10}}

	cna30 := &model.Node{UUID: "p7", ObjName: "cna30", DeviceType: "cna", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	cna40 := &model.Node{UUID: "p8", ObjName: "cna40", DeviceType: "cna", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	cluster20 := &model.Node{UUID: "p6", ObjName: "cluster20", DeviceType: "cluster", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{cna30, cna40}}
	fc20 := &model.Node{UUID: "p5", ObjName: "fc20", DeviceType: "fc", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{cluster20}}

	return &model.Topology{
		RootDevices: []*model.Node{fc10, fc20},
		Links: []*model.Link{
			{SourceUUID: "p3", TargetUUID: "p4", Dir: model.LinkDirTwoWay},
			{SourceUUID: "p7", TargetUUID: "p8", Dir: model.LinkDirOneWay},
		},
	}
}

func TestAlloc_Free_Basic(t *testing.T) {
	logic := buildSimpleLogic()
	testbed := buildSimpleTestbed()

	alloc := NewAllocator()
	alloc.AddTestbed("tb1", testbed)

	result, err := alloc.Alloc(logic, []string{"tb1"})
	if err != nil {
		t.Fatalf("alloc failed: %v", err)
	}

	// Verify nodes are marked used
	for _, node := range testbed.AllNodes() {
		if _, ok := result.Nodes[node.UUID]; ok {
			if node.Status != "used" {
				t.Errorf("node %s should be used, got %s", node.UUID, node.Status)
			}
			if node.RefCount != 1 {
				t.Errorf("node %s should have RefCount=1, got %d", node.UUID, node.RefCount)
			}
		}
	}

	// Free
	if err := alloc.Free(result); err != nil {
		t.Fatalf("free failed: %v", err)
	}

	// Verify nodes are idle again
	for _, node := range testbed.AllNodes() {
		if _, ok := result.Nodes[node.UUID]; ok {
			if node.Status != "idle" {
				t.Errorf("node %s should be idle after free, got %s", node.UUID, node.Status)
			}
			if node.RefCount != 0 {
				t.Errorf("node %s should have RefCount=0 after free, got %d", node.UUID, node.RefCount)
			}
		}
	}
}

func TestAlloc_ShareRefCount(t *testing.T) {
	logic := buildSimpleLogic()
	testbed := buildSimpleTestbed()

	// Make fc20 branch shareable
	for _, uuid := range []string{"p5", "p6", "p7", "p8"} {
		node := testbed.FindNodeByUUID(uuid)
		node.Share = true
	}

	alloc := NewAllocator()
	alloc.AddTestbed("tb1", testbed)

	// First alloc
	result1, err := alloc.Alloc(logic, []string{"tb1"})
	if err != nil {
		t.Fatalf("first alloc failed: %v", err)
	}

	// Second alloc (should succeed on shared nodes)
	result2, err := alloc.Alloc(logic, []string{"tb1"})
	if err != nil {
		t.Fatalf("second alloc failed: %v", err)
	}

	// Verify RefCount=2 on shared nodes
	for _, uuid := range []string{"p5", "p6", "p7", "p8"} {
		node := testbed.FindNodeByUUID(uuid)
		if node.RefCount != 2 {
			t.Errorf("shared node %s should have RefCount=2, got %d", uuid, node.RefCount)
		}
	}

	// Free first alloc
	if err := alloc.Free(result1); err != nil {
		t.Fatalf("first free failed: %v", err)
	}

	// Shared nodes should still be used (RefCount=1)
	for _, uuid := range []string{"p5", "p6", "p7", "p8"} {
		node := testbed.FindNodeByUUID(uuid)
		if node.Status != "used" {
			t.Errorf("shared node %s should still be used after first free, got %s", uuid, node.Status)
		}
		if node.RefCount != 1 {
			t.Errorf("shared node %s should have RefCount=1 after first free, got %d", uuid, node.RefCount)
		}
	}

	// Free second alloc
	if err := alloc.Free(result2); err != nil {
		t.Fatalf("second free failed: %v", err)
	}

	// Now shared nodes should be idle (RefCount=0)
	for _, uuid := range []string{"p5", "p6", "p7", "p8"} {
		node := testbed.FindNodeByUUID(uuid)
		if node.Status != "idle" {
			t.Errorf("shared node %s should be idle after second free, got %s", uuid, node.Status)
		}
		if node.RefCount != 0 {
			t.Errorf("shared node %s should have RefCount=0 after second free, got %d", uuid, node.RefCount)
		}
	}
}

func TestAlloc_NoMatch(t *testing.T) {
	logic := &model.Topology{
		RootDevices: []*model.Node{
			{UUID: "l1", ObjName: "router1", DeviceType: "router", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}},
		},
		Links: []*model.Link{},
	}
	testbed := buildSimpleTestbed()

	alloc := NewAllocator()
	alloc.AddTestbed("tb1", testbed)

	_, err := alloc.Alloc(logic, []string{"tb1"})
	if err == nil {
		t.Error("expected error for no match, got nil")
	}
}

func TestAlloc_TestbedNotFound(t *testing.T) {
	logic := buildSimpleLogic()
	alloc := NewAllocator()

	_, err := alloc.Alloc(logic, []string{"nonexistent"})
	if err == nil {
		t.Error("expected error for missing testbed, got nil")
	}
}
