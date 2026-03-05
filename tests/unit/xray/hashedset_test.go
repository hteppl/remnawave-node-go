package xray_test

import (
	"testing"

	"github.com/hteppl/remnawave-node-go/internal/xray"
)

func TestHashedSet_EmptyHash(t *testing.T) {
	set := xray.NewHashedSet()

	expected := "0000000000000000"
	if got := set.Hash64String(); got != expected {
		t.Errorf("Empty set hash = %q, want %q", got, expected)
	}
}

func TestHashedSet_AddSingleItem(t *testing.T) {
	set := xray.NewHashedSet()
	set.Add("test")

	if set.Hash64String() == "0000000000000000" {
		t.Error("Single item should produce non-zero hash")
	}

	// Size should be 1
	if set.Size() != 1 {
		t.Errorf("Size = %d, want 1", set.Size())
	}
}

func TestHashedSet_OrderIndependence(t *testing.T) {
	set1 := xray.NewHashedSet()
	set1.Add("a")
	set1.Add("b")
	set1.Add("c")

	set2 := xray.NewHashedSet()
	set2.Add("c")
	set2.Add("b")
	set2.Add("a")

	set3 := xray.NewHashedSet()
	set3.Add("b")
	set3.Add("a")
	set3.Add("c")

	hash1 := set1.Hash64String()
	hash2 := set2.Hash64String()
	hash3 := set3.Hash64String()

	if hash1 != hash2 {
		t.Errorf("Order independence failed: {a,b,c}=%s, {c,b,a}=%s", hash1, hash2)
	}
	if hash1 != hash3 {
		t.Errorf("Order independence failed: {a,b,c}=%s, {b,a,c}=%s", hash1, hash3)
	}
}

func TestHashedSet_SelfInverse(t *testing.T) {
	set := xray.NewHashedSet()

	set.Add("test-item")
	if set.Hash64String() == "0000000000000000" {
		t.Error("After add, hash should not be zero")
	}

	set.Delete("test-item")
	if set.Hash64String() != "0000000000000000" {
		t.Errorf("After add+delete, hash = %s, want 0000000000000000", set.Hash64String())
	}
}

func TestHashedSet_MultiItemSelfInverse(t *testing.T) {
	set := xray.NewHashedSet()

	set.Add("first")
	set.Add("second")
	set.Add("third")

	hashWithThree := set.Hash64String()

	set.Delete("second")
	set.Delete("first")
	set.Delete("third")

	if set.Hash64String() != "0000000000000000" {
		t.Errorf("After adding and deleting all items, hash = %s, want 0000000000000000", set.Hash64String())
	}

	set.Add("third")
	set.Add("first")
	set.Add("second")

	if set.Hash64String() != hashWithThree {
		t.Errorf("Re-adding items gave different hash: got %s, want %s", set.Hash64String(), hashWithThree)
	}
}

func TestHashedSet_DuplicateAdd(t *testing.T) {
	set := xray.NewHashedSet()

	set.Add("item")
	hash1 := set.Hash64String()
	size1 := set.Size()

	set.Add("item")
	hash2 := set.Hash64String()
	size2 := set.Size()

	if hash1 != hash2 {
		t.Errorf("Duplicate add changed hash: %s -> %s", hash1, hash2)
	}
	if size1 != size2 {
		t.Errorf("Duplicate add changed size: %d -> %d", size1, size2)
	}
}

func TestHashedSet_DeleteNonexistent(t *testing.T) {
	set := xray.NewHashedSet()

	set.Add("existing")
	hash1 := set.Hash64String()

	set.Delete("nonexistent")
	hash2 := set.Hash64String()

	if hash1 != hash2 {
		t.Errorf("Delete nonexistent changed hash: %s -> %s", hash1, hash2)
	}
}

func TestHashedSet_Has(t *testing.T) {
	set := xray.NewHashedSet()

	if set.Has("item") {
		t.Error("Empty set should not have item")
	}

	set.Add("item")
	if !set.Has("item") {
		t.Error("Set should have added item")
	}

	set.Delete("item")
	if set.Has("item") {
		t.Error("Set should not have deleted item")
	}
}

func TestHashedSet_Clear(t *testing.T) {
	set := xray.NewHashedSet()

	set.Add("a")
	set.Add("b")
	set.Add("c")

	set.Clear()

	if set.Size() != 0 {
		t.Errorf("After clear, size = %d, want 0", set.Size())
	}
	if set.Hash64String() != "0000000000000000" {
		t.Errorf("After clear, hash = %s, want 0000000000000000", set.Hash64String())
	}
}

func TestHashedSet_UUIDs(t *testing.T) {
	set := xray.NewHashedSet()

	uuids := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"f47ac10b-58cc-4372-a567-0e02b2c3d479",
	}

	for _, uuid := range uuids {
		set.Add(uuid)
	}

	if set.Size() != 3 {
		t.Errorf("Size = %d, want 3", set.Size())
	}

	hash1 := set.Hash64String()

	set2 := xray.NewHashedSet()
	for i := len(uuids) - 1; i >= 0; i-- {
		set2.Add(uuids[i])
	}

	if set2.Hash64String() != hash1 {
		t.Errorf("UUID hash not order-independent: %s != %s", hash1, set2.Hash64String())
	}
}

func TestHashedSet_HashFormat(t *testing.T) {
	set := xray.NewHashedSet()
	set.Add("test")

	hash := set.Hash64String()

	if len(hash) != 16 {
		t.Errorf("Hash length = %d, want 16", len(hash))
	}

	// Should be lowercase hex
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Hash contains non-hex character: %c", c)
		}
	}
}

func TestDjb2Dual(t *testing.T) {
	// Test the hash function directly
	high, low := xray.Djb2Dual("test")

	if high == 0 && low == 0 {
		t.Error("Djb2Dual should not return zeros for non-empty string")
	}

	high2, low2 := xray.Djb2Dual("test")
	if high != high2 || low != low2 {
		t.Error("Djb2Dual should be deterministic")
	}

	high3, low3 := xray.Djb2Dual("different")
	if high == high3 && low == low3 {
		t.Error("Djb2Dual should give different values for different inputs")
	}
}
