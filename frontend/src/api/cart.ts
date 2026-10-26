import { apiClient } from './client';
import { AddCartItemRequest, CartResponse, UpdateCartItemRequest } from './types';

export async function getCart(): Promise<CartResponse> {
  const res = await apiClient.get<CartResponse>('/api/v1/cart');
  return res.data;
}

export async function addItemToCart(data: AddCartItemRequest): Promise<CartResponse> {
  const res = await apiClient.post<CartResponse>('/api/v1/cart/items', data);
  return res.data;
}

export async function updateCartItemQuantity(
  productId: string,
  quantity: number
): Promise<CartResponse> {
  const payload: UpdateCartItemRequest = { quantity };
  const res = await apiClient.patch<CartResponse>(
    `/api/v1/cart/items/${productId}`,
    payload
  );
  return res.data;
}

export async function removeCartItem(productId: string): Promise<CartResponse> {
  const res = await apiClient.delete<CartResponse>(`/api/v1/cart/items/${productId}`);
  return res.data;
}

export async function clearCart(): Promise<{ message: string }> {
  const res = await apiClient.delete<{ message: string }>('/api/v1/cart');
  return res.data;
}
