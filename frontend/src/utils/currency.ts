/**
 * Formats a value in paise (1 INR = 100 paise) to a formatted Indian Rupee string.
 * e.g., 7400 -> "₹74.00", 7450 -> "₹74.50"
 */
export function formatPaise(paise: number | undefined | null): string {
  if (paise == null || isNaN(paise)) {
    return '₹0.00';
  }
  const rupees = paise / 100;
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(rupees);
}

/**
 * Converts a rupee float value to paise integer.
 */
export function rupeesToPaise(rupees: number): number {
  return Math.round(rupees * 100);
}
