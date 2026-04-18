/*
 * Warp (C) 2019-2026 MinIO, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package iceberg

import (
	"fmt"
)

type TreeConfig struct {
	NamespaceWidth   int
	NamespaceDepth   int
	TablesPerNS      int
	ViewsPerNS       int
	ColumnsPerTable  int
	ColumnsPerView   int
	PropertiesPerNS  int
	PropertiesPerTbl int
	PropertiesPerVw  int
	BaseLocation     string
	CatalogName      string
}

type Tree struct {
	cfg TreeConfig
}

func NewTree(cfg TreeConfig) *Tree {
	return &Tree{cfg: cfg}
}

// isFlat returns true when the tree represents a flat namespace layout
// (depth=1). In flat mode, all namespaces are siblings with single-element
// paths — this is required by catalogs like AWS S3 Tables that only support
// single-level namespaces. The tree produces NamespaceWidth independent
// leaf namespaces instead of a single root node.
func (t *Tree) isFlat() bool {
	return t.cfg.NamespaceDepth == 1
}

// TotalNamespaces returns the total number of namespace nodes in the tree.
//
// Tree has two layout modes:
//
//   - Flat mode (depth=1): Produces NamespaceWidth independent leaf
//     namespaces. Each namespace has a single-element path (e.g. ["ns0"]).
//     Example: width=5, depth=1 → 5 namespaces: ns0..ns4, all are leaves.
//
//   - Hierarchical mode (depth>1): Produces a full N-ary tree where each
//     node at depth k has NamespaceWidth children. Only leaf nodes (at
//     depth = NamespaceDepth-1) contain tables/views.
//     Example: width=2, depth=3 → 7 namespaces in a binary tree:
//     ns0 → ns1,ns2 → ns3,ns4,ns5,ns6
func (t *Tree) TotalNamespaces() int {
	if t.isFlat() {
		return t.cfg.NamespaceWidth
	}
	if t.cfg.NamespaceWidth == 1 {
		return t.cfg.NamespaceDepth
	}
	n := 1
	for i := 0; i < t.cfg.NamespaceDepth; i++ {
		n *= t.cfg.NamespaceWidth
	}
	return (n - 1) / (t.cfg.NamespaceWidth - 1)
}

// LeafNamespaces returns the number of leaf namespaces that hold tables/views.
//
//   - Flat mode: all namespaces are leaves.
//   - Hierarchical mode: leaf nodes are at the bottom level of the N-ary tree.
func (t *Tree) LeafNamespaces() int {
	if t.isFlat() {
		return t.cfg.NamespaceWidth
	}
	n := 1
	for i := 0; i < t.cfg.NamespaceDepth-1; i++ {
		n *= t.cfg.NamespaceWidth
	}
	return n
}

func (t *Tree) TotalTables() int {
	return t.LeafNamespaces() * t.cfg.TablesPerNS
}

func (t *Tree) TotalViews() int {
	return t.LeafNamespaces() * t.cfg.ViewsPerNS
}

// DepthOf returns the depth of the namespace node at the given ordinal.
//
//   - Flat mode: all nodes are at depth 0.
//   - Hierarchical mode: depth is determined by the level in the N-ary tree.
func (t *Tree) DepthOf(ordinal int) int {
	if t.isFlat() {
		return 0
	}
	if t.cfg.NamespaceWidth == 1 {
		return ordinal
	}
	depth := 0
	size := 1
	total := 0
	for total+size <= ordinal {
		total += size
		size *= t.cfg.NamespaceWidth
		depth++
	}
	return depth
}

// PathToRoot returns the hierarchical namespace path for the node at the
// given ordinal. The path is ordered from root to leaf.
//
//   - Flat mode: returns a single-element path (e.g. ["ns0"]), suitable for
//     catalogs that only support single-level namespaces like AWS S3 Tables.
//   - Hierarchical mode: returns a multi-element path from root ancestor to
//     this node (e.g. ["ns0", "ns1", "ns3"]).
func (t *Tree) PathToRoot(ordinal int) []string {
	if t.isFlat() {
		return []string{t.namespaceName(ordinal)}
	}
	// Build hierarchical path from root to this node
	var path []int
	current := ordinal
	for current >= 0 {
		path = append([]int{current}, path...)
		current = t.parentOf(current)
	}

	result := make([]string, len(path))
	for i, ord := range path {
		result[i] = t.namespaceName(ord)
	}
	return result
}

// parentOf returns the ordinal of the parent node, or -1 if the node has no
// parent (root node in hierarchical mode, or any node in flat mode).
//
//   - Flat mode: no parent — all namespaces are independent.
//   - Hierarchical mode: parent is determined by N-ary tree structure.
func (t *Tree) parentOf(ordinal int) int {
	if t.isFlat() {
		return -1
	}
	if ordinal == 0 {
		return -1
	}
	if t.cfg.NamespaceWidth == 1 {
		return ordinal - 1
	}
	return (ordinal - 1) / t.cfg.NamespaceWidth
}

// ChildrenOf returns the ordinals of the child nodes of the given namespace.
//
//   - Flat mode: no children — all namespaces are leaves.
//   - Hierarchical mode: returns the width child nodes, or nil if at max depth.
func (t *Tree) ChildrenOf(ordinal int) []int {
	if t.isFlat() {
		return nil
	}

	if t.DepthOf(ordinal) >= t.cfg.NamespaceDepth-1 {
		return nil
	}

	if t.cfg.NamespaceWidth == 1 {
		return []int{ordinal + 1}
	}

	first := ordinal*t.cfg.NamespaceWidth + 1
	children := make([]int, t.cfg.NamespaceWidth)
	for i := 0; i < t.cfg.NamespaceWidth; i++ {
		children[i] = first + i
	}
	return children
}

// IsLeaf returns true if the namespace node at the given ordinal is a leaf
// (contains tables/views rather than sub-namespaces).
//
//   - Flat mode: all namespaces are leaves.
//   - Hierarchical mode: leaf nodes are at depth = NamespaceDepth-1.
func (t *Tree) IsLeaf(ordinal int) bool {
	if t.isFlat() {
		return true
	}
	return t.DepthOf(ordinal) == t.cfg.NamespaceDepth-1
}

func (t *Tree) LeafOrdinals() []int {
	total := t.TotalNamespaces()
	leafCount := t.LeafNamespaces()
	start := total - leafCount

	ordinals := make([]int, leafCount)
	for i := 0; i < leafCount; i++ {
		ordinals[i] = start + i
	}
	return ordinals
}

func (t *Tree) namespaceName(ordinal int) string {
	// MinIO S3 Tables doesn't allow underscores in namespace names
	return fmt.Sprintf("ns%d", ordinal)
}

func (t *Tree) TableName(index int) string {
	return fmt.Sprintf("t%d", index)
}

func (t *Tree) ViewName(index int) string {
	return fmt.Sprintf("v%d", index)
}

func (t *Tree) TableLocation(namespace []string, tableName string) string {
	path := t.cfg.BaseLocation + "/" + t.cfg.CatalogName
	for _, ns := range namespace {
		path += "/" + ns
	}
	return path + "/" + tableName
}

func (t *Tree) Config() TreeConfig {
	return t.cfg
}

type NamespaceInfo struct {
	Ordinal   int
	Path      []string
	IsLeaf    bool
	TableIdxs []int
	ViewIdxs  []int
}

func (t *Tree) AllNamespaces() []NamespaceInfo {
	total := t.TotalNamespaces()
	result := make([]NamespaceInfo, total)

	tableIdx := 0
	viewIdx := 0

	for i := 0; i < total; i++ {
		info := NamespaceInfo{
			Ordinal: i,
			Path:    t.PathToRoot(i),
			IsLeaf:  t.IsLeaf(i),
		}

		if info.IsLeaf {
			info.TableIdxs = make([]int, t.cfg.TablesPerNS)
			for j := 0; j < t.cfg.TablesPerNS; j++ {
				info.TableIdxs[j] = tableIdx
				tableIdx++
			}
			info.ViewIdxs = make([]int, t.cfg.ViewsPerNS)
			for j := 0; j < t.cfg.ViewsPerNS; j++ {
				info.ViewIdxs[j] = viewIdx
				viewIdx++
			}
		}

		result[i] = info
	}

	return result
}

type TableInfo struct {
	Index     int
	Name      string
	Namespace []string
	Location  string
}

func (t *Tree) AllTables() []TableInfo {
	var tables []TableInfo
	tableIdx := 0

	for _, ns := range t.AllNamespaces() {
		if !ns.IsLeaf {
			continue
		}
		for i := 0; i < t.cfg.TablesPerNS; i++ {
			name := t.TableName(tableIdx)
			tables = append(tables, TableInfo{
				Index:     tableIdx,
				Name:      name,
				Namespace: ns.Path,
				Location:  t.TableLocation(ns.Path, name),
			})
			tableIdx++
		}
	}

	return tables
}

type ViewInfo struct {
	Index     int
	Name      string
	Namespace []string
	Location  string
}

func (t *Tree) AllViews() []ViewInfo {
	var views []ViewInfo
	viewIdx := 0

	for _, ns := range t.AllNamespaces() {
		if !ns.IsLeaf {
			continue
		}
		for i := 0; i < t.cfg.ViewsPerNS; i++ {
			name := t.ViewName(viewIdx)
			views = append(views, ViewInfo{
				Index:     viewIdx,
				Name:      name,
				Namespace: ns.Path,
				Location:  t.TableLocation(ns.Path, name),
			})
			viewIdx++
		}
	}

	return views
}
