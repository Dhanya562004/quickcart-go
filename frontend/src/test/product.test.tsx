import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProductCard } from '../components/product/ProductCard';
import { CartProvider } from '../context/CartContext';
import { AuthProvider } from '../context/AuthContext';
import { ToastProvider } from '../context/ToastContext';
import { StoreProduct } from '../api/types';

const mockProduct: StoreProduct = {
  product_id: 'prod-test-01',
  name: 'Amul Taaza Toned Milk 1L',
  sku: 'MILK-AMUL-1L',
  price_paise: 7400,
  available_quantity: 45,
  unit: '1 L',
};

const mockOutOfStockProduct: StoreProduct = {
  product_id: 'prod-test-02',
  name: 'Organic Strawberries',
  sku: 'FRUIT-STRAW-200G',
  price_paise: 12000,
  available_quantity: 0,
  unit: '200 g',
};

function renderWithProviders(ui: React.ReactElement) {
  return render(
    <ToastProvider>
      <AuthProvider>
        <CartProvider>{ui}</CartProvider>
      </AuthProvider>
    </ToastProvider>
  );
}

describe('ProductCard Component', () => {
  it('renders product name, unit, SKU, formatted price, and stock badge', () => {
    renderWithProviders(
      <ProductCard product={mockProduct} storeId="store-1" />
    );

    expect(screen.getByText('Amul Taaza Toned Milk 1L')).toBeInTheDocument();
    expect(screen.getByText('1 L')).toBeInTheDocument();
    expect(screen.getByText('MILK-AMUL-1L')).toBeInTheDocument();
    expect(screen.getByText(/74\.00/)).toBeInTheDocument();
    expect(screen.getByText('In Stock: 45')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add/i })).toBeEnabled();
  });

  it('renders Out of Stock badge and disables ADD button when available_quantity is 0', () => {
    renderWithProviders(
      <ProductCard product={mockOutOfStockProduct} storeId="store-1" />
    );

    expect(screen.getByText('Organic Strawberries')).toBeInTheDocument();
    expect(screen.getByText('Out of Stock')).toBeInTheDocument();
    const addButton = screen.getByRole('button', { name: /add/i });
    expect(addButton).toBeDisabled();
  });

  it('calls onViewDetails callback when quick view button is clicked', async () => {
    const onViewDetails = vi.fn();
    renderWithProviders(
      <ProductCard
        product={mockProduct}
        storeId="store-1"
        onViewDetails={onViewDetails}
      />
    );

    const viewButton = screen.getByLabelText('View product details');
    await userEvent.click(viewButton);
    expect(onViewDetails).toHaveBeenCalledWith(mockProduct);
  });
});
