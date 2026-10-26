import { apiClient } from './client';
import { CancelOrderRequest, CheckoutRequest, Order } from './types';

export async function checkout(
  idempotencyKey: string,
  data: CheckoutRequest
): Promise<Order> {
  const res = await apiClient.post<Order>('/api/v1/checkout', data, {
    headers: {
      'Idempotency-Key': idempotencyKey,
    },
  });
  return res.data;
}

export async function listOrders(
  limit: number = 20,
  offset: number = 0
): Promise<Order[]> {
  const res = await apiClient.get<Order[]>('/api/v1/orders', {
    params: { limit, offset },
  });
  return res.data || [];
}

export async function getOrder(id: string): Promise<Order> {
  const res = await apiClient.get<Order>(`/api/v1/orders/${id}`);
  return res.data;
}

export async function cancelOrder(
  id: string,
  reason: string
): Promise<Order> {
  const payload: CancelOrderRequest = { reason };
  const res = await apiClient.post<Order>(`/api/v1/orders/${id}/cancel`, payload);
  return res.data;
}
