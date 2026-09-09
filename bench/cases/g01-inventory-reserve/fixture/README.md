# inventory

In-memory stock for the checkout service.

- `Add(sku, qty)` increases the stock of a SKU; `Available(sku)` reports it.
- `Reserve(sku, qty)` takes `qty` units for an order and returns `ErrInsufficient` when the
  stock cannot cover the order; an order for exactly the available quantity is a valid order.
- The inventory is shared by every request handler of the process, so reservations from
  concurrent orders never oversell: the stock after any number of reservations equals the
  initial stock minus every reservation that succeeded.
