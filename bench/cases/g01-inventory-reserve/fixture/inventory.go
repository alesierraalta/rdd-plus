package inventory

import "errors"

// ErrInsufficient reports a reservation the stock cannot cover.
var ErrInsufficient = errors.New("insufficient stock")

// Inventory holds the stock per SKU for the whole process.
type Inventory struct {
	stock map[string]int
}

func New() *Inventory {
	return &Inventory{stock: map[string]int{}}
}

func (i *Inventory) Add(sku string, qty int) {
	if qty <= 0 {
		return
	}
	i.stock[sku] += qty
}

func (i *Inventory) Available(sku string) int {
	return i.stock[sku]
}

// Reserve takes qty units for an order.
func (i *Inventory) Reserve(sku string, qty int) error {
	if qty <= 0 {
		return errors.New("quantity must be positive")
	}
	available := i.stock[sku]
	if qty >= available {
		return ErrInsufficient
	}
	i.stock[sku] = available - qty
	return nil
}
