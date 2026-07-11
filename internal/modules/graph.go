package modules

import (
	"log"
	"strings"
)

// WarnModuleCycles logs a warning when manifest declarations suggest a cyclic
// event-routing graph. Wildcard patterns make detection incomplete; runtime
// dispatch uses a seen list as the safety net.
func WarnModuleCycles(mods []Module) {
	names := make([]string, 0, len(mods))
	byName := make(map[string]Manifest, len(mods))
	for _, mod := range mods {
		manifest, err := loadModuleManifest(mod)
		if err != nil {
			continue
		}
		if !manifest.EventRoutes() && len(manifest.Provides) == 0 {
			continue
		}
		names = append(names, manifest.Name)
		byName[manifest.Name] = manifest
	}

	adj := make(map[string][]string, len(names))
	for _, from := range names {
		fromManifest := byName[from]
		for _, to := range names {
			if from == to {
				continue
			}
			toManifest := byName[to]
			if !toManifest.EventRoutes() {
				continue
			}
			if patternsOverlap(fromManifest.Provides, toManifest.Consumes) {
				adj[from] = append(adj[from], to)
			}
		}
	}

	cycles := findCycles(names, adj)
	for _, cycle := range cycles {
		log.Printf("modules: warning: possible event-routing cycle: %s", strings.Join(cycle, " -> "))
	}
}

func patternsOverlap(provides, consumes []string) bool {
	for _, provide := range provides {
		for _, consume := range consumes {
			if patternOverlaps(provide, consume) {
				return true
			}
		}
	}
	return false
}

func patternOverlaps(provide, consume string) bool {
	if !strings.Contains(provide, "*") && !strings.Contains(consume, "*") {
		return provide == consume
	}
	if !strings.Contains(provide, "*") {
		return MatchType([]string{consume}, provide)
	}
	if !strings.Contains(consume, "*") {
		return MatchType([]string{provide}, consume)
	}
	return true
}

func findCycles(nodes []string, adj map[string][]string) [][]string {
	visited := make(map[string]bool, len(nodes))
	stack := make(map[string]bool, len(nodes))
	var cycles [][]string

	var visit func(node string, path []string)
	visit = func(node string, path []string) {
		if stack[node] {
			for i, n := range path {
				if n == node {
					cycle := append(append([]string(nil), path[i:]...), node)
					cycles = append(cycles, cycle)
					break
				}
			}
			return
		}
		if visited[node] {
			return
		}
		visited[node] = true
		stack[node] = true
		nextPath := append(path, node)
		for _, next := range adj[node] {
			visit(next, nextPath)
		}
		delete(stack, node)
	}

	for _, node := range nodes {
		visit(node, nil)
	}
	return cycles
}
