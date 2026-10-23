import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

export const checkoutErrors = new Rate('checkout_error_rate');
export const checkoutDuration = new Trend('checkout_duration_ms');

export const options = {
  stages: [
    { duration: '10s', target: 20 },  // Ramp-up
    { duration: '30s', target: 50 },  // Sustained checkout load
    { duration: '10s', target: 0 },   // Ramp-down
  ],
  thresholds: {
    checkout_error_rate: ['rate<0.02'],            // < 2% error rate
    checkout_duration_ms: ['p(95)<150', 'p(99)<300'], // 95% of checkouts under 150ms
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

function randomUUID() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

export default function () {
  const userEmail = `shopper_${__VU}_${__ITER}_${Date.now()}@quickcart.in`;
  const password = 'ShopperSecurePass123!';

  // 1. Register new customer
  const regPayload = JSON.stringify({
    email: userEmail,
    password: password,
    first_name: 'Quick',
    last_name: 'Shopper',
  });
  const regRes = http.post(`${BASE_URL}/api/v1/auth/register`, regPayload, {
    headers: { 'Content-Type': 'application/json' },
  });

  const regOk = check(regRes, {
    'registered successfully (201)': (r) => r.status === 201,
  });
  if (!regOk) {
    checkoutErrors.add(1);
    return;
  }

  const token = regRes.json('access_token');
  const authHeaders = {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  };

  // 2. Fetch dark stores
  const storesRes = http.get(`${BASE_URL}/api/v1/stores`);
  const stores = storesRes.json();
  if (!Array.isArray(stores) || stores.length === 0) {
    checkoutErrors.add(1);
    return;
  }
  const storeId = stores[0].id;

  // 3. Add item to cart
  const cartRes = http.post(
    `${BASE_URL}/api/v1/cart/items`,
    JSON.stringify({
      store_id: storeId,
      product_id: 'b2c3d4e5-f6a7-8b9c-0d1e-2f3a4b5c6d7e',
      quantity: 1,
    }),
    { headers: authHeaders }
  );

  // 4. Atomic Checkout with unique Idempotency-Key
  const idempotencyKey = randomUUID();
  const start = Date.now();
  const checkoutRes = http.post(
    `${BASE_URL}/api/v1/checkout`,
    JSON.stringify({
      store_id: storeId,
      delivery_address: '123 Koramangala 4th Block, Bengaluru, KA 560034',
    }),
    {
      headers: Object.assign({}, authHeaders, {
        'Idempotency-Key': idempotencyKey,
      }),
    }
  );
  checkoutDuration.add(Date.now() - start);

  const checkoutOk = check(checkoutRes, {
    'checkout created (201)': (r) => r.status === 201,
    'order id present': (r) => r.json('id') !== undefined,
  });
  checkoutErrors.add(!checkoutOk);

  if (checkoutOk) {
    const orderId = checkoutRes.json('id');
    // 5. Pay order
    http.post(
      `${BASE_URL}/api/v1/orders/${orderId}/pay`,
      JSON.stringify({ payment_method: 'UPI' }),
      { headers: authHeaders }
    );
  }

  sleep(1);
}
