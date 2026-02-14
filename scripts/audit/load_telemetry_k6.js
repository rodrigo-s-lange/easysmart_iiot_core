import http from 'k6/http';
import { check, sleep } from 'k6';

const rate = Number(__ENV.RATE || 100);
const duration = __ENV.DURATION || '2m';

export const options = {
  scenarios: {
    telemetry_ingest: {
      executor: 'constant-arrival-rate',
      rate,
      timeUnit: '1s',
      duration,
      preAllocatedVUs: Number(__ENV.PRE_ALLOCATED_VUS || 50),
      maxVUs: Number(__ENV.MAX_VUS || 400),
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<500'],
  },
};

const apiBase = __ENV.API_BASE_URL || 'http://host.docker.internal:3001';
const apiKey = __ENV.API_KEY;
const tenantID = __ENV.TENANT_ID;
const deviceIDs = (__ENV.DEVICE_IDS || '').split(',').map((s) => s.trim()).filter(Boolean);
const singleDevice = __ENV.DEVICE_ID;

if (!apiKey || !tenantID || (deviceIDs.length === 0 && !singleDevice)) {
  throw new Error('API_KEY, TENANT_ID and DEVICE_ID or DEVICE_IDS are required');
}

export default function () {
  const now = Date.now();
  const slot = (__VU + __ITER) % 8;
  const deviceID = deviceIDs.length > 0 ? deviceIDs[(__VU + __ITER) % deviceIDs.length] : singleDevice;

  const payload = JSON.stringify({
    clientid: `k6-${__VU}`,
    topic: `tenants/${tenantID}/devices/${deviceID}/telemetry/slot/${slot}`,
    payload: { value: now % 100000 },
    timestamp: String(now),
  });

  const res = http.post(`${apiBase}/api/telemetry`, payload, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${apiKey}`,
    },
  });

  check(res, {
    'status is 200': (r) => r.status === 200,
  });

  sleep(0.01);
}
