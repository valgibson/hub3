package ead

import (
	"encoding/xml"
	"sync/atomic"
)

// NewNodeList converts the Archival Description Level to a full NodeList
func (dsc *Cdsc) NewNodeList() (*NodeList, uint64, error) {
	return dsc.newSparseNodeList(false)
}

// NewNodeList converts the Archival Description Level to a Sparse NodeList
func (dsc *Cdsc) NewSparseNodeList() (*NodeList, uint64, error) {
	return dsc.newSparseNodeList(true)
}

// newSparseNodeList converts the Archival Description Level to a Nodelist
// Nodelist is an optimized lossless Protocol Buffer container.
func (dsc *Cdsc) newSparseNodeList(sparse bool) (*NodeList, uint64, error) {
	nl := &NodeList{}
	nl.Type = dsc.Attrtype
	for _, label := range dsc.Chead {
		nl.Label = append(nl.Label, label.Head)
	}

	var counter uint64

	for _, nn := range dsc.Nested {
		node, err := nn.NewNode(&counter, sparse)
		if err != nil {
			return nil, 0, err
		}
		nl.Nodes = append(nl.Nodes, node)
	}
	return nl, atomic.LoadUint64(&counter), nil
}

// NewNodeID converts a unitid field from the EAD did to a NodeID
func (ui *Cunitid) NewNodeID() (*NodeID, error) {
	id := &NodeID{
		ID:       ui.ID,
		TypeID:   ui.Attridentifier,
		Type:     ui.Attrtype,
		Audience: ui.Attraudience,
	}
	return id, nil
}

// NewNodeIDs extract Unit Identifiers from the EAD did
func (cdid *Cdid) NewNodeIDs(sparse bool) ([]*NodeID, string, error) {
	ids := []*NodeID{}
	var invertoryNumber string
	for _, unitid := range cdid.Cunitid {
		id, err := unitid.NewNodeID()
		if err != nil {
			return nil, "", err
		}
		switch id.GetType() {
		case "ABS", "series_code", "":
			invertoryNumber = id.GetID()
		}
		if !sparse {
			ids = append(ids, id)
		}
	}
	return ids, invertoryNumber, nil
}

// NewNodeDate extract date infomation frme the EAD unitdate
func (date *Cunitdate) NewNodeDate() (*NodeDate, error) {
	nDate := &NodeDate{
		Calendar: date.Attrcalendar,
		Era:      date.Attrera,
		Normal:   date.Attrnormal,
		Label:    date.Date,
	}
	return nDate, nil
}

// NewHeader creates an Archival Header
func (cdid *Cdid) NewHeader(sparse bool) (*Header, error) {
	header := &Header{}
	if cdid.Cphysdesc != nil && !sparse {
		header.Physdesc = cdid.Cphysdesc.PhyscDesc
	}

	for _, label := range cdid.Cunittitle {
		if len(label.Cunitdate) != 0 {
			header.DateAsLabel = true
			for _, date := range label.Cunitdate {
				nodeDate, err := date.NewNodeDate()
				if err != nil {
					return nil, err
				}
				switch sparse {
				case true:
					header.Label = append(header.Label, nodeDate.GetLabel())
					header.DateAsLabel = false
				case false:
					header.Date = append(header.Date, nodeDate)
				}
			}
			continue
		}
		header.Label = append(header.Label, label.Title)
	}

	if !sparse {
		for _, date := range cdid.Cunitdate {
			nodeDate, err := date.NewNodeDate()
			if err != nil {
				return nil, err
			}
			header.Date = append(header.Date, nodeDate)
		}
	}

	nodeIDs, inventoryID, err := cdid.NewNodeIDs(sparse)
	if err != nil {
		return nil, err
	}
	if inventoryID != "" {
		header.InventoryNumber = inventoryID
	}
	header.ID = append(header.ID, nodeIDs...)

	return header, nil
}

