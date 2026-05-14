package model

// LinkDir represents the direction of a link
type LinkDir string

const (
	LinkDirOneWay LinkDir = "单向"
	LinkDirTwoWay LinkDir = "双向"
)

// Node represents a device node in a topology tree
type Node struct {
	UUID       string            // internal reference id for link resolution
	ObjName    string            // node object name
	DeviceType string            // device type (required)
	Children   []*Node           // child nodes (unordered)
	Properties map[string]string // dynamic properties
	Status     string            // "idle" or "used" (only for testbed nodes)
	Share      bool              // allow shared allocation
	RefCount   int               // reference count for share nodes (internal, not serialized)
}

// Link represents a directed connection between two nodes
type Link struct {
	SourceUUID string  // source node UUID
	TargetUUID string  // target node UUID
	Dir        LinkDir // "单向" or "双向", default "双向"
}

// Topology represents a complete topology tree with links
type Topology struct {
	RootDevices []*Node // root-level devices
	Links       []*Link // inter-node links
}

// NewNode creates a new Node with default values
func NewNode() *Node {
	return &Node{
		Children:   make([]*Node, 0),
		Properties: make(map[string]string),
		Status:     "idle",
	}
}

// AllNodes returns all nodes in the tree (breadth-first traversal)
func (t *Topology) AllNodes() []*Node {
	var result []*Node
	queue := make([]*Node, len(t.RootDevices))
	copy(queue, t.RootDevices)
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		result = append(result, n)
		queue = append(queue, n.Children...)
	}
	return result
}

// FindNodeByUUID finds a node by its UUID
func (t *Topology) FindNodeByUUID(uuid string) *Node {
	for _, n := range t.AllNodes() {
		if n.UUID == uuid {
			return n
		}
	}
	return nil
}
