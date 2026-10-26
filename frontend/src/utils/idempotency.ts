/**
 * Generates a stable and collision-resistant idempotency key for checkout and sensitive mutations.
 */
export function generateIdempotencyKey(prefix: string = 'idemp_order'): string {
  const timestamp = Date.now();
  const randomPart = typeof crypto !== 'undefined' && crypto.randomUUID
    ? crypto.randomUUID().replace(/-/g, '').slice(0, 16)
    : Math.random().toString(36).substring(2, 18);
  return `${prefix}_${timestamp}_${randomPart}`;
}
