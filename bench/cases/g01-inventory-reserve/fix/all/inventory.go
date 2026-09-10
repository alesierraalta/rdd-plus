package inventory

import (
	"errors"
	"sync"
)

// ErrInsufficient reports a reservation the stock cannot cover.
var ErrInsufficient = errors.New("insufficient stock")

// Inventory holds the stock per SKU for the whole process.
type Inventory struct {
	mu    sync.Mutex
	stock map[string]int
}

func New() *Inventory {
	return &Inventory{stock: map[string]int{}}
}

func (i *Inventory) Add(sku string, qty int) {
	if qty <= 0 {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.stock[sku] += qty
}

func (i *Inventory) Available(sku string) int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.stock[sku]
}

// Reserve takes qty units for an order; the check and the decrement happen under one lock.
func (i *Inventory) Reserve(sku string, qty int) error {
	if qty <= 0 {
		return errors.New("quantity must be positive")
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	available := i.stock[sku]
	if qty > available {
		return ErrInsufficient
	}
	i.stock[sku] = available - qty
	return nil
}
