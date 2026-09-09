# order-state

Lifecycle of a shop order.

- States: `created`, `paid`, `shipped`, `delivered`, `returned`, `refunded`, `cancelled`.
- `transition(state, event)` returns the next state or throws `InvalidTransition`.
- The normal path is `created → paid → shipped → delivered`; a delivered or shipped order may be
  `returned`, and a returned order is then `refunded`.
- An order can be cancelled only before it ships; once shipped it can no longer be cancelled.
- `terminal(state)` reports the states that admit no further event.
