import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios';
import { ApiError, TokenPair } from './types';

// API Base URL from environment variables or default to production deployment
export const API_BASE_URL =
  import.meta.env.VITE_API_BASE_URL?.replace(/\/$/, '') ||
  'https://quickcart-go.onrender.com';

const ACCESS_TOKEN_KEY = 'quickcart_access_token';
const REFRESH_TOKEN_KEY = 'quickcart_refresh_token';

// In-memory token cache to minimize storage lookups
let inMemoryAccessToken: string | null = null;
let inMemoryRefreshToken: string | null = null;

export function getAccessToken(): string | null {
  if (inMemoryAccessToken) return inMemoryAccessToken;
  return localStorage.getItem(ACCESS_TOKEN_KEY);
}

export function getRefreshToken(): string | null {
  if (inMemoryRefreshToken) return inMemoryRefreshToken;
  return localStorage.getItem(REFRESH_TOKEN_KEY);
}

export function setTokens(tokens: TokenPair | { access_token: string; refresh_token: string }) {
  inMemoryAccessToken = tokens.access_token;
  inMemoryRefreshToken = tokens.refresh_token;
  try {
    localStorage.setItem(ACCESS_TOKEN_KEY, tokens.access_token);
    localStorage.setItem(REFRESH_TOKEN_KEY, tokens.refresh_token);
  } catch (e) {
    // Gracefully handle storage quota or private browsing exceptions
  }
}

export function clearTokens() {
  inMemoryAccessToken = null;
  inMemoryRefreshToken = null;
  try {
    localStorage.removeItem(ACCESS_TOKEN_KEY);
    localStorage.removeItem(REFRESH_TOKEN_KEY);
  } catch (e) {
    // Ignore storage clear error
  }
}

// Axios instance configured for QuickCart
export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Mutex & queue for concurrency-safe refresh token rotation
let isRefreshing = false;
let failedQueue: Array<{
  resolve: (value?: unknown) => void;
  reject: (reason?: unknown) => void;
}> = [];

const processQueue = (error: Error | null, token: string | null = null) => {
  failedQueue.forEach((prom) => {
    if (error) {
      prom.reject(error);
    } else {
      prom.resolve(token);
    }
  });
  failedQueue = [];
};

// Request interceptor: injects Authorization Bearer token
apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = getAccessToken();
    if (token && !config.headers.Authorization) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor: handles 401 Unauthorized via atomic refresh rotation
apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError<ApiError>) => {
    const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean };

    if (!error.response || !originalRequest) {
      return Promise.reject(error);
    }

    const is401 = error.response.status === 401;
    const isAuthRoute =
      originalRequest.url?.includes('/auth/login') ||
      originalRequest.url?.includes('/auth/register') ||
      originalRequest.url?.includes('/auth/refresh');

    if (is401 && !originalRequest._retry && !isAuthRoute) {
      if (isRefreshing) {
        // Enqueue concurrent requests while single refresh is in flight
        return new Promise((resolve, reject) => {
          failedQueue.push({ resolve, reject });
        })
          .then((token) => {
            originalRequest.headers.Authorization = `Bearer ${token}`;
            return apiClient(originalRequest);
          })
          .catch((err) => Promise.reject(err));
      }

      originalRequest._retry = true;
      isRefreshing = true;

      const refreshToken = getRefreshToken();
      if (!refreshToken) {
        isRefreshing = false;
        clearTokens();
        return Promise.reject(error);
      }

      try {
        // Atomic refresh rotation request
        const { data } = await axios.post<TokenPair>(
          `${API_BASE_URL}/api/v1/auth/refresh`,
          { refresh_token: refreshToken },
          { headers: { 'Content-Type': 'application/json' } }
        );

        setTokens(data);
        processQueue(null, data.access_token);

        originalRequest.headers.Authorization = `Bearer ${data.access_token}`;
        return apiClient(originalRequest);
      } catch (refreshErr) {
        processQueue(refreshErr as Error, null);
        clearTokens();
        return Promise.reject(refreshErr);
      } finally {
        isRefreshing = false;
      }
    }

    return Promise.reject(error);
  }
);

/**
 * Extracts a human-friendly error string from an Axios error or standard error.
 */
export function getApiErrorMessage(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const data = error.response?.data as ApiError | undefined;
    if (data?.error) return data.error;
    if (data?.message) return data.message;
    if (data?.detail) return data.detail;
    if (error.response?.status === 409) return 'Inventory conflict or stock unavailable.';
    if (error.response?.status === 429) return 'Rate limit exceeded. Please wait a moment.';
    if (error.code === 'ECONNABORTED' || error.message.includes('timeout')) {
      return 'Request timed out. The backend container may be cold-starting; please retry.';
    }
    return error.message || 'Network request failed.';
  }
  if (error instanceof Error) {
    return error.message;
  }
  return 'An unexpected error occurred.';
}
