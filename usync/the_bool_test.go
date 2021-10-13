package usync

import (
	"testing"
)

func TestAtomicBool(t *testing.T) {
	var b AtomicBool

	if b.IsSet() {
		t.Fatalf("should NOT be set")
	}
	if !b.SetUnlessSet() {
		t.Fatalf("should have set unset")
	}
	if b.SetUnlessSet() {
		t.Fatalf("should NOT have set already set")
	}
	if !b.IsSet() {
		t.Fatalf("should be set")
	}
	b.Clear()
	if b.IsSet() {
		t.Fatalf("should NOT be set")
	}
	b.Set()
	if !b.IsSet() {
		t.Fatalf("should be set")
	}
}

func TestAtomicBool32(t *testing.T) {
	var b AtomicBool32

	if b.IsSet() {
		t.Fatalf("should NOT be set")
	}
	if !b.SetUnlessSet() {
		t.Fatalf("should have set unset")
	}
	if b.SetUnlessSet() {
		t.Fatalf("should NOT have set already set")
	}
	if !b.IsSet() {
		t.Fatalf("should be set")
	}
	b.Clear()
	if b.IsSet() {
		t.Fatalf("should NOT be set")
	}
	b.Set()
	if !b.IsSet() {
		t.Fatalf("should be set")
	}
}

func TestAtomicBools(t *testing.T) {
	var b AtomicBools

	if 0 != b.GetAll() {
		t.Fatalf("no bits should be set")
	}
	for i := 0; i < 64; i++ {
		if b.IsSet(i) {
			t.Fatalf("no bit should be set")
		}
	}
	b.Set(5)
	if !b.IsSet(5) {
		t.Fatalf("bit 5 should be set")
	}
	b.Set(5)
	if !b.IsSet(5) {
		t.Fatalf("bit 5 should be set")
	}
	if b.SetUnlessSet(5) {
		t.Fatalf("should not be able to set bit 5 if aleady set (0x%x)", b.GetAll())
	}
	b.Clear(5)
	if b.IsSet(5) {
		t.Fatalf("bit 5 should be clear")
	}
	b.Clear(5)
	if b.IsSet(5) {
		t.Fatalf("bit 5 should be clear")
	}
	if !b.SetUnlessSet(5) {
		t.Fatalf("should be able to set bit 5 since not set")
	}
	if !b.IsSet(5) {
		t.Fatalf("bit 5 should be set")
	}
}

func TestAtomicBools32(t *testing.T) {
	var b AtomicBools32

	if 0 != b.GetAll() {
		t.Fatalf("no bits should be set")
	}
	for i := 0; i < 32; i++ {
		if b.IsSet(i) {
			t.Fatalf("no bit should be set")
		}
	}
	b.Set(5)
	if !b.IsSet(5) {
		t.Fatalf("bit 5 should be set")
	}
	b.Set(5)
	if !b.IsSet(5) {
		t.Fatalf("bit 5 should be set")
	}
	if b.SetUnlessSet(5) {
		t.Fatalf("should not be able to set bit 5 if aleady set (0x%x)", b.GetAll())
	}
	b.Clear(5)
	if b.IsSet(5) {
		t.Fatalf("bit 5 should be clear")
	}
	b.Clear(5)
	if b.IsSet(5) {
		t.Fatalf("bit 5 should be clear")
	}
	if !b.SetUnlessSet(5) {
		t.Fatalf("should be able to set bit 5 since not set")
	}
	if !b.IsSet(5) {
		t.Fatalf("bit 5 should be set")
	}
}
