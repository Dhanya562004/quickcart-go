import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { CartItem, CartResponse } from '../api/types';
import * as cartApi from '../api/cart';
import { useAuth } from './AuthContext';
import { useToast } from './ToastContext';
import { getApiErrorMessage } from '../api/client';
import axios from 'axios';

interface CrossStoreConflictData {
  storeId: string;
  productId: string;
  productName: string;
  quantity: number;
}

interface CartContextType {
  cart: CartResponse | null;
  items: CartItem[];
  itemCount: number;
  totalAmountPaise: number;
  isLoading: boolean;
  isCartOpen: boolean;
  openCart: () => void;
  closeCart: () => void;
  toggleCart: () => void;
  addItem: (storeId: string, productId: string, productName: string, quantity?: number) => Promise<void>;
  updateQuantity: (productId: string, quantity: number) => Promise<void>;
  removeItem: (productId: string) => Promise<void>;
  clearCart: () => Promise<void>;
  refetchCart: () => Promise<void>;
  crossStoreConflict: CrossStoreConflictData | null;
  resolveConflict: (clearAndAdd: boolean) => Promise<void>;
}

const CartContext = createContext<CartContextType | undefined>(undefined);

export const CartProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [cart, setCart] = useState<CartResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isCartOpen, setIsCartOpen] = useState(false);
  const [crossStoreConflict, setCrossStoreConflict] = useState<CrossStoreConflictData | null>(null);

  const { isAuthenticated } = useAuth();
  const { success, error, info } = useToast();

  const fetchCart = useCallback(async () => {
    if (!isAuthenticated) {
      setCart(null);
      return;
    }

    setIsLoading(true);
    try {
      const data = await cartApi.getCart();
      setCart(data);
    } catch (err: unknown) {
      // If empty or initial, cart might return 404/empty or null
      setCart(null);
    } finally {
      setIsLoading(false);
    }
  }, [isAuthenticated]);

  useEffect(() => {
    fetchCart();
  }, [fetchCart]);

  const openCart = () => setIsCartOpen(true);
  const closeCart = () => setIsCartOpen(false);
  const toggleCart = () => setIsCartOpen((prev) => !prev);

  const addItem = async (
    storeId: string,
    productId: string,
    productName: string,
    quantity: number = 1
  ) => {
    if (!isAuthenticated) {
      info('Please sign in to add items to your shopping cart.', 'Authentication Required');
      return;
    }

    try {
      const updated = await cartApi.addItemToCart({
        store_id: storeId,
        product_id: productId,
        quantity,
      });
      setCart(updated);
      success(`${productName} added to cart!`, 'Cart Updated');
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.status === 409) {
        // Cross-store conflict
        setCrossStoreConflict({
          storeId,
          productId,
          productName,
          quantity,
        });
        return;
      }
      error(getApiErrorMessage(err), 'Could Not Add Item');
    }
  };

  const updateQuantity = async (productId: string, quantity: number) => {
    if (!isAuthenticated) return;

    try {
      if (quantity <= 0) {
        await removeItem(productId);
        return;
      }
      const updated = await cartApi.updateCartItemQuantity(productId, quantity);
      setCart(updated);
    } catch (err: unknown) {
      error(getApiErrorMessage(err), 'Cart Update Failed');
    }
  };

  const removeItem = async (productId: string) => {
    if (!isAuthenticated) return;

    try {
      const updated = await cartApi.removeCartItem(productId);
      setCart(updated);
      info('Item removed from cart.');
    } catch (err: unknown) {
      error(getApiErrorMessage(err), 'Remove Failed');
    }
  };

  const clearCart = async () => {
    if (!isAuthenticated) return;

    try {
      await cartApi.clearCart();
      setCart(null);
      info('Your cart has been cleared.');
    } catch (err: unknown) {
      error(getApiErrorMessage(err), 'Clear Cart Failed');
    }
  };

  const resolveConflict = async (clearAndAdd: boolean) => {
    if (!crossStoreConflict) return;
    const pending = { ...crossStoreConflict };
    setCrossStoreConflict(null);

    if (clearAndAdd) {
      try {
        await cartApi.clearCart();
        const updated = await cartApi.addItemToCart({
          store_id: pending.storeId,
          product_id: pending.productId,
          quantity: pending.quantity,
        });
        setCart(updated);
        success(`Cart cleared and ${pending.productName} added!`, 'Store Switched');
      } catch (err: unknown) {
        error(getApiErrorMessage(err), 'Operation Failed');
      }
    }
  };

  const items = cart?.items || [];
  const itemCount = cart?.total_items || items.reduce((sum, item) => sum + item.quantity, 0);
  const totalAmountPaise = cart?.total_amount_paise || 0;

  return (
    <CartContext.Provider
      value={{
        cart,
        items,
        itemCount,
        totalAmountPaise,
        isLoading,
        isCartOpen,
        openCart,
        closeCart,
        toggleCart,
        addItem,
        updateQuantity,
        removeItem,
        clearCart,
        refetchCart: fetchCart,
        crossStoreConflict,
        resolveConflict,
      }}
    >
      {children}
    </CartContext.Provider>
  );
};

export function useCart(): CartContextType {
  const context = useContext(CartContext);
  if (!context) {
    throw new Error('useCart must be used within a CartProvider');
  }
  return context;
}
