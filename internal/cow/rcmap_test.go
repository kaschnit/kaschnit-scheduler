package cow_test

import (
	"testing"

	"github.com/kaschnit/kaschnit-scheduler/internal/cow"
	"github.com/stretchr/testify/assert"
)

type testData struct {
	a int
	b string
}

func TestRCMap(t *testing.T) {
	t.Run("clone and delete value", func(t *testing.T) {
		rcm1 := cow.NewRCMap[string, testData]()
		rcm1.Put("empty", testData{})
		rcm1.Put("value", testData{
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
		assert.Equal(t, map1, map[string]testData{
			"empty": {},
			"value": {
				a: 1,
				b: "hello",
			},
		})

		map2 := rcm2.ToMap()
		assert.Equal(t, map2, map[string]testData{
			"empty": {},
		})
	})
}
