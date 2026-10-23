# QuickCart — k6 Load Testing Suite

This directory contains the production-grade [k6](https://k6.io/) performance and concurrency test suites for QuickCart, designed to stress-test the 3-instance Go API cluster behind Nginx.

---

## Scenarios Overview

| Scenario | File | Description | Target Thresholds |
| :--- | :--- | :--- | :--- |
| **1. Catalog Browsing** | `01_browse_catalog.js` | High-frequency read traffic testing Redis cache-aside hit ratio | p95 < 50ms, error < 1% |
| **2. Cart & Checkout** | `02_cart_checkout.js` | Complete customer flow: register, add to cart, atomic checkout, pay | p95 < 150ms, error < 2% |
| **3. Flash Sale Oversell** | `03_oversell_headline.js` | **1,000 concurrent VUs competing for 10 units** across 3 API replicas | Exactly 10 succeed, 990 conflict (409) |
| **4. SSE Live Tracking** | `04_sse_tracking.js` | Concurrent streaming EventSource connections through Nginx | p95 < 100ms, error < 1% |

---

## Prerequisites

Install k6:
- **macOS**: `brew install k6`
- **Linux (Debian/Ubuntu)**: `sudo gpg -k && sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D34EEF1D && echo "deb https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list && sudo apt-get update && sudo apt-get install k6`
- **Windows**: `winget install k6` or `choco install k6`
- **Docker**: `docker run --rm -i grafana/k6 run - <loadtest/01_browse_catalog.js`

---

## Running the Tests

Ensure the QuickCart stack is running via Docker Compose:
```bash
docker compose -f deployments/docker-compose.yml up -d
```

### 1. Catalog Browsing
```bash
k6 run loadtest/01_browse_catalog.js
```

### 2. Cart & Checkout Pipeline
```bash
k6 run loadtest/02_cart_checkout.js
```

### 3. Flash Sale Zero-Overselling Concurrency Test
```bash
k6 run loadtest/03_oversell_headline.js
```

### 4. SSE Real-Time Tracking Streams
```bash
k6 run loadtest/04_sse_tracking.js
```

---

## PostgreSQL Verification Commands

After running the flash sale oversell scenario (`03_oversell_headline.js`), execute this query to verify zero overselling directly in the database:

```sql
-- 1. Verify available and reserved stock in store inventory
SELECT product_id, available, reserved
FROM store_inventories
WHERE store_id = 'a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d'
  AND product_id = 'c3d4e5f6-a7b8-9c0d-1e2f-3a4b5c6d7e8f';
-- Expected: available = 0, reserved = 10

-- 2. Verify total successful orders for this product
SELECT COUNT(*) AS total_successful_orders
FROM order_items oi
JOIN orders o ON o.id = oi.order_id
WHERE oi.product_id = 'c3d4e5f6-a7b8-9c0d-1e2f-3a4b5c6d7e8f'
  AND o.status = 'PLACED';
-- Expected: 10

-- 3. Verify exactly 10 active inventory reservations
SELECT COUNT(*) AS total_reservations, status
FROM inventory_reservations
WHERE product_id = 'c3d4e5f6-a7b8-9c0d-1e2f-3a4b5c6d7e8f'
GROUP BY status;
-- Expected: status = 'RESERVED', count = 10
```
