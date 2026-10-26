import { describe, it, expect, beforeEach } from 'vitest';
import { formatPaise, rupeesToPaise } from '../utils/currency';
import { generateIdempotencyKey } from '../utils/idempotency';
import {
  getAccessToken,
  getRefreshToken,
  setTokens,
  clearTokens,
  getApiErrorMessage,
  apiClient,
} from '../api/client';
import axios from 'axios';

describe('Currency and Idempotency Utilities', () => {
  it('formats paise to Indian Rupee strings correctly', () => {
    expect(formatPaise(7400)).toMatch(/₹\s*74\.00/);
    expect(formatPaise(3550)).toMatch(/₹\s*35\.50/);
    expect(formatPaise(0)).toMatch(/₹\s*0\.00/);
    expect(formatPaise(null)).toMatch(/₹\s*0\.00/);
    expect(formatPaise(undefined)).toMatch(/₹\s*0\.00/);
  });

  it('converts rupees float to paise integer accurately', () => {
    expect(rupeesToPaise(74)).toBe(7400);
    expect(rupeesToPaise(35.5)).toBe(3550);
    expect(rupeesToPaise(199.99)).toBe(19999);
  });

  it('generates unique, collision-resistant idempotency keys with valid prefixes', () => {
    const key1 = generateIdempotencyKey('order');
    const key2 = generateIdempotencyKey('order');
    expect(key1).toMatch(/^order_\d+_[a-z0-9]+$/);
    expect(key2).toMatch(/^order_\d+_[a-z0-9]+$/);
    expect(key1).not.toBe(key2);
  });
});

describe('API Client & Token Management', () => {
  beforeEach(() => {
    clearTokens();
  });

  it('stores and retrieves access and refresh tokens safely', () => {
    expect(getAccessToken()).toBeNull();
    expect(getRefreshToken()).toBeNull();

    setTokens({
      access_token: 'test_access_jwt_123',
      refresh_token: 'test_refresh_random_456',
    });

    expect(getAccessToken()).toBe('test_access_jwt_123');
    expect(getRefreshToken()).toBe('test_refresh_random_456');

    clearTokens();
    expect(getAccessToken()).toBeNull();
    expect(getRefreshToken()).toBeNull();
  });

  it('extracts human-friendly error messages from various response objects', () => {
    // Normal Error
    expect(getApiErrorMessage(new Error('Network disconnected'))).toBe('Network disconnected');

    // Non-error fallback
    expect(getApiErrorMessage('unknown')).toBe('An unexpected error occurred.');

    // Axios error with data.error
    const customAxiosErr = new axios.AxiosError('Request failed with 400');
    customAxiosErr.response = {
      status: 400,
      statusText: 'Bad Request',
      headers: {},
      config: {} as any,
      data: { error: 'invalid email or password' },
    };
    expect(getApiErrorMessage(customAxiosErr)).toBe('invalid email or password');
  });

  it('verifies API client default configuration', () => {
    expect(apiClient.defaults.baseURL).toContain('http');
    expect(apiClient.defaults.timeout).toBe(30000);
  });
});
