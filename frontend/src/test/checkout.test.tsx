import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { CheckoutPage } from '../pages/CheckoutPage';
import { CartProvider } from '../context/CartContext';
import { AuthProvider } from '../context/AuthContext';
import { ToastProvider } from '../context/ToastContext';

describe('CheckoutPage Component', () => {
  it('renders empty cart state when no items exist to checkout', () => {
    render(
      <ToastProvider>
        <AuthProvider>
          <CartProvider>
            <MemoryRouter>
              <CheckoutPage />
            </MemoryRouter>
          </CartProvider>
        </AuthProvider>
      </ToastProvider>
    );

    expect(screen.getByText('Your cart is empty')).toBeInTheDocument();
    expect(
      screen.getByText(/Add items to your cart before proceeding to atomic checkout/i)
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: /browse product catalog/i })
    ).toBeInTheDocument();
  });
});
