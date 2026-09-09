package inventory

import (
	"errors"
	"testing"
)

func TestReserveReducesStock(t *testing.T) {
	inv := New()
	inv.Add("sku-1", 10)
	if err := inv.Reserve("sku-1", 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := inv.Available("sku-1"); got != 7 {
		t.Fatalf("available = %d, want 7", got)
	}
}

func TestReserveMoreThanAvailable(t *testing.T) {
	inv := New()
	inv.Add("sku-1", 2)
	if err := inv.Reserve("sku-1", 5); !errors.Is(err, ErrInsufficient) {
		t.Fatalf("err = %v, want ErrInsufficient", err)
	}
	if got := inv.Available("sku-1"); got != 2 {
		t.Fatalf("stock changed on a failed reservation: %d", got)
	}
}

func TestSkusAreIndependent(t *testing.T) {
	inv := New()
	inv.Add("a", 5)
	inv.Add("b", 1)
	if err := inv.Reserve("a", 4); err != nil {
		t.Fatal(err)
	}
	if inv.Available("b") != 1 {
		t.Fatal("reserving a changed b")
	}
}

func TestRejectsNonPositiveQuantity(t *testing.T) {
	inv := New()
	inv.Add("a", 5)
	if err := inv.Reserve("a", 0); err == nil {
		t.Fatal("expected an error for qty 0")
	}
}
