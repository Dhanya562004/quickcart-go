import { API_BASE_URL, getAccessToken } from './client';
import { SSEOrderEvent } from './types';

export type SSEConnectionState = 'connecting' | 'connected' | 'reconnecting' | 'disconnected' | 'error';

export interface SSETrackerOptions {
  orderId: string;
  onEvent?: (event: SSEOrderEvent) => void;
  onHeartbeat?: (data: { timestamp: string }) => void;
  onError?: (error: Error) => void;
  onStateChange?: (state: SSEConnectionState) => void;
  maxReconnectAttempts?: number;
}

/**
 * Creates a secure Server-Sent Events streaming connection using the Fetch API.
 * Keeps JWT access tokens strictly in Authorization headers rather than query strings.
 */
export function createOrderEventStream(options: SSETrackerOptions): { close: () => void } {
  const {
    orderId,
    onEvent,
    onHeartbeat,
    onError,
    onStateChange,
    maxReconnectAttempts = 5,
  } = options;

  let isClosed = false;
  let abortController = new AbortController();
  let reconnectAttempts = 0;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  const url = `${API_BASE_URL}/api/v1/orders/${orderId}/events`;

  async function connect() {
    if (isClosed) return;

    abortController = new AbortController();
    onStateChange?.(reconnectAttempts > 0 ? 'reconnecting' : 'connecting');

    const token = getAccessToken();
    if (!token) {
      onStateChange?.('error');
      onError?.(new Error('Authentication token required for order event stream'));
      return;
    }

    try {
      const response = await fetch(url, {
        method: 'GET',
        headers: {
          Authorization: `Bearer ${token}`,
          Accept: 'text/event-stream',
          'Cache-Control': 'no-cache',
        },
        signal: abortController.signal,
      });

      if (!response.ok) {
        throw new Error(`SSE connection failed with HTTP status ${response.status}`);
      }

      if (!response.body) {
        throw new Error('ReadableStream not supported by response');
      }

      reconnectAttempts = 0;
      onStateChange?.('connected');

      const reader = response.body.getReader();
      const decoder = new TextDecoder('utf-8');
      let buffer = '';

      while (!isClosed) {
        const { value, done } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const parts = buffer.split('\n\n');
        buffer = parts.pop() || '';

        for (const message of parts) {
          if (!message.trim()) continue;

          let eventType = 'message';
          let eventData = '';

          const lines = message.split('\n');
          for (const line of lines) {
            if (line.startsWith('event:')) {
              eventType = line.slice(6).trim();
            } else if (line.startsWith('data:')) {
              eventData += line.slice(5).trim();
            }
          }

          if (eventType === 'order_status') {
            try {
              const parsed: SSEOrderEvent = JSON.parse(eventData);
              onEvent?.(parsed);
            } catch (err) {
              console.warn('Failed to parse SSE order_status payload:', eventData);
            }
          } else if (eventType === 'heartbeat') {
            try {
              const parsed = JSON.parse(eventData);
              onHeartbeat?.(parsed);
            } catch {
              // Heartbeat ping
            }
          }
        }
      }

      if (!isClosed) {
        scheduleReconnect();
      }
    } catch (err: unknown) {
      if (isClosed || (err instanceof DOMException && err.name === 'AbortError')) {
        return;
      }
      onError?.(err instanceof Error ? err : new Error('SSE stream connection error'));
      scheduleReconnect();
    }
  }

  function scheduleReconnect() {
    if (isClosed) return;

    if (reconnectAttempts >= maxReconnectAttempts) {
      onStateChange?.('error');
      onError?.(new Error('Maximum order tracking reconnection attempts exceeded'));
      return;
    }

    reconnectAttempts++;
    onStateChange?.('reconnecting');

    const delay = Math.min(1000 * Math.pow(2, reconnectAttempts - 1), 15000);
    reconnectTimer = setTimeout(() => {
      connect();
    }, delay);
  }

  // Initial connection
  connect();

  return {
    close: () => {
      isClosed = true;
      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      abortController.abort();
      onStateChange?.('disconnected');
    },
  };
}