// NewNode converts EAD c01 to a Archival Node
func (c *Cc01) NewNode(counter *uint64, sparse bool) (*Node, error) {
	atomic.AddUint64(counter, 1)
	node := &Node{
		CTag:    c.XMLName.Local,
		Depth:   int32(1),
		Type:    c.Attrlevel,
		SubType: c.Attrotherlevel,
		Order:   atomic.LoadUint64(counter),
	}

	// add header
	if sparse {
		node.CTag = ""
	}
	header, err := c.Cdid.NewHeader(sparse)
	if err != nil {
		return nil, err
	}
	node.Header = header

	// add scope content
	if c.Cscopecontent != nil && !sparse {
		html, err := xml.Marshal(c.Cscopecontent.Cp)
		if err != nil {
			return nil, err
		}
		node.HTML = string(html)
	}

	// add nested
	parentIDS := []string{header.GetInventoryNumber()}

	if len(c.Nested) != 0 {
		for _, nn := range c.Nested {
			n, err := nn.NewNode(parentIDS, counter, sparse)
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

// NewNode converts EAD nested cLevel to an Archival Node
func (c *Cc02) NewNode(parentIDs []string, counter *uint64, sparse bool) (*Node, error) {
	atomic.AddUint64(counter, 1)
	node := &Node{
		CTag:      c.XMLName.Local,
		Depth:     int32(len(parentIDs) + 1),
		Type:      c.Attrlevel,
		SubType:   c.Attrotherlevel,
		ParentIDs: parentIDs,
		Order:     atomic.LoadUint64(counter),
	}

	// add header
	if sparse {
		node.CTag = ""
	}
	header, err := c.Cdid.NewHeader(sparse)
	if err != nil {
		return nil, err
	}
	node.Header = header

	// add scope content
	if c.Cscopecontent != nil && !sparse {
		html, err := xml.Marshal(c.Cscopecontent.Cp)
		if err != nil {
			return nil, err
		}
		node.HTML = string(html)
	}

	// add nested
	parentIDs = append(parentIDs, header.GetInventoryNumber())

	if len(c.Nested) != 0 {
		for _, nn := range c.Nested {
			n, err := nn.NewNode(parentIDs, counter, sparse)
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

// NewNode converts EAD nested cLevel to an Archival Node
func (c *Cc03) NewNode(parentIDs []string, counter *uint64, sparse bool) (*Node, error) {
	atomic.AddUint64(counter, 1)
	node := &Node{
		CTag:      c.XMLName.Local,
		Depth:     int32(len(parentIDs) + 1),
		Type:      c.Attrlevel,
		SubType:   c.Attrotherlevel,
		ParentIDs: parentIDs,
		Order:     atomic.LoadUint64(counter),
	}

	// add header
	if sparse {
		node.CTag = ""
	}
	header, err := c.Cdid.NewHeader(sparse)
	if err != nil {
		return nil, err
	}
	node.Header = header

	// add scope content
	if c.Cscopecontent != nil && !sparse {
		html, err := xml.Marshal(c.Cscopecontent.Cp)
		if err != nil {
			return nil, err
		}
		node.HTML = string(html)
	}

	// add nested
	parentIDs = append(parentIDs, header.GetInventoryNumber())

	if len(c.Nested) != 0 {
		for _, nn := range c.Nested {
			n, err := nn.NewNode(parentIDs, counter, sparse)
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

// NewNode converts EAD nested cLevel to an Archival Node
func (c *Cc04) NewNode(parentIDs []string, counter *uint64, sparse bool) (*Node, error) {
	atomic.AddUint64(counter, 1)
	node := &Node{
		CTag:      c.XMLName.Local,
		Depth:     int32(len(parentIDs) + 1),
		Type:      c.Attrlevel,
		SubType:   c.Attrotherlevel,
		ParentIDs: parentIDs,
		Order:     atomic.LoadUint64(counter),
	}

	// add header
	if sparse {
		node.CTag = ""
	}
	header, err := c.Cdid.NewHeader(sparse)
	if err != nil {
		return nil, err
	}
	node.Header = header

	// add scope content
	if c.Cscopecontent != nil && !sparse {
		html, err := xml.Marshal(c.Cscopecontent.Cp)
		if err != nil {
			return nil, err
		}
		node.HTML = string(html)
	}

	// add nested
	parentIDs = append(parentIDs, header.GetInventoryNumber())

	if len(c.Nested) != 0 {
		for _, nn := range c.Nested {
			n, err := nn.NewNode(parentIDs, counter, sparse)
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

// NewNode converts EAD nested cLevel to an Archival Node
func (c *Cc05) NewNode(parentIDs []string, counter *uint64, sparse bool) (*Node, error) {
	atomic.AddUint64(counter, 1)
	node := &Node{
		CTag:      c.XMLName.Local,
		Depth:     int32(len(parentIDs) + 1),
		Type:      c.Attrlevel,
		SubType:   c.Attrotherlevel,
		ParentIDs: parentIDs,
		Order:     atomic.LoadUint64(counter),
	}

	// add header
	if sparse {
		node.CTag = ""
	}
	header, err := c.Cdid.NewHeader(sparse)
	if err != nil {
		return nil, err
	}
	node.Header = header

	// add scope content
	if c.Cscopecontent != nil && !sparse {
		html, err := xml.Marshal(c.Cscopecontent.Cp)
		if err != nil {
			return nil, err
		}
		node.HTML = string(html)
	}

	// add nested
	parentIDs = append(parentIDs, header.GetInventoryNumber())

	if len(c.Nested) != 0 {
		for _, nn := range c.Nested {
			n, err := nn.NewNode(parentIDs, counter, sparse)
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

// NewNode converts EAD nested cLevel to an Archival Node
func (c *Cc06) NewNode(parentIDs []string, counter *uint64, sparse bool) (*Node, error) {
	atomic.AddUint64(counter, 1)
	node := &Node{
		CTag:      c.XMLName.Local,
		Depth:     int32(len(parentIDs) + 1),
		Type:      c.Attrlevel,
		SubType:   c.Attrotherlevel,
		ParentIDs: parentIDs,
		Order:     atomic.LoadUint64(counter),
	}

	// add header
	if sparse {
		node.CTag = ""
	}
	header, err := c.Cdid.NewHeader(sparse)
	if err != nil {
		return nil, err
	}
	node.Header = header

	// add scope content
	if c.Cscopecontent != nil && !sparse {
		html, err := xml.Marshal(c.Cscopecontent.Cp)
		if err != nil {
			return nil, err
		}
		node.HTML = string(html)
	}

	// add nested
	parentIDs = append(parentIDs, header.GetInventoryNumber())

	if len(c.Nested) != 0 {
		for _, nn := range c.Nested {
			n, err := nn.NewNode(parentIDs, counter, sparse)
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

// NewNode converts EAD nested cLevel to an Archival Node
func (c *Cc07) NewNode(parentIDs []string, counter *uint64, sparse bool) (*Node, error) {
	atomic.AddUint64(counter, 1)
	node := &Node{
		CTag:      c.XMLName.Local,
		Depth:     int32(len(parentIDs) + 1),
		Type:      c.Attrlevel,
		SubType:   c.Attrotherlevel,
		ParentIDs: parentIDs,
		Order:     atomic.LoadUint64(counter),
	}

	// add header
	if sparse {
		node.CTag = ""
	}
	header, err := c.Cdid.NewHeader(sparse)
	if err != nil {
		return nil, err
	}
	node.Header = header

	// add scope content
	if c.Cscopecontent != nil && !sparse {
		html, err := xml.Marshal(c.Cscopecontent.Cp)
		if err != nil {
			return nil, err
		}
		node.HTML = string(html)
	}

	// add nested
	parentIDs = append(parentIDs, header.GetInventoryNumber())

	if len(c.Nested) != 0 {
		for _, nn := range c.Nested {
			n, err := nn.NewNode(parentIDs, counter, sparse)
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

// NewNode converts EAD nested cLevel to an Archival Node
func (c *Cc08) NewNode(parentIDs []string, counter *uint64, sparse bool) (*Node, error) {
	atomic.AddUint64(counter, 1)
	node := &Node{
		CTag:      c.XMLName.Local,
		Depth:     int32(len(parentIDs) + 1),
		Type:      c.Attrlevel,
		SubType:   c.Attrotherlevel,
		ParentIDs: parentIDs,
		Order:     atomic.LoadUint64(counter),
	}

	// add header
	if sparse {
		node.CTag = ""
	}
	header, err := c.Cdid.NewHeader(sparse)
	if err != nil {
		return nil, err
	}
	node.Header = header

	// add scope content
	if c.Cscopecontent != nil && !sparse {
		html, err := xml.Marshal(c.Cscopecontent.Cp)
		if err != nil {
			return nil, err
		}
		node.HTML = string(html)
	}

	// add nested
	parentIDs = append(parentIDs, header.GetInventoryNumber())

	if len(c.Nested) != 0 {
		for _, nn := range c.Nested {
			n, err := nn.NewNode(parentIDs, counter, sparse)
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

// NewNode converts EAD nested cLevel to an Archival Node
func (c *Cc09) NewNode(parentIDs []string, counter *uint64, sparse bool) (*Node, error) {
	atomic.AddUint64(counter, 1)
	node := &Node{
		CTag:      c.XMLName.Local,
		Depth:     int32(len(parentIDs) + 1),
		Type:      c.Attrlevel,
		SubType:   c.Attrotherlevel,
		ParentIDs: parentIDs,
		Order:     atomic.LoadUint64(counter),
	}

	// add header
	if sparse {
		node.CTag = ""
	}
	header, err := c.Cdid.NewHeader(sparse)
	if err != nil {
		return nil, err
	}
	node.Header = header

	// add scope content
	if c.Cscopecontent != nil && !sparse {
		html, err := xml.Marshal(c.Cscopecontent.Cp)
		if err != nil {
			return nil, err
		}
		node.HTML = string(html)
	}

	// add nested
	parentIDs = append(parentIDs, header.GetInventoryNumber())

	if len(c.Nested) != 0 {
		for _, nn := range c.Nested {
			n, err := nn.NewNode(parentIDs, counter, sparse)
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

// NewNode converts EAD nested cLevel to an Archival Node
func (c *Cc10) NewNode(parentIDs []string, counter *uint64, sparse bool) (*Node, error) {
	atomic.AddUint64(counter, 1)
	node := &Node{
		CTag:      c.XMLName.Local,
		Depth:     int32(len(parentIDs) + 1),
		Type:      c.Attrlevel,
		SubType:   c.Attrotherlevel,
		ParentIDs: parentIDs,
		Order:     atomic.LoadUint64(counter),
	}

	// add header
	if sparse {
		node.CTag = ""
	}
	header, err := c.Cdid.NewHeader(sparse)
	if err != nil {
		return nil, err
	}
	node.Header = header

	// add scope content
	if c.Cscopecontent != nil && !sparse {
		html, err := xml.Marshal(c.Cscopecontent.Cp)
		if err != nil {
			return nil, err
		}
		node.HTML = string(html)
	}

	// add nested
	parentIDs = append(parentIDs, header.GetInventoryNumber())

	if len(c.Nested) != 0 {
		for _, nn := range c.Nested {
			n, err := nn.NewNode(parentIDs, counter, sparse)
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

// NewNode converts EAD nested cLevel to an Archival Node
func (c *Cc11) NewNode(parentIDs []string, counter *uint64, sparse bool) (*Node, error) {
	atomic.AddUint64(counter, 1)
	node := &Node{
		CTag:      c.XMLName.Local,
		Depth:     int32(len(parentIDs) + 1),
		Type:      c.Attrlevel,
		SubType:   c.Attrotherlevel,
		ParentIDs: parentIDs,
		Order:     atomic.LoadUint64(counter),
	}

	// add header
	if sparse {
		node.CTag = ""
	}
	header, err := c.Cdid.NewHeader(sparse)
	if err != nil {
		return nil, err
	}
	node.Header = header

	// add scope content
	if c.Cscopecontent != nil && !sparse {
		html, err := xml.Marshal(c.Cscopecontent.Cp)
		if err != nil {
			return nil, err
		}
		node.HTML = string(html)
	}

	// add nested
	parentIDs = append(parentIDs, header.GetInventoryNumber())

	if len(c.Nested) != 0 {
		for _, nn := range c.Nested {
			n, err := nn.NewNode(parentIDs, counter, sparse)
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}

// NewNode converts EAD nested cLevel to an Archival Node
func (c *Cc12) NewNode(parentIDs []string, counter *uint64, sparse bool) (*Node, error) {
	atomic.AddUint64(counter, 1)
	node := &Node{
		CTag:      c.XMLName.Local,
		Depth:     int32(len(parentIDs) + 1),
		Type:      c.Attrlevel,
		SubType:   c.Attrotherlevel,
		ParentIDs: parentIDs,
		Order:     atomic.LoadUint64(counter),
	}

	// add header
	if sparse {
		node.CTag = ""
	}
	header, err := c.Cdid.NewHeader(sparse)
	if err != nil {
		return nil, err
	}
	node.Header = header

	// add scope content
	if c.Cscopecontent != nil && !sparse {
		html, err := xml.Marshal(c.Cscopecontent.Cp)
		if err != nil {
			return nil, err
		}
		node.HTML = string(html)
	}

	// add nested
	parentIDs = append(parentIDs, header.GetInventoryNumber())

	if len(c.Nested) != 0 {
		for _, nn := range c.Nested {
			n, err := nn.NewNode(parentIDs, counter, sparse)
			if err != nil {
				return nil, err
			}
			node.Nodes = append(node.Nodes, n)
		}
	}
	return node, nil
}