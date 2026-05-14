package model

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
)

// XML parsing structures

type xmlTopology struct {
	XMLName xml.Name   `xml:"logic_topology"` // also matches <testbed> via custom logic
	Devices xmlDevices `xml:"devices"`
	Links   []xmlLink  `xml:"links>link"`
}

type xmlTestbed struct {
	XMLName xml.Name   `xml:"testbed"`
	Devices xmlDevices `xml:"devices"`
	Links   []xmlLink  `xml:"links>link"`
}

type xmlDevices struct {
	Devices []xmlDevice `xml:"device"`
}

type xmlDevice struct {
	Properties xmlProperties `xml:"properties"`
	DeviceType string        `xml:"devicetype"`
	Children   xmlDevices    `xml:"devices"`
	Children2  xmlDevices    `xml:"deivces"` // handle common typo
}

type xmlProperties struct {
	Props []xmlProperty `xml:"property"`
}

type xmlProperty struct {
	XMLName xml.Name `xml:"property"`
	ID      string   `xml:"id,attr,omitempty"`
	Name    string   `xml:"name,attr,omitempty"`
	Value   string   `xml:",chardata"`
}

type xmlLink struct {
	Source string `xml:"source,attr"`
	Target string `xml:"target,attr"`
	Dir    string `xml:"dir,attr"`
}

// ParseXMLFile reads and parses a topology XML file
func ParseXMLFile(path string) (*Topology, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file %s: %w", path, err)
	}
	defer f.Close()
	return ParseXML(f)
}

// ParseXML reads and parses a topology from an io.Reader
func ParseXML(r io.Reader) (*Topology, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read xml: %w", err)
	}

	topo := &Topology{
		RootDevices: make([]*Node, 0),
		Links:       make([]*Link, 0),
	}

	// Try logic_topology format first
	var xt xmlTopology
	if err := xml.Unmarshal(data, &xt); err == nil && len(xt.Devices.Devices) > 0 {
		for _, xd := range xt.Devices.Devices {
			topo.RootDevices = append(topo.RootDevices, parseXMLDevice(xd))
		}
		for _, xl := range xt.Links {
			dir := LinkDirTwoWay
			if xl.Dir != "" {
				dir = LinkDir(xl.Dir)
			}
			topo.Links = append(topo.Links, &Link{
				SourceUUID: xl.Source,
				TargetUUID: xl.Target,
				Dir:        dir,
			})
		}
		return topo, nil
	}

	// Try testbed format
	var xtb xmlTestbed
	if err := xml.Unmarshal(data, &xtb); err == nil && len(xtb.Devices.Devices) > 0 {
		for _, xd := range xtb.Devices.Devices {
			topo.RootDevices = append(topo.RootDevices, parseXMLDevice(xd))
		}
		for _, xl := range xtb.Links {
			dir := LinkDirTwoWay
			if xl.Dir != "" {
				dir = LinkDir(xl.Dir)
			}
			topo.Links = append(topo.Links, &Link{
				SourceUUID: xl.Source,
				TargetUUID: xl.Target,
				Dir:        dir,
			})
		}
		return topo, nil
	}

	return nil, fmt.Errorf("xml does not match logic_topology or testbed format")
}

func parseXMLDevice(xd xmlDevice) *Node {
	node := NewNode()

	// Parse properties
	for _, xp := range xd.Properties.Props {
		if xp.ID != "" {
			node.UUID = xp.ID
		}
		if xp.Name != "" {
			switch xp.Name {
			case "objname":
				node.ObjName = xp.Value
			default:
				node.Properties[xp.Name] = xp.Value
			}
		}
	}

	node.DeviceType = xd.DeviceType

	// Parse children (handle both correct and typo spelling)
	children := xd.Children.Devices
	if len(children) == 0 {
		children = xd.Children2.Devices
	}
	for _, child := range children {
		node.Children = append(node.Children, parseXMLDevice(child))
	}

	return node
}
