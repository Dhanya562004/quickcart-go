import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

// Custom counters for zero-overselling verification
export const successfulOrders = new Counter('successful_orders');
export const inventoryConflicts = new Counter('inventory_conflicts');
export const unexpectedFailures = new Counter('unexpected_failures');

export const options = {
  scenarios: {
    oversell_burst: {
      executor: 'per-vu-iterations',
      vus: 1000,          // 1,000 concurrent Virtual Users
      iterations: 1,      // 1 checkout attempt per user
      maxDuration: '1m',  // Fast burst execution
    },
  },
  thresholds: {
    // Invariant: In an inventory of 10 items, exactly 10 orders must succeed
    successful_orders: ['count==10'],
    // Invariant: Exactly 990 requests must be rejected with 409 Conflict
    inventory_conflicts: ['count==990'],
    // Zero unexpected errors (e.g. 500s or timeouts)
    unexpected_failures: ['count==0'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const STORE_ID = __ENV.STORE_ID || 'a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d';
const FLASH_PRODUCT_ID = __ENV.PRODUCT_ID || 'c3d4e5f6-a7b8-9c0d-1e2f-3a4b5c6d7e8f';

function randomUUID() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

export function setup() {
  // Admin setup: Seed store inventory to exactly 10 available units before flash sale starts
  const adminTokenRes = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({
      email: 'admin@quickcart.in',
      password: 'AdminPassword123!',
    }),
    { headers: { 'Content-Type': 'application/json' } }
  );

  if (adminTokenRes.status === 200) {
    const adminToken = adminTokenRes.json('access_token');
    // Upsert inventory to exactly 10 available units
    http.post(
      `${BASE_URL}/api/v1/admin/stores/${STORE_ID}/inventory`,
      JSON.stringify({
        product_id: FLASH_PRODUCT_ID,
        available: 10,
      }),
      {
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${adminToken}`,
        },
      }
    );
  }
  return { ready: true };
}

export default function () {
  // 1. Each VU acts as a distinct authenticated customer
  const userEmail = `flash_user_${__VU}_${Date.now()}@quickcart.in`;
  const regRes = http.post(
    `${BASE_URL}/api/v1/auth/register`,
    JSON.stringify({
      email: userEmail,
      password: 'FlashCustomerPass123!',
      first_name: 'Flash',
      last_name: `Customer-${__VU}`,
    }),
    { headers: { 'Content-Type': 'application/json' } }
  );

  if (regRes.status !== 201) {
    unexpectedFailures.add(1);
    return;
  }

  const token = regRes.json('access_token');
  const authHeaders = {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  };

  // 2. Add 1 unit of limited flash product to cart
  const cartRes = http.post(
    `${BASE_URL}/api/v1/cart/items`,
    JSON.stringify({
      store_id: STORE_ID,
      product_id: FLASH_PRODUCT_ID,
      quantity: 1,
    }),
    { headers: authHeaders }
  );

  if (cartRes.status !== 200 && cartRes.status !== 201) {
    unexpectedFailures.add(1);
    return;
  }

  // 3. Compete in flash sale checkout with unique Idempotency-Key
  const idempotencyKey = `flash_order_vu_${__VU}_${randomUUID()}`;
  const checkoutRes = http.post(
    `${BASE_URL}/api/v1/checkout`,
    JSON.stringify({
      store_id: STORE_ID,
      delivery_address: `Apt ${__VU}, 12th Main, Indiranagar, Bengaluru, KA 560038`,
    }),
    {
      headers: Object.assign({}, authHeaders, {
        'Idempotency-Key': idempotencyKey,
      }),
    }
  );

  if (checkoutRes.status === 201) {
    // Successfully secured 1 of the 10 units!
    successfulOrders.add(1);
    check(checkoutRes, {
      'order placed successfully': (r) => r.status === 201,
      'order id present': (r) => r.json('id') !== undefined,
    });
  } else if (checkoutRes.status === 409) {
    // Correctly rejected: stock depleted
    inventoryConflicts.add(1);
    check(checkoutRes, {
      'insufficient stock conflict (409)': (r) => r.status === 409,
    });
  } else {
    // Unexpected error (e.g. 500 or timeout)
    unexpectedFailures.add(1);
  }
}

export function teardown() {
  console.log('Zero-Overselling Flash Sale Verification complete.');
}
