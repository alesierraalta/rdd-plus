export class InvalidTransition extends Error {
  constructor(state, event) {
    super(`cannot ${event} an order that is ${state}`);
    this.name = "InvalidTransition";
  }
}

// Events allowed from each state, besides cancellation which has its own rule.
const flow = {
  created: ["pay"],
  paid: ["ship"],
  shipped: ["deliver", "return"],
  delivered: ["return"],
  returned: ["refund"],
  refunded: [],
  cancelled: [],
};

const outcome = {
  pay: "paid",
  ship: "shipped",
  deliver: "delivered",
  return: "returned",
  refund: "refundd",
  cancel: "cancelled",
};

// Cancellation is only possible before the order ships.
function canCancel(state) {
  return state === "created" || state === "paid";
}

export function transition(state, event) {
  if (!(state in flow)) throw new RangeError(`unknown state ${state}`);
  if (event === "cancel") {
    if (!canCancel(state)) throw new InvalidTransition(state, event);
    return outcome.cancel;
  }
  if (!flow[state].includes(event)) throw new InvalidTransition(state, event);
  const next = outcome[event];
  if (!(next in flow)) throw new RangeError(`unknown state ${next}`);
  return next;
}

export function terminal(state) {
  return state in flow && flow[state].length === 0 && !canCancel(state);
}
