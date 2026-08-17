// Command archcheck enforces the identity layering at build time. A layer
// rule that is only written down is a suggestion; this makes it a gate.
package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const internalPrefix = "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/"

// importable lists, for every layer, the layers it may import directly. A layer
// missing from a value set is unreachable from that key by construction.
var importable = map[string][]string{
	"model":      {},
	"biz":        {"model"},
	"data":       {"biz", "model"},
	"api":        {},
	"appmodel":   {"model"},
	"service":    {"appmodel", "model"},
	"logic":      {"appmodel", "service", "biz", "model", "authctx"},
	"controller": {"api", "appmodel", "service", "common", "authctx"},
	"router":     {"controller", "service", "common", "authctx"},
	"authctx":    {},
	"common":     {"model", "authctx", "common"},
	"config":     {},
	"app":        {"model", "biz", "data", "appmodel", "service", "logic", "controller", "router", "common", "authctx", "config", "app"},
	"cmd":        {"app", "config"},
}

// forbiddenImports lists, per layer, the import prefixes that layer must not
// reach. The inner layers stay framework-free so one implementation can serve
// HTTP, gRPC and tests alike; config may decode its own file but never opens a
// port or a connection. These are checked through the module's own packages as
// well, so a layer cannot borrow a framework via a neighbour it may import.
var forbiddenImports = map[string][]string{
	"model":    {"github.com/gogf/gf", "google.golang.org/grpc", "database/sql", "net/http"},
	"biz":      {"github.com/gogf/gf", "google.golang.org/grpc", "database/sql", "net/http"},
	"appmodel": {"github.com/gogf/gf", "google.golang.org/grpc", "database/sql", "net/http"},
	"service":  {"github.com/gogf/gf", "google.golang.org/grpc", "database/sql", "net/http"},
	"authctx":  {"github.com/gogf/gf", "google.golang.org/grpc", "database/sql"},
	"config":   {"github.com/gogf/gf/v2/net", "google.golang.org/grpc", "database/sql", "net/http"},
	"logic":    {"github.com/gogf/gf", "database/sql"},
	"data":     {"github.com/gogf/gf", "net/http"},
}

// pkg is one package of this module, addressed by its directory relative to the
// internal root.
type pkg struct {
	directory string
	layer     string
	surface   string
	files     []string
	internal  []string
	external  []string
}

func main() {
	root := "internal/identity"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	violations, err := check(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "archcheck:", err)
		os.Exit(2)
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		for _, violation := range violations {
			fmt.Fprintln(os.Stderr, "archcheck:", violation)
		}
		fmt.Fprintf(os.Stderr, "archcheck: %d 处违规\n", len(violations))
		os.Exit(1)
	}
	fmt.Println("archcheck: 分层约束通过")
}

func check(root string) ([]string, error) {
	packages, err := load(root)
	if err != nil {
		return nil, err
	}
	var violations []string
	for _, directory := range sorted(packages) {
		current := packages[directory]
		violations = append(violations, directEdges(packages, current)...)
		violations = append(violations, reachableFrameworks(packages, current)...)
	}
	return violations, nil
}

func load(root string) (map[string]*pkg, error) {
	packages := map[string]*pkg{}
	fileSet := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		relative := strings.TrimPrefix(filepath.ToSlash(path), filepath.ToSlash(root)+"/")
		directory := filepath.ToSlash(filepath.Dir(relative))
		layer, surface := classify(directory)
		if layer == "" {
			return nil
		}
		current, ok := packages[directory]
		if !ok {
			current = &pkg{directory: directory, layer: layer, surface: surface}
			packages[directory] = current
		}
		current.files = append(current.files, relative)

		parsed, err := parser.ParseFile(fileSet, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range parsed.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if target, ok := strings.CutPrefix(imported, internalPrefix); ok {
				current.internal = appendOnce(current.internal, target)
				continue
			}
			current.external = appendOnce(current.external, imported)
		}
		return nil
	})
	return packages, err
}

// directEdges checks the layer graph and surface isolation on what a package
// imports itself.
func directEdges(packages map[string]*pkg, current *pkg) []string {
	var violations []string
	for _, target := range current.internal {
		imported, ok := packages[target]
		if !ok {
			continue
		}
		// External test packages may import the package in their own directory.
		// This does not create a dependency between architectural layers.
		if imported.directory == current.directory {
			continue
		}
		if current.surface != "" && imported.surface != "" && current.surface != imported.surface {
			violations = append(violations, fmt.Sprintf("%s（surface %s）不得导入 surface %s；surface 之间必须隔离",
				current.directory, current.surface, imported.surface))
			continue
		}
		if !allowed(current.layer, imported.layer) {
			violations = append(violations, fmt.Sprintf("%s（%s 层）不得导入 %s（%s 层）",
				current.directory, current.layer, imported.directory, imported.layer))
		}
	}
	return violations
}

// reachableFrameworks follows this module's own import edges, so a framework-free
// layer cannot pick up a framework through a package it is allowed to import.
// Only internal edges are followed; a third-party package's own dependencies are
// out of reach and out of scope.
func reachableFrameworks(packages map[string]*pkg, current *pkg) []string {
	forbidden := forbiddenImports[current.layer]
	if len(forbidden) == 0 {
		return nil
	}
	var violations []string
	visited := map[string]bool{}
	var walk func(node *pkg, chain []string)
	walk = func(node *pkg, chain []string) {
		if visited[node.directory] {
			return
		}
		visited[node.directory] = true
		for _, imported := range node.external {
			for _, framework := range forbidden {
				if !strings.HasPrefix(imported, framework) {
					continue
				}
				via := "直接依赖"
				if len(chain) > 1 {
					via = "经由 " + strings.Join(chain[1:], " → ")
				}
				violations = append(violations, fmt.Sprintf("%s（%s 层）不得依赖 %s：%s",
					current.directory, current.layer, framework, via))
			}
		}
		for _, target := range node.internal {
			if next, ok := packages[target]; ok {
				walk(next, append(chain, next.directory))
			}
		}
	}
	walk(current, []string{current.directory})
	return violations
}

func allowed(layer, target string) bool {
	for _, candidate := range importable[layer] {
		if candidate == target {
			return true
		}
	}
	return false
}

// classify maps a package directory to its layer and, inside application, to
// the surface that owns it.
func classify(directory string) (layer, surface string) {
	parts := strings.Split(strings.Trim(directory, "/"), "/")
	if len(parts) == 0 || parts[0] == "" || parts[0] == "." {
		return "", ""
	}
	switch parts[0] {
	case "capability":
		if len(parts) > 2 && parts[2] == "model" {
			return "model", ""
		}
		return "biz", ""
	case "biz":
		if len(parts) > 1 && parts[1] == "model" {
			return "model", ""
		}
		return "biz", ""
	case "data":
		return "data", ""
	case "config":
		return "config", ""
	case "app":
		return "app", ""
	case "cmd":
		return "cmd", ""
	case "common":
		// authctx is the only transport-neutral member of common, so it is the
		// only one an inner layer may read the caller from.
		if len(parts) > 1 && parts[1] == "authctx" {
			return "authctx", ""
		}
		return "common", ""
	case "application":
		if len(parts) < 3 {
			return "", ""
		}
		surface = parts[1]
		switch parts[2] {
		case "api", "appmodel", "service", "logic", "controller", "router":
			return parts[2], surface
		}
	}
	return "", ""
}

func appendOnce(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func sorted(packages map[string]*pkg) []string {
	keys := make([]string, 0, len(packages))
	for key := range packages {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
