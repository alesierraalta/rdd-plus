# stock-allocate

Allocates available stock to requests in order.

- `allocate(stock, requests)` skips zero and negative requests, grants each remaining request up to the stock left, and returns the granted amounts with the stock left over.
- A request larger than the available stock receives only what remains, and the remaining stock never becomes negative.
