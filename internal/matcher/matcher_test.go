package matcher

import (
	"testing"

	"github.com/w00199552/topo-match/internal/model"
)

// Build the example logic topology:
// fc1(fc) -> cluster1(cluster) -> [cna1(cna), cna2(cna)]
// Link: cna1 -> cna2 (单向)
func buildLogicTopo() *model.Topology {
	cna1 := &model.Node{UUID: "l3", ObjName: "cna1", DeviceType: "cna", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	cna2 := &model.Node{UUID: "l4", ObjName: "cna2", DeviceType: "cna", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	cluster1 := &model.Node{UUID: "l2", ObjName: "cluster1", DeviceType: "cluster", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{cna1, cna2}}
	fc1 := &model.Node{UUID: "l1", ObjName: "fc1", DeviceType: "fc", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{cluster1}}

	return &model.Topology{
		RootDevices: []*model.Node{fc1},
		Links: []*model.Link{
			{SourceUUID: "l3", TargetUUID: "l4", Dir: model.LinkDirOneWay},
		},
	}
}

func buildTestbed() *model.Topology {
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

func TestMatch_OneWayLink(t *testing.T) {
	logic := buildLogicTopo()
	testbed := buildTestbed()
	m := NewMatcher()

	result := m.Match(logic, testbed, "testbed1")
	if result == nil {
		t.Fatal("expected match, got nil")
	}
	if result.Mapping["l1"] != "p5" {
		t.Errorf("expected l1 -> p5, got l1 -> %s", result.Mapping["l1"])
	}
}

func TestMatch_TwoWayLinkNotMatchOneWay(t *testing.T) {
	cna1 := &model.Node{UUID: "l3", ObjName: "cna1", DeviceType: "cna", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	cna2 := &model.Node{UUID: "l4", ObjName: "cna2", DeviceType: "cna", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	cluster1 := &model.Node{UUID: "l2", ObjName: "cluster1", DeviceType: "cluster", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{cna1, cna2}}
	fc1 := &model.Node{UUID: "l1", ObjName: "fc1", DeviceType: "fc", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{cluster1}}

	logic := &model.Topology{
		RootDevices: []*model.Node{fc1},
		Links: []*model.Link{
			{SourceUUID: "l3", TargetUUID: "l4", Dir: model.LinkDirTwoWay},
		},
	}
	testbed := buildTestbed()
	m := NewMatcher()

	result := m.Match(logic, testbed, "testbed1")
	if result == nil {
		t.Fatal("expected match on fc10 branch (two-way link), got nil")
	}
	if result.Mapping["l1"] != "p1" {
		t.Errorf("expected l1 -> p1, got l1 -> %s", result.Mapping["l1"])
	}
}

func TestMatch_NoMatch(t *testing.T) {
	logic := &model.Topology{
		RootDevices: []*model.Node{
			{UUID: "l1", ObjName: "router1", DeviceType: "router", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}},
		},
		Links: []*model.Link{},
	}
	testbed := buildTestbed()
	m := NewMatcher()

	result := m.Match(logic, testbed, "testbed1")
	if result != nil {
		t.Error("expected no match, got a result")
	}
}

func TestMatch_PropertyConstraint(t *testing.T) {
	logic := &model.Topology{
		RootDevices: []*model.Node{
			{
				UUID: "l1", ObjName: "fc1", DeviceType: "fc", Status: "idle",
				Properties: map[string]string{"version": "2"},
				Children:   []*model.Node{},
			},
		},
		Links: []*model.Link{},
	}

	fc10 := &model.Node{
		UUID: "p1", ObjName: "fc10", DeviceType: "fc", Status: "idle",
		Properties: map[string]string{"version": "1"},
		Children:   []*model.Node{},
	}
	testbed := &model.Topology{
		RootDevices: []*model.Node{fc10},
		Links:       []*model.Link{},
	}

	m := NewMatcher()
	result := m.Match(logic, testbed, "testbed1")
	if result != nil {
		t.Error("expected no match due to property mismatch, got a result")
	}
}

func TestMatch_UsedNodeSkipped(t *testing.T) {
	logic := buildLogicTopo()
	testbed := buildTestbed()

	fc20 := testbed.FindNodeByUUID("p5")
	fc20.Status = "used"

	m := NewMatcher()
	result := m.Match(logic, testbed, "testbed1")
	if result != nil {
		t.Error("expected no match (fc20 used, fc10 has wrong link direction), got a result")
	}
}

func TestMatch_ShareNodeCanBeReused(t *testing.T) {
	logic := buildLogicTopo()
	testbed := buildTestbed()

	fc20 := testbed.FindNodeByUUID("p5")
	fc20.Status = "used"
	fc20.Share = true
	cluster20 := testbed.FindNodeByUUID("p6")
	cluster20.Status = "used"
	cluster20.Share = true
	cna30 := testbed.FindNodeByUUID("p7")
	cna30.Status = "used"
	cna30.Share = true
	cna40 := testbed.FindNodeByUUID("p8")
	cna40.Status = "used"
	cna40.Share = true

	m := NewMatcher()
	result := m.Match(logic, testbed, "testbed1")
	if result == nil {
		t.Fatal("expected match on shared fc20 branch, got nil")
	}
	if result.Mapping["l1"] != "p5" {
		t.Errorf("expected l1 -> p5, got l1 -> %s", result.Mapping["l1"])
	}
}

func TestMatch_UsedNodeNotShareable(t *testing.T) {
	logic := buildLogicTopo()
	testbed := buildTestbed()

	fc20 := testbed.FindNodeByUUID("p5")
	fc20.Status = "used"
	fc20.Share = false

	m := NewMatcher()
	result := m.Match(logic, testbed, "testbed1")
	if result != nil {
		t.Error("expected no match (fc20 used and not shareable), got a result")
	}
}

func TestMatch_PropertyEmptySkipped(t *testing.T) {
	logic := &model.Topology{
		RootDevices: []*model.Node{
			{
				UUID: "l1", ObjName: "fc1", DeviceType: "fc", Status: "idle",
				Properties: map[string]string{"version": ""},
				Children:   []*model.Node{},
			},
		},
		Links: []*model.Link{},
	}

	fc10 := &model.Node{
		UUID: "p1", ObjName: "fc10", DeviceType: "fc", Status: "idle",
		Properties: map[string]string{"version": "1"},
		Children:   []*model.Node{},
	}
	testbed := &model.Topology{
		RootDevices: []*model.Node{fc10},
		Links:       []*model.Link{},
	}

	m := NewMatcher()
	result := m.Match(logic, testbed, "testbed1")
	if result == nil {
		t.Fatal("expected match (empty property should not constrain), got nil")
	}
}

func TestMatch_MultipleRootDevices(t *testing.T) {
	// Logic: two separate fc nodes (no children, no links)
	fc1 := &model.Node{UUID: "l1", ObjName: "fc1", DeviceType: "fc", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	fc2 := &model.Node{UUID: "l2", ObjName: "fc2", DeviceType: "fc", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}

	logic := &model.Topology{
		RootDevices: []*model.Node{fc1, fc2},
		Links:       []*model.Link{},
	}

	// Testbed: two fc nodes
	pfc1 := &model.Node{UUID: "p1", ObjName: "fc10", DeviceType: "fc", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	pfc2 := &model.Node{UUID: "p2", ObjName: "fc20", DeviceType: "fc", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}

	testbed := &model.Topology{
		RootDevices: []*model.Node{pfc1, pfc2},
		Links:       []*model.Link{},
	}

	m := NewMatcher()
	result := m.Match(logic, testbed, "testbed1")
	if result == nil {
		t.Fatal("expected match with multiple roots, got nil")
	}
	if len(result.Mapping) != 2 {
		t.Errorf("expected 2 mappings, got %d", len(result.Mapping))
	}
}

func TestMatch_MultipleRootsNotEnoughPhysical(t *testing.T) {
	// Logic: two fc nodes
	fc1 := &model.Node{UUID: "l1", ObjName: "fc1", DeviceType: "fc", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}
	fc2 := &model.Node{UUID: "l2", ObjName: "fc2", DeviceType: "fc", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}

	logic := &model.Topology{
		RootDevices: []*model.Node{fc1, fc2},
		Links:       []*model.Link{},
	}

	// Testbed: only one fc node
	pfc1 := &model.Node{UUID: "p1", ObjName: "fc10", DeviceType: "fc", Status: "idle", Properties: map[string]string{}, Children: []*model.Node{}}

	testbed := &model.Topology{
		RootDevices: []*model.Node{pfc1},
		Links:       []*model.Link{},
	}

	m := NewMatcher()
	result := m.Match(logic, testbed, "testbed1")
	if result != nil {
		t.Error("expected no match (not enough physical nodes), got a result")
	}
}
