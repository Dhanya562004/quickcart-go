import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
export const errorRate = new Rate('browse_error_rate');
export const catalogDuration = new Trend('browse_catalog_duration_ms');

export const options = {
  stages: [
    { duration: '15s', target: 50 },  // Ramp-up to 50 VUs
    { duration: '30s', target: 100 }, // Hold at 100 VUs (read burst)
    { duration: '15s', target: 0 },   // Ramp-down
  ],
  thresholds: {
    browse_error_rate: ['rate<0.01'],         // Error rate must remain under 1%
    http_req_duration: ['p(95)<50', 'p(99)<100'], // 95% of requests under 50ms (Redis cache-aside)
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  // 1. Fetch available dark stores
  const storesRes = http.get(`${BASE_URL}/api/v1/stores`);
  const storesOk = check(storesRes, {
    'stores returned 200': (r) => r.status === 200,
    'stores list is array': (r) => Array.isArray(r.json()),
  });
  errorRate.add(!storesOk);

  let storeId = 'a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d';
  const stores = storesRes.json();
  if (Array.isArray(stores) && stores.length > 0) {
    storeId = stores[0].id;
  }

  // 2. Browse products in store (leveraging Redis cache-aside)
  const start = Date.now();
  const productsRes = http.get(`${BASE_URL}/api/v1/stores/${storeId}/products`);
  catalogDuration.add(Date.now() - start);

  const productsOk = check(productsRes, {
    'products returned 200': (r) => r.status === 200,
    'products array returned': (r) => Array.isArray(r.json()),
  });
  errorRate.add(!productsOk);

  sleep(0.5);
}
