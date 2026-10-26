import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ToastProvider } from './context/ToastContext';
import { AuthProvider } from './context/AuthContext';
import { StoreProvider } from './context/StoreContext';
import { CartProvider } from './context/CartContext';
import { Layout } from './components/layout/Layout';
import { ProtectedRoute } from './components/auth/ProtectedRoute';

import { HomePage } from './pages/HomePage';
import { CatalogPage } from './pages/CatalogPage';
import { LoginPage } from './pages/LoginPage';
import { RegisterPage } from './pages/RegisterPage';
import { AccountPage } from './pages/AccountPage';
import { CheckoutPage } from './pages/CheckoutPage';
import { OrdersPage } from './pages/OrdersPage';
import { PaymentPage } from './pages/PaymentPage';
import { OrderTrackingPage } from './pages/OrderTrackingPage';
import { NotFoundPage } from './pages/NotFoundPage';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 60 * 1000,
    },
  },
});

export const App: React.FC = () => {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <AuthProvider>
          <StoreProvider>
            <CartProvider>
              <BrowserRouter>
                <Layout>
                  <Routes>
                    <Route path="/" element={<HomePage />} />
                    <Route path="/catalog" element={<CatalogPage />} />
                    <Route path="/login" element={<LoginPage />} />
                    <Route path="/register" element={<RegisterPage />} />

                    {/* Authenticated Protected Routes */}
                    <Route
                      path="/account"
                      element={
                        <ProtectedRoute>
                          <AccountPage />
                        </ProtectedRoute>
                      }
                    />
                    <Route
                      path="/checkout"
                      element={
                        <ProtectedRoute>
                          <CheckoutPage />
                        </ProtectedRoute>
                      }
                    />
                    <Route
                      path="/orders"
                      element={
                        <ProtectedRoute>
                          <OrdersPage />
                        </ProtectedRoute>
                      }
                    />
                    <Route
                      path="/orders/:id"
                      element={
                        <ProtectedRoute>
                          <OrderTrackingPage />
                        </ProtectedRoute>
                      }
                    />
                    <Route
                      path="/orders/:id/pay"
                      element={
                        <ProtectedRoute>
                          <PaymentPage />
                        </ProtectedRoute>
                      }
                    />
                    <Route
                      path="/orders/:id/track"
                      element={
                        <ProtectedRoute>
                          <OrderTrackingPage />
                        </ProtectedRoute>
                      }
                    />

                    {/* Fallback route */}
                    <Route path="/404" element={<NotFoundPage />} />
                    <Route path="*" element={<Navigate to="/404" replace />} />
                  </Routes>
                </Layout>
              </BrowserRouter>
            </CartProvider>
          </StoreProvider>
        </AuthProvider>
      </ToastProvider>
    </QueryClientProvider>
  );
};

export default App;
