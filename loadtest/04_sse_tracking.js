import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Counter } from 'k6/metrics';

export const sseErrors = new Rate('sse_error_rate');
export const sseConnected = new Counter('sse_connections_active');

export const options = {
  stages: [
    { duration: '10s', target: 20 },  // Ramp-up to 20 concurrent SSE streams
    { duration: '30s', target: 50 },  // Hold at 50 concurrent SSE streams
    { duration: '10s', target: 0 },   // Ramp-down
  ],
  thresholds: {
    sse_error_rate: ['rate<0.01'],    // < 1% error rate
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  const userEmail = `tracker_${__VU}_${__ITER}_${Date.now()}@quickcart.in`;

  // 1. Authenticate user
  const regRes = http.post(
    `${BASE_URL}/api/v1/auth/register`,
    JSON.stringify({
      email: userEmail,
      password: 'TrackerSecurePassword123!',
      first_name: 'SSE',
      last_name: 'Tracker',
    }),
    { headers: { 'Content-Type': 'application/json' } }
  );

  if (regRes.status !== 201) {
    sseErrors.add(1);
    return;
  }
  const token = regRes.json('access_token');
  const authHeaders = {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  };

  // 2. Create order to track
  const storesRes = http.get(`${BASE_URL}/api/v1/stores`);
  const stores = storesRes.json();
  if (!Array.isArray(stores) || stores.length === 0) {
    sseErrors.add(1);
    return;
  }
  const storeId = stores[0].id;

  http.post(
    `${BASE_URL}/api/v1/cart/items`,
    JSON.stringify({
      store_id: storeId,
      product_id: 'b2c3d4e5-f6a7-8b9c-0d1e-2f3a4b5c6d7e',
      quantity: 1,
    }),
    { headers: authHeaders }
  );

  const checkoutRes = http.post(
    `${BASE_URL}/api/v1/checkout`,
    JSON.stringify({
      store_id: storeId,
      delivery_address: '456 Indiranagar, Bengaluru, KA 560038',
    }),
    {
      headers: Object.assign({}, authHeaders, {
        'Idempotency-Key': `sse_test_key_${__VU}_${__ITER}_${Date.now()}`,
      }),
    }
  );

  if (checkoutRes.status !== 201) {
    sseErrors.add(1);
    return;
  }
  const orderId = checkoutRes.json('id');

  // 3. Connect to live SSE order tracking stream (simulating browser EventSource with query param token)
  const sseRes = http.get(
    `${BASE_URL}/api/v1/orders/${orderId}/events?token=${token}`,
    {
      headers: {
        Accept: 'text/event-stream',
      },
      timeout: '5s',
    }
  );

  const sseOk = check(sseRes, {
    'sse returned 200': (r) => r.status === 200,
    'content-type is event-stream': (r) =>
      r.headers['Content-Type'] && r.headers['Content-Type'].includes('text/event-stream'),
  });
  sseErrors.add(!sseOk);
  if (sseOk) {
    sseConnected.add(1);
  }

  sleep(1);
}
