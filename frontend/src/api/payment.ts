import { apiClient } from './client';
import { Payment, PayOrderRequest } from './types';

export async function payOrder(
  orderId: string,
  data?: PayOrderRequest
): Promise<Payment> {
  const res = await apiClient.post<Payment>(
    `/api/v1/orders/${orderId}/pay`,
    data || { method: 'UPI' }
  );
  return res.data;
}
