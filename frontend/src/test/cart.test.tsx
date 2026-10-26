import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { CartDrawer } from '../components/cart/CartDrawer';
import { CartProvider, useCart } from '../context/CartContext';
import { AuthProvider } from '../context/AuthContext';
import { ToastProvider } from '../context/ToastContext';
import React, { useEffect } from 'react';

// Test harness component to open cart drawer
const OpenCartHarness: React.FC = () => {
  const { openCart } = useCart();
  useEffect(() => {
    openCart();
  }, [openCart]);
  return <CartDrawer />;
};

describe('CartDrawer Component', () => {
  it('renders empty cart state with message when cart has no items', () => {
    render(
      <ToastProvider>
        <AuthProvider>
          <CartProvider>
            <MemoryRouter>
              <OpenCartHarness />
            </MemoryRouter>
          </CartProvider>
        </AuthProvider>
      </ToastProvider>
    );

    expect(screen.getByText('Your cart is empty')).toBeInTheDocument();
    expect(
      screen.getByText(/Explore our catalog to add fresh groceries/i)
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /browse catalog/i })).toBeInTheDocument();
  });
});
