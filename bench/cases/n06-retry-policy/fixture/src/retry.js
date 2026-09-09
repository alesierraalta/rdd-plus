// Bounded retries with exponential backoff; 429 honors Retry-After.
export async function fetchWithRetry(fetchFn, url, { maxAttempts = 3, baseDelayMs = 100, sleep = defaultSleep } = {}) {
  let attempt = 0;
  for (;;) {
    attempt += 1;
    const res = await fetchFn(url);
    if (res.status >= 200 && res.status < 300) return { ok: true, data: res.body };
    if (res.status === 429) {
      const retryAfter = Number(res.headers?.["retry-after"]);
      await sleep(Number.isFinite(retryAfter) && retryAfter > 0 ? retryAfter * 1000 : baseDelayMs);
      continue;
    }
    if (res.status >= 400 && res.status < 500) return { ok: false, error: `http ${res.status}` };
    if (attempt < maxAttempts) {
      await sleep(baseDelayMs * 2 ** (attempt - 1));
      continue;
    }
    return { ok: true, data: null };
  }
}

function defaultSleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
