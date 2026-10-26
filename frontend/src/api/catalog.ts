import { apiClient } from './client';
import { Category, DarkStore, Product, StoreProduct } from './types';

export async function listCategories(): Promise<Category[]> {
  const res = await apiClient.get<Category[]>('/api/v1/categories');
  return res.data || [];
}

export async function getCategory(id: string): Promise<Category> {
  const res = await apiClient.get<Category>(`/api/v1/categories/${id}`);
  return res.data;
}

export async function listStores(): Promise<DarkStore[]> {
  const res = await apiClient.get<DarkStore[]>('/api/v1/stores');
  return res.data || [];
}

export async function listStoreProducts(
  storeId: string,
  limit: number = 50,
  offset: number = 0
): Promise<StoreProduct[]> {
  const res = await apiClient.get<StoreProduct[]>(
    `/api/v1/stores/${storeId}/products`,
    {
      params: { limit, offset },
    }
  );
  return res.data || [];
}

export async function getProduct(id: string): Promise<Product> {
  const res = await apiClient.get<Product>(`/api/v1/products/${id}`);
  return res.data;
}
