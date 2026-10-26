import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { LoginRequest, RegisterRequest, UserResponse } from '../api/types';
import * as authApi from '../api/auth';
import { clearTokens, getAccessToken, setTokens } from '../api/client';
import { useToast } from './ToastContext';

interface AuthContextType {
  user: UserResponse | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (credentials: LoginRequest) => Promise<UserResponse>;
  register: (data: RegisterRequest) => Promise<UserResponse>;
  logout: () => Promise<void>;
  refreshProfile: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<UserResponse | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const { success } = useToast();

  const loadProfile = useCallback(async () => {
    const token = getAccessToken();
    if (!token) {
      setUser(null);
      setIsLoading(false);
      return;
    }

    try {
      const profile = await authApi.getMe();
      setUser(profile);
    } catch (err) {
      // If profile fetch fails after interceptor attempts refresh, clear tokens
      clearTokens();
      setUser(null);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadProfile();
  }, [loadProfile]);

  const login = async (credentials: LoginRequest): Promise<UserResponse> => {
    try {
      const res = await authApi.login(credentials);
      setTokens(res.tokens);
      setUser(res.user);
      success(`Welcome back, ${res.user.full_name}!`, 'Signed In');
      return res.user;
    } catch (err: unknown) {
      throw err;
    }
  };

  const register = async (data: RegisterRequest): Promise<UserResponse> => {
    try {
      const res = await authApi.register(data);
      success('Account created successfully! Please sign in with your credentials.', 'Registered');
      return res;
    } catch (err: unknown) {
      throw err;
    }
  };

  const logout = async () => {
    try {
      await authApi.logout();
    } catch (err) {
      // Clean up locally regardless of network outcome
    } finally {
      clearTokens();
      setUser(null);
      success('You have been signed out safely.', 'Signed Out');
    }
  };

  const refreshProfile = async () => {
    await loadProfile();
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isLoading,
        login,
        register,
        logout,
        refreshProfile,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

export function useAuth(): AuthContextType {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
