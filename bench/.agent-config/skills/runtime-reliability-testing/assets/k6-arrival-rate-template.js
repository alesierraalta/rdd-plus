import http from 'k6/http';
import { check } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const customLatency = new Trend('target_custom_latency', true);
const errorRate = new Rate('target_error_rate');
const required = (name) => {
  if (!__ENV[name]) throw new Error(`${name} is required; set an explicit target-owned value`);
  return __ENV[name];
};
const number = (name, example) => Number(__ENV[name] || example);
const stages = __ENV.TARGET_STAGES
  ? JSON.parse(__ENV.TARGET_STAGES)
  : [{ duration: '1m', target: 100 }, { duration: '3m', target: 100 }, { duration: '30s', target: 0 }]; // EXAMPLE only
const TARGET_URL = required('TARGET_URL');
const REQUEST_TIMEOUT = __ENV.TARGET_TIMEOUT || '3s'; // EXAMPLE only

export const options = {
  scenarios: { open_model_workload: {
    executor: 'ramping-arrival-rate',
    startRate: number('TARGET_START_RATE', 20), // EXAMPLE only
    timeUnit: '1s',
    preAllocatedVUs: number('TARGET_PREALLOCATED_VUS', 50), // EXAMPLE only
    maxVUs: number('TARGET_MAX_VUS', 500), // EXAMPLE only
    stages,
  } },
  thresholds: {
    http_req_duration: [`p(95)<${number('TARGET_P95_MS', 200)}`, `p(99)<${number('TARGET_P99_MS', 400)}`],
    http_req_failed: [`rate<${number('TARGET_HTTP_ERROR_RATE', 0.005)}`],
    target_error_rate: [`rate<${number('TARGET_ERROR_RATE', 0.005)}`],
    dropped_iterations: ['count==0'], // Exhausted VUs makes the result INCONCLUSIVE.
  },
};

// EXAMPLE only: target owners may configure the marker or replace this hook with schema validation.
const RESPONSE_MARKER = __ENV.TARGET_RESPONSE_MARKER || 'ok'; // EXAMPLE only
export function validateResponse(res) {
  const body = typeof res.body === 'string' ? res.body : '';
  return (res.status === 200 || res.status === 201) && body.includes(RESPONSE_MARKER);
}

export default function () {
  const res = http.post(`${TARGET_URL}/api/v1/workload`, JSON.stringify({ timestamp: Date.now(), action: 'probe' }), {
    headers: { 'Content-Type': 'application/json' }, timeout: REQUEST_TIMEOUT,
  });
  customLatency.add(res.timings.duration);
  const ok = check(res, {
    'status and semantic response are valid': (r) => (r.status === 200 || r.status === 201) && validateResponse(r),
  });
  errorRate.add(!ok);
}

export function handleSummary(data) {
  if ((data.metrics.dropped_iterations?.values?.count || 0) > 0)
    console.warn('INCONCLUSIVE: dropped_iterations > 0; maxVUs was exhausted');
  return {};
}
