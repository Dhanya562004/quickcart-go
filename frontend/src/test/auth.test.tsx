import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import { LoginPage } from '../pages/LoginPage';
import { ProtectedRoute } from '../components/auth/ProtectedRoute';
import { AuthProvider } from '../context/AuthContext';
import { ToastProvider } from '../context/ToastContext';

describe('Authentication & Protected Routes', () => {
  it('renders login form elements and validation errors on empty submission', async () => {
    render(
      <ToastProvider>
        <AuthProvider>
          <MemoryRouter>
            <LoginPage />
          </MemoryRouter>
        </AuthProvider>
      </ToastProvider>
    );

    expect(screen.getByPlaceholderText('name@example.com')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('••••••••')).toBeInTheDocument();
    const submitButton = screen.getByRole('button', { name: /sign in/i });

    await userEvent.click(submitButton);

    expect(await screen.findByText(/please enter a valid email address/i)).toBeInTheDocument();
  });

  it('redirects unauthenticated user from protected route to login', () => {
    render(
      <ToastProvider>
        <AuthProvider>
          <MemoryRouter initialEntries={['/account']}>
            <Routes>
              <Route
                path="/account"
                element={
                  <ProtectedRoute>
                    <div>Secret Account Content</div>
                  </ProtectedRoute>
                }
              />
              <Route path="/login" element={<div>Login Page Redirected</div>} />
            </Routes>
          </MemoryRouter>
        </AuthProvider>
      </ToastProvider>
    );

    // Because user is not authenticated, should not show secret content
    expect(screen.queryByText('Secret Account Content')).not.toBeInTheDocument();
    expect(screen.getByText('Login Page Redirected')).toBeInTheDocument();
  });
});
