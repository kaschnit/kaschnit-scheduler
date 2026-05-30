package cow_test

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaschnit/kaschnit-scheduler/internal/cow"
	"github.com/stretchr/testify/assert"
)

type testData struct {
	a          int
	b          string
	cloneCount int // verify the lazy execution boundaries
}

func (td *testData) Clone() *testData {
	return &testData{
		a:          td.a,
		b:          td.b,
		cloneCount: td.cloneCount + 1,
	}
}

func TestRCMapBasicLogic(t *testing.T) {
	t.Run("basic crud operations on single generation", func(t *testing.T) {
		rcm := cow.NewRCMap[string, *testData]()

		// Get on empty map
		val, found := rcm.Get("non-existent")
		assert.False(t, found)
		assert.Nil(t, val)

		// Put and immediate Get
		data := &testData{a: 42, b: "root"}
		rcm.Put("key1", data)

		// Because we used newDirectLazyClone, cloneCount should remain 0 on first write
		val, found = rcm.Get("key1")
		assert.True(t, found)
		assert.Equal(t, 42, val.a)
		assert.Equal(t, 0, val.cloneCount)

		// Update existing key
		rcm.Put("key1", &testData{a: 99, b: "updated"})
		val, _ = rcm.Get("key1")
		assert.Equal(t, 99, val.a)

		// Delete check
		assert.True(t, rcm.Delete("key1"))
		// Second delete is no-op
		assert.False(t, rcm.Delete("key1"))
	})
	t.Run("lazy evaluation boundary on read", func(t *testing.T) {
		rcm1 := cow.NewRCMap[string, *testData]()
		rcm1.Put("k1", &testData{a: 10, b: "init"})

		// Read from rcm1. We initially put k1 in rcm1, so it never gets cloned when reading rcm1.
		rcm1K1Val, _ := rcm1.Get("k1")
		assert.Equal(t, 0, rcm1K1Val.cloneCount)

		rcm2 := rcm1.Clone()

		// Write to rcm2 to force copy-on-write, but no deep clone of k1 yet.
		rcm2.Put("k2", &testData{a: 20, b: "unrelated"})

		// Read again from rcm1. We initially put k1 in rcm1, so it never gets cloned when reading rcm1.
		rcm1K1Val, _ = rcm1.Get("k1")
		assert.Equal(t, 0, rcm1K1Val.cloneCount)

		// Read from rcm2. It should perform its own independent clone from its current root reference point
		rcm2K1Val, _ := rcm2.Get("k1")
		assert.Equal(t, 1, rcm2K1Val.cloneCount) // It clones the original, so count goes 0 -> 1

		// Ensure memory spaces are isolated
		rcm1K1Val.a = 999
		assert.Equal(t, 10, rcm2K1Val.a, "Mutating data in generation 1 should not affect generation 2")
	})
	t.Run("clear breaks reference tracking properly", func(t *testing.T) {
		rcm1 := cow.NewRCMap[string, *testData]()
		rcm1.Put("k1", &testData{a: 1})

		rcm2 := rcm1.Clone()
		assert.Equal(t, int64(2), rcm1.RefCount())

		rcm1.Clear()

		// rcm1 should be totally blanked out
		assert.Equal(t, int64(1), rcm1.RefCount())
		v, found := rcm1.Get("k1")
		assert.False(t, found)
		assert.Nil(t, v)

		// rcm2 should remain completely intact with the historical snapshot data
		assert.Equal(t, int64(1), rcm2.RefCount())
		v2, found2 := rcm2.Get("k1")
		assert.True(t, found2)
		assert.Equal(t, 1, v2.a)
	})
	t.Run("iterator sequence validation and early loop termination", func(t *testing.T) {
		rcm := cow.NewRCMap[string, *testData]()
		rcm.Put("A", &testData{a: 1})
		rcm.Put("B", &testData{a: 2})
		rcm.Put("C", &testData{a: 3})

		// Test complete exhaustion loop
		count := 0
		for range rcm.All() {
			count++
		}
		assert.Equal(t, 3, count)
		assert.Equal(t, int64(1), rcm.RefCount(), "Ref count must return to normal after loop block finishes")

		// Test breaking out of loop early (simulates early termination inside seq generator)
		i := 0
		for range rcm.All() {
			if i == 1 {
				break
			}
			i++
		}
		assert.Equal(t, int64(1), rcm.RefCount(), "Ref count must be freed even if loop breaks early")
	})
	t.Run("clone and delete value", func(t *testing.T) {
		rcm1 := cow.NewRCMap[string, *testData]()
		rcm1.Put("empty", &testData{})
		rcm1.Put("value", &testData{
			a: 1,
			b: "hello",
		})
		// One reference to underlying map
		assert.Equal(t, int64(1), rcm1.RefCount())

		rcm2 := rcm1.Clone()
		// Both referencing same underlying map
		assert.Equal(t, int64(2), rcm1.RefCount())
		assert.Equal(t, int64(2), rcm2.RefCount())

		deleted := rcm2.Delete("value")
		assert.True(t, deleted)
		assert.Equal(t, int64(1), rcm1.RefCount())
		assert.Equal(t, int64(1), rcm2.RefCount())

		map1 := rcm1.ToMap()
		assert.Equal(t, map1, map[string]*testData{
			"empty": {},
			"value": {
				a: 1,
				b: "hello",
			},
		})

		map2 := rcm2.ToMap()
		assert.Equal(t, map2, map[string]*testData{
			"empty": {
				cloneCount: 1, // Was cloned when reading
			},
		})
	})
	t.Run("nested deep-cloning generations lineage", func(t *testing.T) {
		rcm1 := cow.NewRCMap[string, *testData]()
		rcm1.Put("quota", &testData{a: 100}) // Generation 0 (cloned: true)

		rcm2 := rcm1.Clone()               // RefCount = 2
		rcm2.Put("unrelated", &testData{}) // Forces map fork. "quota" forks to (cloned: false)

		rcm3 := rcm2.Clone()             // rcm2 and rcm3 now share the forked map. RefCount = 2
		rcm3.Put("another", &testData{}) // Forces another map fork. "quota" forks AGAIN to (cloned: false)

		// Read from Gen 3. It must execute exactly 1 clone from Gen 2's value state.
		v3, _ := rcm3.Get("quota")
		assert.Equal(t, 1, v3.cloneCount)

		// Read from Gen 2. It must execute its own clone independently.
		v2, _ := rcm2.Get("quota")
		assert.Equal(t, 1, v2.cloneCount)

		// Gen 1 remains completely un-cloned.
		v1, _ := rcm1.Get("quota")
		assert.Equal(t, 0, v1.cloneCount)
	})
	t.Run("iterator mutation isolation during execution", func(t *testing.T) {
		rcm := cow.NewRCMap[string, *testData]()
		rcm.Put("k1", &testData{a: 1})
		rcm.Put("k2", &testData{a: 2})

		// This simulates a background quota monitor modifying the primary map
		// while the scheduler cycle is actively looping over a snapshot iterator.
		for k, v := range rcm.All() {
			if k == "k1" {
				// Modify active map mid-iteration
				rcm.Put("k1", &testData{a: 999})
				// Delete structural key mid-iteration
				rcm.Delete("k2")
			}

			// The iterator stream must reflect the snapshot state from when All() was called.
			if k == "k1" {
				assert.Equal(t, 1, v.a, "Iterator value must remain isolated from concurrent Put")
			}
			if k == "k2" {
				assert.Equal(t, 2, v.a, "Iterator key must remain visible despite concurrent Delete")
			}
		}
	})
	t.Run("resurrected key structural isolation", func(t *testing.T) {
		rcm1 := cow.NewRCMap[string, *testData]()
		rcm1.Put("quota-X", &testData{a: 100, b: "v1"})

		rcm2 := rcm1.Clone()

		// 1. Delete forcing structural fork
		assert.True(t, rcm2.Delete("quota-X"))

		// 2. Put the same key back instantly with brand new properties
		rcm2.Put("quota-X", &testData{a: 999, b: "resurrected"})

		// 3. Audit structural isolation
		v1, found1 := rcm1.Get("quota-X")
		assert.True(t, found1)
		assert.Equal(t, 100, v1.a, "Historical generation value was corrupted by key resurrection")
		assert.Equal(t, 0, v1.cloneCount)

		v2, found2 := rcm2.Get("quota-X")
		assert.True(t, found2)
		assert.Equal(t, 999, v2.a)
		assert.Equal(t, 0, v2.cloneCount)
	})
	t.Run("mid-flight iterator snapshotting race", func(t *testing.T) {
		t.Parallel()

		rcm := cow.NewRCMap[string, *testData]()
		for i := range 100 {
			rcm.Put(fmt.Sprintf("k-%d", i), &testData{a: i})
		}

		var wg sync.WaitGroup
		var stopSignal int32

		// Worker Group A: Constantly spinning up iterators and reading values
		for range 5 {
			wg.Go(func() {
				for atomic.LoadInt32(&stopSignal) == 0 {
					for _, v := range rcm.All() {
						_ = v.a // Force evaluation execution
					}
				}
			})
		}

		// Worker Group B: Constantly snapshotting/cloning and clearing handles
		for c := 0; c < 3; c++ {
			wg.Go(func() {
				for atomic.LoadInt32(&stopSignal) == 0 {
					clonedHandle := rcm.Clone()
					// Do a quick operation on the clone
					_, _ = clonedHandle.Get("k-50")
					clonedHandle.Clear()
				}
			})
		}

		time.Sleep(100 * time.Millisecond)
		atomic.StoreInt32(&stopSignal, 1)
		wg.Wait()
	})
}

