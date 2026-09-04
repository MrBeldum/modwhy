package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ModuleID is "path@version" as emitted by go mod graph, or just "path" for the main module.
type ModuleID string

// PathOnly strips the @version suffix from a module ID.
func PathOnly(id ModuleID) string {
	s := string(id)
	if i := strings.Index(s, "@"); i >= 0 {
		return s[:i]
	}
	return s
}

// Graph is a directed requirement graph: from -> list of to (with versions).
type Graph struct {
	// Forward edges: A requires B
	Out map[ModuleID][]ModuleID
	// All known module IDs (with versions where present)
	Nodes map[ModuleID]struct{}
	// Main module path (no version)
	Main Path
}

type Path string

// ParseGraph parses "go mod graph" output (lines "A@v B@v" or "mainpath B@v").
func ParseGraph(r io.Reader) (*Graph, error) {
	g := &Graph{
		Out:   make(map[ModuleID][]ModuleID),
		Nodes: make(map[ModuleID]struct{}),
	}
	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			return nil, fmt.Errorf("graph line %d: want 2 fields, got %q", lineNo, line)
		}
		from, to := ModuleID(parts[0]), ModuleID(parts[1])
		g.Nodes[from] = struct{}{}
		g.Nodes[to] = struct{}{}
		g.Out[from] = append(g.Out[from], to)
		if g.Main == "" {
			g.Main = Path(PathOnly(from))
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return g, nil
}

// MatchTarget returns all ModuleIDs whose path equals target (or equals target when target includes @ver).
func (g *Graph) MatchTarget(target string, moduleOnly bool) []ModuleID {
	_ = moduleOnly // reserved for package-mode distinction at call site
	wantPath := target
	wantVer := ""
	if i := strings.Index(target, "@"); i >= 0 {
		wantPath = target[:i]
		wantVer = target[i+1:]
	}
	var hits []ModuleID
	seen := make(map[ModuleID]struct{})
	for id := range g.Nodes {
		p := PathOnly(id)
		if p != wantPath {
			continue
		}
		if wantVer != "" {
			s := string(id)
			at := strings.Index(s, "@")
			if at < 0 || s[at+1:] != wantVer {
				continue
			}
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		hits = append(hits, id)
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i] < hits[j] })
	return hits
}

// FindMainNode returns the main module's ModuleID as it appears in the graph (usually without @version).
func (g *Graph) FindMainNode() ModuleID {
	main := string(g.Main)
	// Prefer exact path-only node
	if _, ok := g.Nodes[ModuleID(main)]; ok {
		return ModuleID(main)
	}
	for id := range g.Nodes {
		if PathOnly(id) == main {
			return id
		}
	}
	return ModuleID(main)
}

// FindPaths finds up to maxPaths distinct simple paths from main to any target hit.
// Paths are sequences of ModuleIDs from root to leaf.
func (g *Graph) FindPaths(targets []ModuleID, maxPaths int) [][]ModuleID {
	if maxPaths <= 0 || len(targets) == 0 {
		return nil
	}
	targetSet := make(map[ModuleID]struct{}, len(targets))
	for _, t := range targets {
		targetSet[t] = struct{}{}
	}
	root := g.FindMainNode()
	var results [][]ModuleID
	var dfs func(cur ModuleID, trail []ModuleID, visited map[ModuleID]struct{})
	dfs = func(cur ModuleID, trail []ModuleID, visited map[ModuleID]struct{}) {
		if len(results) >= maxPaths {
			return
		}
		if _, hit := targetSet[cur]; hit && len(trail) > 0 {
			// Copy path
			p := make([]ModuleID, len(trail))
			copy(p, trail)
			results = append(results, p)
			return
		}
		for _, next := range g.Out[cur] {
			if _, seen := visited[next]; seen {
				continue
			}
			visited[next] = struct{}{}
			dfs(next, append(trail, next), visited)
			delete(visited, next)
			if len(results) >= maxPaths {
				return
			}
		}
	}
	visited := map[ModuleID]struct{}{root: {}}
	// If root itself is a target (unusual), report single-node path
	if _, hit := targetSet[root]; hit {
		results = append(results, []ModuleID{root})
		if len(results) >= maxPaths {
			return results
		}
	}
	dfs(root, []ModuleID{root}, visited)
	return results
}

// FormatASCII renders paths as separate ASCII trees (root at top, target at bottom).
func FormatASCII(paths [][]ModuleID) string {
	if len(paths) == 0 {
		return ""
	}
	var b strings.Builder
	for i, path := range paths {
		if i > 0 {
			b.WriteByte('\n')
		}
		for j, id := range path {
			if j == 0 {
				b.WriteString(string(id))
				b.WriteByte('\n')
				continue
			}
			indent := strings.Repeat("    ", j-1)
			b.WriteString(indent)
			b.WriteString("└── ")
			b.WriteString(string(id))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// PathJSON is the JSON shape for -json output.
type PathJSON struct {
	Path []string `json:"path"`
}

type ResultJSON struct {
	Target string     `json:"target"`
	Paths  []PathJSON `json:"paths"`
}

// FormatJSON encodes the found paths as JSON.
func FormatJSON(target string, paths [][]ModuleID) ([]byte, error) {
	out := ResultJSON{Target: target, Paths: make([]PathJSON, 0, len(paths))}
	for _, p := range paths {
		pj := PathJSON{Path: make([]string, len(p))}
		for i, id := range p {
			pj.Path[i] = string(id)
		}
		out.Paths = append(out.Paths, pj)
	}
	return json.MarshalIndent(out, "", "  ")
}
