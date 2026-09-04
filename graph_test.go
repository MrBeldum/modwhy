package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const sampleGraph = `example.com/cmd/app github.com/foo/bar@v1.2.3
example.com/cmd/app github.com/other/lib@v0.9.0
github.com/foo/bar@v1.2.3 github.com/target/mod@v0.1.0
github.com/foo/bar@v1.2.3 github.com/mid/dep@v2.0.0
github.com/mid/dep@v2.0.0 github.com/target/mod@v0.1.0
github.com/other/lib@v0.9.0 golang.org/x/sys@v0.1.0
`

func TestParseGraph(t *testing.T) {
	g, err := ParseGraph(strings.NewReader(sampleGraph))
	if err != nil {
		t.Fatalf("ParseGraph: %v", err)
	}
	if g.Main != "example.com/cmd/app" {
		t.Fatalf("Main = %q, want example.com/cmd/app", g.Main)
	}
	if len(g.Out[ModuleID("example.com/cmd/app")]) != 2 {
		t.Fatalf("root outs = %d, want 2", len(g.Out[ModuleID("example.com/cmd/app")]))
	}
}

func TestPathOnly(t *testing.T) {
	if got := PathOnly("github.com/foo@v1.2.3"); got != "github.com/foo" {
		t.Fatalf("got %q", got)
	}
	if got := PathOnly("example.com/app"); got != "example.com/app" {
		t.Fatalf("got %q", got)
	}
}

func TestMatchTargetAndFindPaths(t *testing.T) {
	g, err := ParseGraph(strings.NewReader(sampleGraph))
	if err != nil {
		t.Fatal(err)
	}
	hits := g.MatchTarget("github.com/target/mod", true)
	if len(hits) != 1 || hits[0] != "github.com/target/mod@v0.1.0" {
		t.Fatalf("hits = %v", hits)
	}
	paths := g.FindPaths(hits, 5)
	if len(paths) < 2 {
		t.Fatalf("expected at least 2 paths, got %d: %v", len(paths), paths)
	}
	// All paths should end at target
	for _, p := range paths {
		if PathOnly(p[len(p)-1]) != "github.com/target/mod" {
			t.Fatalf("path does not end at target: %v", p)
		}
		if PathOnly(p[0]) != "example.com/cmd/app" {
			t.Fatalf("path does not start at main: %v", p)
		}
	}
}

func TestFindPathsMax(t *testing.T) {
	g, err := ParseGraph(strings.NewReader(sampleGraph))
	if err != nil {
		t.Fatal(err)
	}
	hits := g.MatchTarget("github.com/target/mod", true)
	paths := g.FindPaths(hits, 1)
	if len(paths) != 1 {
		t.Fatalf("max=1 got %d paths", len(paths))
	}
}

func TestFormatASCII(t *testing.T) {
	paths := [][]ModuleID{
		{"example.com/cmd/app", "github.com/foo/bar@v1.2.3", "github.com/target/mod@v0.1.0"},
	}
	got := FormatASCII(paths)
	want := "example.com/cmd/app\n└── github.com/foo/bar@v1.2.3\n    └── github.com/target/mod@v0.1.0\n"
	if got != want {
		t.Fatalf("ASCII mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatJSON(t *testing.T) {
	paths := [][]ModuleID{
		{"example.com/cmd/app", "github.com/target/mod@v0.1.0"},
	}
	b, err := FormatJSON("github.com/target/mod", paths)
	if err != nil {
		t.Fatal(err)
	}
	var r ResultJSON
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	if r.Target != "github.com/target/mod" || len(r.Paths) != 1 || len(r.Paths[0].Path) != 2 {
		t.Fatalf("unexpected JSON: %+v", r)
	}
}

func TestNotFound(t *testing.T) {
	g, err := ParseGraph(strings.NewReader(sampleGraph))
	if err != nil {
		t.Fatal(err)
	}
	hits := g.MatchTarget("github.com/missing/mod", true)
	if len(hits) != 0 {
		t.Fatalf("expected no hits, got %v", hits)
	}
}

func TestIntegrationSkipWithoutModule(t *testing.T) {
	// Smoke: ParseGraph on empty is fine; real go mod graph needs a module.
	g, err := ParseGraph(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 0 {
		t.Fatalf("expected empty graph")
	}
}
