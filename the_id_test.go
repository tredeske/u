package u

import "testing"

func TestId(t *testing.T) {
	b := NewIdBuilder()

	m := make(map[string]bool)

	for i := 0; i < 100000; i++ {
		id := b.NewId()
		_, exists := m[id]
		if exists {
			t.Fatalf("%d: duplicate ID (%s)generated!", i, id)
		}
		m[id] = true
	}
}