func TestRCMapKitchenSink(t *testing.T) {
	t.Run("complex lineage generational mutations and isolation", func(t *testing.T) {
		// We will maintain an array of snapshots to track historical states
		const generations = 20
		snapshots := make([]*cow.RCMap[string, *testData], generations)

		// Create the root map (Generation 0)
		rcm := cow.NewRCMap[string, *testData]()
		snapshots[0] = rcm

		// Seed the root map with core tracking keys
		rcm.Put("global-limit", &testData{a: 1000, b: "root"})
		rcm.Put("user-quota-A", &testData{a: 50, b: "root"})
		rcm.Put("user-quota-B", &testData{a: 100, b: "root"})

		// 1. Lineage Building Phase:
		// Linearly loop through creating new generations, mutating some keys,
		// adding new ones, and deleting others to simulate intense scheduler cycles.
		for i := 1; i < generations; i++ {
			// Take a snapshot of the previous generation
			snapshots[i] = snapshots[i-1].Clone()

			// Mutate an existing key in the new generation handle.
			// Because RefCount > 1, this forces a map fork, setting 'cloned: false' for items.
			snapshots[i].Put("global-limit", &testData{a: 1000 + i, b: fmt.Sprintf("gen-%d", i)})

			// Lazily mutate a user quota every 3 generations
			if i%3 == 0 {
				snapshots[i].Put("user-quota-A", &testData{a: 50 + i, b: fmt.Sprintf("gen-%d", i)})
			}

			// Introduce a generation-specific ephemeral key
			snapshots[i].Put(fmt.Sprintf("ephemeral-%d", i), &testData{a: i, b: "temp"})

			// Delete the previous generation's ephemeral key to keep map structures distinct
			if i > 1 {
				snapshots[i].Delete(fmt.Sprintf("ephemeral-%d", i-1))
			}
		}

		// 2. Comprehensive Validation Phase:
		// Now we walk backward and forward across historical snapshots to verify
		// that lazy deep-clones correctly trigger, historical data remains completely
		// uncorrupted, and cross-talk is impossible.

		// Assert Generation 0 remains pristine
		vMin, _ := snapshots[0].Get("global-limit")
		assert.Equal(t, 1000, vMin.a)
		assert.Equal(t, "root", vMin.b)
		assert.Equal(t, 0, vMin.cloneCount, "Gen 0 should never have been cloned on read")

		// Assert downstream generation values are structurally sound
		for i := 1; i < generations; i++ {
			vLimit, found := snapshots[i].Get("global-limit")
			assert.True(t, found)
			assert.Equal(t, 1000+i, vLimit.a)
			assert.Equal(t, fmt.Sprintf("gen-%d", i), vLimit.b)

			// Because snapshots[i] was created via Clone and then modified via Put,
			// the "global-limit" key was explicitly overwritten via Put(newDirectLazyClone).
			// Therefore, reading it should trigger 0 lazy clones!
			assert.Equal(t, 0, vLimit.cloneCount)

			// Validate ephemeral isolation boundaries
			_, currentEphemeralFound := snapshots[i].Get(fmt.Sprintf("ephemeral-%d", i))
			assert.True(t, currentEphemeralFound, "Generation %d must have its own ephemeral key", i)

			if i > 1 {
				_, oldEphemeralFound := snapshots[i].Get(fmt.Sprintf("ephemeral-%d", i-1))
				assert.False(t, oldEphemeralFound, "Generation %d must have successfully deleted the previous ephemeral key", i)
			}
		}

		// 3. Lazy Trigger Deep-Validation (The Interleaved Read Phase)
		// Let's look closely at "user-quota-B". We put it in Generation 0, and NEVER explicitly
		// updated it via Put in any subsequent generation. It has cascaded down through 20 map forks.

		// Read it from Generation 15. This should trigger EXACTLY 1 deep-clone from its root pointer state.
		v15, _ := snapshots[15].Get("user-quota-B")
		assert.Equal(t, 100, v15.a)
		assert.Equal(t, 1, v15.cloneCount, "Gen 15 read should lazy-clone the root value exactly once")

		// Mutate Gen 15's lazy clone locally
		v15.a = 5555

		// Read it from Generation 19. It should perform its own independent 1-step clone from the root pointer,
		// completely unaffected by Gen 15's mutation.
		v19, _ := snapshots[19].Get("user-quota-B")
		assert.Equal(t, 100, v19.a, "Gen 19 must be completely isolated from Gen 15 mutations")
		assert.Equal(t, 1, v19.cloneCount)

		// Read it from Generation 0. It must still be the original un-cloned root data.
		vRoot, _ := snapshots[0].Get("user-quota-B")
		assert.Equal(t, 100, vRoot.a)
		assert.Equal(t, 0, vRoot.cloneCount)

		// 4. Cleanup Phase:
		// Clear out generations one by one and ensure reference counts balance perfectly back to 0.
		for i := generations - 1; i >= 0; i-- {
			snapshots[i].Clear()
			assert.Equal(t, int64(1), snapshots[i].RefCount(), "Cleared generation handle must reset to 1 internal ref count")
		}
	})
}

