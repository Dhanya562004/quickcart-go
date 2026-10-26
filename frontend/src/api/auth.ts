import { apiClient, clearTokens, getRefreshToken } from './client';
import {
  LoginRequest,
  LoginResponse,
  RegisterRequest,
  TokenPair,
  UserResponse,
} from './types';

export async function register(data: RegisterRequest): Promise<UserResponse> {
  const res = await apiClient.post<UserResponse>('/api/v1/auth/register', data);
  return res.data;
}

export async function login(data: LoginRequest): Promise<LoginResponse> {
  const res = await apiClient.post<LoginResponse>('/api/v1/auth/login', data);
  return res.data;
}

export async function refresh(refreshToken: string): Promise<TokenPair> {
  const res = await apiClient.post<TokenPair>('/api/v1/auth/refresh', {
    refresh_token: refreshToken,
  });
  return res.data;
}

export async function logout(): Promise<void> {
  const token = getRefreshToken();
  try {
    if (token) {
      await apiClient.post('/api/v1/auth/logout', { refresh_token: token });
    }
  } catch (err) {
    // Silently proceed with local cleanup on network error
  } finally {
    clearTokens();
  }
}

export async function getMe(): Promise<UserResponse> {
  const res = await apiClient.get<UserResponse>('/api/v1/auth/me');
  return res.data;
}
