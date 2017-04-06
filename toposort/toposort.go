package toposort

// author: github.com/dfuentes

// buildDegreeMap builds a map which contains the incoming degree of each vertex
// in an adjacency list as well as a list of vertices with no incoming edges
func buildDegreeMap(adjacencyList map[string][]string) (map[string]int, []string) {
	inDegrees := make(map[string]int, len(adjacencyList))
	for key, val := range adjacencyList {
		if _, ok := inDegrees[key]; !ok {
			inDegrees[key] = 0
		}
		for _, v := range val {
			inDegrees[v]++
		}
	}

	noIncoming := []string{}
	for key, val := range inDegrees {
		if val == 0 {
			noIncoming = append(noIncoming, key)
		}
	}
	return inDegrees, noIncoming
}

// CycleDetectedError is returned if a cycle is detected while performing a topo sort
type CycleDetectedError struct{}

func (e CycleDetectedError) Error() string {
	return "Detected cycle in dependency graph"
}

// Sort takes a map of from object to object dependencies and performs a topological sort,
// returning list of objects in dependency order.
func Sort(deps map[string][]string) ([][]string, error) {
	inDegrees, noIncoming := buildDegreeMap(deps)
	waves := [][]string{}

	for len(noIncoming) > 0 {
		currentWave := make([]string, len(noIncoming))
		copy(currentWave, noIncoming)
		noIncoming = []string{}
		waves = append(waves, currentWave)
		for _, current := range currentWave {
			for _, dep := range deps[current] {
				inDegrees[dep] = inDegrees[dep] - 1
				if inDegrees[dep] == 0 {
					noIncoming = append(noIncoming, dep)
					delete(inDegrees, dep)
				}
			}
		}
	}
	if countTotal(waves) != len(deps) {
		return [][]string{}, CycleDetectedError{}
	}

	// reverse slice
	for i, j := 0, len(waves)-1; i < j; i, j = i+1, j-1 {
		waves[i], waves[j] = waves[j], waves[i]
	}

	return waves, nil
}

func countTotal(nested [][]string) int {
	count := 0
	for _, i := range nested {
		count += len(i)
	}
	return count
}
