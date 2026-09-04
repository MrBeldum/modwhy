package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("modwhy", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dir := fs.String("C", ".", "module root directory")
	moduleOnly := fs.Bool("m", false, "treat argument as module path only")
	jsonOut := fs.Bool("json", false, "emit JSON on stdout")
	maxPaths := fs.Int("max", 5, "max dependency paths to show")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "usage: modwhy [flags] <module-or-package>\n")
		fs.PrintDefaults()
		return 2
	}
	target := fs.Arg(0)
	if *maxPaths < 1 {
		fmt.Fprintf(os.Stderr, "modwhy: -max must be >= 1\n")
		return 2
	}

	modRoot, err := findModuleRoot(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "modwhy: %v\n", err)
		return 2
	}

	targetMod := target
	if !*moduleOnly {
		// Best-effort: if target looks like a package under a known module,
		// resolve to its module via go list -m -f '{{.Path}}' or go list -json.
		if resolved, rerr := resolveModulePath(modRoot, target); rerr == nil && resolved != "" {
			targetMod = resolved
		}
		// else keep target as-is (treat as module path)
	}

	graphData, err := runGoModGraph(modRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "modwhy: %v\n", err)
		return 1
	}

	g, err := ParseGraph(bytes.NewReader(graphData))
	if err != nil {
		fmt.Fprintf(os.Stderr, "modwhy: parse graph: %v\n", err)
		return 1
	}

	hits := g.MatchTarget(targetMod, *moduleOnly)
	if len(hits) == 0 {
		fmt.Fprintf(os.Stderr, "modwhy: %s not found in module graph\n", target)
		return 1
	}

	paths := g.FindPaths(hits, *maxPaths)
	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "modwhy: no path from main module to %s\n", target)
		return 1
	}

	if *jsonOut {
		b, err := FormatJSON(target, paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "modwhy: json: %v\n", err)
			return 1
		}
		fmt.Println(string(b))
	} else {
		fmt.Print(FormatASCII(paths))
	}
	return 0
}

func findModuleRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	cmd := exec.Command("go", "env", "GOMOD")
	cmd.Dir = abs
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go env GOMOD in %s: %w", abs, err)
	}
	gomod := strings.TrimSpace(string(out))
	if gomod == "" || gomod == "/dev/null" {
		return "", fmt.Errorf("no go.mod found under %s", abs)
	}
	return filepath.Dir(gomod), nil
}

func runGoModGraph(modRoot string) ([]byte, error) {
	cmd := exec.Command("go", "mod", "graph")
	cmd.Dir = modRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("go mod graph: %s", msg)
	}
	return out, nil
}

// resolveModulePath tries to map a package path to its module path using go list.
func resolveModulePath(modRoot, pkgOrMod string) (string, error) {
	// Already looks like module@version
	if strings.Contains(pkgOrMod, "@") {
		return PathOnly(ModuleID(pkgOrMod)), nil
	}
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Path}}", pkgOrMod)
	cmd.Dir = modRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		p := strings.TrimSpace(string(out))
		if p != "" {
			return p, nil
		}
	}
	// Try as package: go list -f '{{.Module.Path}}'
	cmd = exec.Command("go", "list", "-f", "{{if .Module}}{{.Module.Path}}{{end}}", pkgOrMod)
	cmd.Dir = modRoot
	cmd.Stderr = &stderr
	out, err = cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
