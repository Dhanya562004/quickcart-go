import { apiClient } from './client';

export interface HealthResponse {
  status: string;
  timestamp: string;
}

export interface ReadinessResponse {
  status: string;
  database: string;
  redis: string;
  timestamp: string;
  error?: string;
}

export interface SystemInfoResponse {
  service: string;
  version: string;
  environment: string;
  timestamp: string;
}

export async function getHealth(): Promise<HealthResponse> {
  const res = await apiClient.get<HealthResponse>('/healthz', { timeout: 8000 });
  return res.data;
}

export async function getReadiness(): Promise<ReadinessResponse> {
  const res = await apiClient.get<ReadinessResponse>('/readyz', { timeout: 8000 });
  return res.data;
}

export async function getSystemInfo(): Promise<SystemInfoResponse> {
  const res = await apiClient.get<SystemInfoResponse>('/api/v1/system/info');
  return res.data;
}
