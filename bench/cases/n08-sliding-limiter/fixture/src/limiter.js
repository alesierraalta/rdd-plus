// At most `limit` requests per identifier within any sliding window of `windowMs`.
export function createLimiter({ limit, windowMs, now = () => Date.now() }) {
  if (!Number.isInteger(limit) || limit < 1) throw new RangeError("limit must be a positive integer");
  const hits = new Map();

  function normalize(id) {
    if (typeof id !== "string" || !id.includes(":")) throw new TypeError("id must be kind:value");
    return id.startsWith("user:") ? id.toLowerCase() : id;
  }

  return {
    allow(id) {
      const key = normalize(id);
      const t = now();
      const recent = (hits.get(key) || []).filter((ts) => t - ts < windowMs);
      if (recent.length > limit) {
        hits.set(key, recent);
        return false;
      }
      recent.push(t);
      hits.set(key, recent);
      return true;
    },
    remaining(id) {
      const key = normalize(id);
      const t = now();
      const recent = (hits.get(key) || []).filter((ts) => t - ts < windowMs);
      return Math.max(0, limit - recent.length);
    },
  };
}
