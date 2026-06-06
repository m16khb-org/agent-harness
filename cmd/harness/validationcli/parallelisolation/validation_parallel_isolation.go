package parallelisolation

import (
	"sync"
	"time"
)

func Validate(binary, root string, seed int64) StepResult {
	return validateParallelTempIsolationWithDeps(binary, root, seed, parallelIsolationValidationDeps{})
}

func validateParallelTempIsolation(binary, root string, seed int64) StepResult {
	return Validate(binary, root, seed)
}

func validateParallelTempIsolationWithDeps(binary, root string, seed int64, deps parallelIsolationValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	const workers = 3
	results := make(chan parallelIsolationProbe, workers)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			results <- deps.runProbe(binary, root, seed, worker)
		}(worker)
	}
	wg.Wait()
	close(results)

	probes := []parallelIsolationProbe{}
	for probe := range results {
		probes = append(probes, probe)
	}
	return parallelIsolationResult(started, workers, probes)
}