func TestRCMapConcurrency(t *testing.T) {
	t.Run("concurrent read-read contention on shared un-cloned wrappers", func(t *testing.T) {
		t.Parallel()

		// Scenario: Dozens of workers read from different snapshot handles that
		// share the exact same un-cloned lazyClone pointer. This fiercely stress-tests
		// lazyClone.lock serialization when cloned == false.
		rcmRoot := cow.NewRCMap[string, *testData]()
		rcmRoot.Put("quota-key", &testData{a: 5000})

		const workerCount = 50
		clones := make([]*cow.RCMap[string, *testData], workerCount)
		for i := range workerCount {
			clones[i] = rcmRoot.Clone()
		}

		var wg sync.WaitGroup
		wg.Add(workerCount)

		// Start all read operations at roughly the same time
		for i := range workerCount {
			go func(idx int) {
				defer wg.Done()
				val, found := clones[idx].Get("quota-key")
				if assert.True(t, found) {
					assert.Equal(t, 5000, val.a)
				}
			}(i)
		}
		wg.Wait()

		// The root element must have been safely mutated to cloned: true by exactly ONE
		// of the threads, and the total clone count across all generations must match
		// the lazy expectations.
		rootVal, _ := rcmRoot.Get("quota-key")
		assert.Equal(t, 0, rootVal.cloneCount, "Root map data should remain un-cloned on read")

		for i := range workerCount {
			clones[i].Clear()
		}
	})
	t.Run("fork vs get interleaved execution race", func(t *testing.T) {
		t.Parallel()

		// Scenario: A heavy master writer constantly triggers COW forks via Put/Delete
		// on one handle, while multiple background readers aggressively call Get()
		// on a separate snapshot generation handle.
		rcmActive := cow.NewRCMap[string, *testData]()
		for i := range 100 {
			rcmActive.Put(fmt.Sprintf("key-%d", i), &testData{a: i})
		}

		snapshot := rcmActive.Clone() // Freeze generation state

		var stopSignal atomic.Int32
		var wg sync.WaitGroup

		// 1. Launch Reader Group reading strictly from the static snapshot
		const readerCount = 5
		wg.Add(readerCount)
		for r := range readerCount {
			go func(readerID int) {
				defer wg.Done()
				for stopSignal.Load() == 0 {
					// Read across keys unpredictably to force random lazy-clone evaluations
					targetKey := fmt.Sprintf("key-%d", (readerID*17)%100)
					val, found := snapshot.Get(targetKey)
					if found {
						// Ensure data isolation: values must never be corrupted by active writer updates
						assert.True(t, val.a >= 0 && val.a < 100)
					}
				}
			}(r)
		}

		// 2. Active writer pipeline constantly hammering map mutations
		wg.Go(func() {
			for iteration := range 200 {
				// Alternating writes and deletes triggers continuous map structural forks
				rcmActive.Put(fmt.Sprintf("new-key-%d", iteration), &testData{a: iteration})
				rcmActive.Delete(fmt.Sprintf("key-%d", iteration%100))
			}
		})

		// Let the chaos run for a moment
		time.Sleep(50 * time.Millisecond)
		stopSignal.Store(1)
		wg.Wait()

		// Clean up snapshots
		snapshot.Clear()
	})
	t.Run("heavy concurrent iterator registration and cleanup", func(t *testing.T) {
		t.Parallel()

		// Scenario: Multiple goroutines continuously spin up All() iterators,
		// partly exhaust them, and break early. This verifies rcm.shared.rc reference
		// incrementing/decrementing remains perfectly bounded without race leaks.
		rcm := cow.NewRCMap[string, *testData]()
		for i := range 50 {
			rcm.Put(fmt.Sprintf("k-%d", i), &testData{a: i})
		}

		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() {
				for range 100 {
					// Invoke the iterator closure sequence
					iterCount := 0
					for range rcm.All() {
						iterCount++
						if iterCount == 5 {
							break // Crucial early break to stress defer cowClone.Clear()
						}
					}
				}
			})
		}

		wg.Wait()
		// If reference counts leaked, RefCount would be > 1
		assert.Equal(t, int64(1), rcm.RefCount(), "Reference count must return to exactly 1 after all iterators dissolve")
	})
	t.Run("brute force multi-method race profile", func(t *testing.T) {
		t.Parallel()

		// Scenario: A chaotic combination of every map operation executing concurrently
		// on the same base pointer instance to check general structural thread-safety.
		rcm := cow.NewRCMap[string, *testData]()

		var wg sync.WaitGroup
		const operationsPerWorker = 500

		// Writer 1 (Puts)
		wg.Go(func() {
			for i := range operationsPerWorker {
				rcm.Put(fmt.Sprintf("race-key-%d", i%50), &testData{a: i})
			}
		})

		// Writer 2 (Deletes)
		wg.Go(func() {
			for i := range operationsPerWorker {
				rcm.Delete(fmt.Sprintf("race-key-%d", i%50))
			}
		})

		// Readers
		for range 4 {
			wg.Go(func() {
				for i := range operationsPerWorker {
					_, _ = rcm.Get(fmt.Sprintf("race-key-%d", i%50))
					_ = rcm.RefCount()
					_ = rcm.ToMap()
				}
			})
		}

		wg.Wait()
	})
	t.Run("chaotic multi-generation snapshot and read stress", func(t *testing.T) {
		t.Parallel()

		const (
			maxGenerations = 30
			readersPerGen  = 10
		)

		// An array holding active historical snapshot generations as they are created
		var generations sync.Map // Map[int]*cow.RCMap[string, *testData]

		// Seed the root map (Gen 0)
		rcmRoot := cow.NewRCMap[string, *testData]()
		rcmRoot.Put("global-limit", &testData{a: 1000, b: "gen-0"})
		rcmRoot.Put("shared-quota", &testData{a: 500, b: "gen-0"})
		generations.Store(0, rcmRoot)

		var wg sync.WaitGroup
		var stopSignal int32
		var activeGenIdx int32 // Monotonically increasing generation index

		// 1. PIPELINE WORKER: Active Lineage Creator
		// Simulates the scheduler advancing cycles, taking a snapshot of the previous cycle,
		// and mutating configuration limits/ephemeral states.
		wg.Go(func() {
			// Seed a source map pointer
			currentSource := rcmRoot

			for gen := 1; gen < maxGenerations; gen++ {
				if atomic.LoadInt32(&stopSignal) == 1 {
					return
				}

				// Create the next snapshot layer
				nextGenMap := currentSource.Clone()

				// Apply mutations unique to this generation handle
				nextGenMap.Put("global-limit", &testData{a: 1000 + gen, b: fmt.Sprintf("gen-%d", gen)})
				nextGenMap.Put(fmt.Sprintf("ephemeral-%d", gen), &testData{a: gen, b: "temp"})

				// Expose this generation to the reading pool
				generations.Store(gen, nextGenMap)
				atomic.StoreInt32(&activeGenIdx, int32(gen))

				// Move pointer forward for next nested clone loop iteration
				currentSource = nextGenMap

				// Control generation pacing (forces readers to span old and new handles concurrently)
				time.Sleep(10 * time.Millisecond)
			}
		})

		// Wait slightly to let at least 2 generations spawn before setting readers loose
		time.Sleep(5 * time.Millisecond)

		// 2. PIPELINE WORKERS: Reader Pool (Hammering different generations)
		// Spawns readers assigned to random active historical generations. They aggressively
		// verify cross-talk isolation while elements are fork-cloned underneath them.
		totalReaders := maxGenerations * readersPerGen
		wg.Add(totalReaders)

		for i := range totalReaders {
			go func(workerID int) {
				defer wg.Done()

				r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

				for atomic.LoadInt32(&stopSignal) == 0 {
					currentMax := atomic.LoadInt32(&activeGenIdx)
					if currentMax == 0 {
						continue
					}

					// Pick a random generation currently alive in the system
					targetGen := r.Intn(int(currentMax) + 1)

					genVal, ok := generations.Load(targetGen)
					if !ok {
						continue
					}
					targetMap := genVal.(*cow.RCMap[string, *testData])

					// Action A: Read the heavily overwritten key
					limit, found := targetMap.Get("global-limit")
					if found {
						assert.Equal(t, 1000+targetGen, limit.a)
						assert.Equal(t, fmt.Sprintf("gen-%d", targetGen), limit.b)
					}

					// Action B: Read the untouched cascaded key ("shared-quota")
					// This key cascades through all generations un-mutated. Reading it triggers
					// lazy evaluations across multiple threads simultaneously on shared forked wrappers.
					quota, foundQuota := targetMap.Get("shared-quota")
					if foundQuota {
						assert.Equal(t, 500, quota.a)
					}

					// Action C: Read ephemeral keys
					_, foundEphem := targetMap.Get(fmt.Sprintf("ephemeral-%d", targetGen))
					if targetGen > 0 {
						assert.True(t, foundEphem, "Gen %d must have its own ephemeral key visible", targetGen)
					}
				}
			}(i)
		}

		// Let the structural chaos run to completion
		time.Sleep(150 * time.Millisecond)
		atomic.StoreInt32(&stopSignal, 1)
		wg.Wait()

		// 3. Post-Chaos Deterministic Audits
		// Clean up handles completely and assert reference counts drop safely back to 1.
		for i := 0; i < maxGenerations; i++ {
			if val, ok := generations.Load(i); ok {
				m := val.(*cow.RCMap[string, *testData])
				m.Clear()
				assert.Equal(t, int64(1), m.RefCount(), "Generation %d leaked reference tracking counts", i)
			}
		}
	})
}
