/**
 * QuickCart API Type Definitions
 * Directly reflects OpenAPI 3.0 specification in api/openapi.yaml
 */

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  access_expires_in: number;
  refresh_expires_in: number;
}

export interface UserResponse {
  id: string;
  email: string;
  full_name: string;
  phone: string;
  role: 'customer' | 'admin';
  created_at: string;
}

export interface LoginResponse {
  tokens: TokenPair;
  user: UserResponse;
}

export interface RegisterRequest {
  email: string;
  password: string;
  full_name: string;
  phone: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

export interface RefreshRequest {
  refresh_token: string;
}

export interface Category {
  id: string;
  name: string;
  slug: string;
  description?: string;
}

export interface Product {
  id: string;
  name: string;
  sku: string;
  price_paise: number;
  unit: string;
  is_active: boolean;
}

export interface DarkStore {
  id: string;
  name: string;
  latitude: number;
  longitude: number;
  is_active: boolean;
}

export interface StoreProduct {
  product_id: string;
  name: string;
  sku: string;
  price_paise: number;
  available_quantity: number;
  unit?: string;
  category_id?: string;
}

export interface CartItem {
  product_id: string;
  product_name: string;
  quantity: number;
  unit_price_paise: number;
  total_price_paise: number;
}

export interface CartResponse {
  id: string;
  user_id: string;
  store_id?: string;
  items: CartItem[];
  total_items: number;
  total_amount_paise: number;
}

export interface AddCartItemRequest {
  store_id: string;
  product_id: string;
  quantity: number;
}

export interface UpdateCartItemRequest {
  quantity: number;
}

export interface CheckoutRequest {
  delivery_address: string;
}

export type OrderStatus =
  | 'PLACED'
  | 'CONFIRMED'
  | 'PACKING'
  | 'OUT_FOR_DELIVERY'
  | 'DELIVERED'
  | 'CANCELLED';

export interface Order {
  id: string;
  order_number: string;
  user_id: string;
  store_id: string;
  status: OrderStatus;
  total_amount_paise: number;
  delivery_address: string;
  idempotency_key: string;
  created_at: string;
}

export type PaymentMethod = 'UPI' | 'CARD' | 'NETBANKING' | 'WALLET';

export type PaymentStatus = 'PENDING' | 'CAPTURED' | 'FAILED' | 'REFUNDED';

export interface Payment {
  id: string;
  order_id: string;
  attempt: number;
  amount_paise: number;
  currency: string;
  status: PaymentStatus;
  payment_method: PaymentMethod;
  gateway_reference: string;
}

export interface PayOrderRequest {
  method?: PaymentMethod;
}

export interface CancelOrderRequest {
  reason: string;
}

export interface SSEOrderEvent {
  order_id: string;
  status: OrderStatus;
  message?: string;
  timestamp: string;
}

export interface ApiError {
  error?: string;
  message?: string;
  detail?: string;
  status?: number;
  product_id?: string;
  product_name?: string;
  requested?: number;
  available?: number;
}
