import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { DarkStore } from '../api/types';
import { listStores } from '../api/catalog';

interface StoreContextType {
  stores: DarkStore[];
  selectedStore: DarkStore | null;
  isLoadingStores: boolean;
  storeError: string | null;
  setSelectedStore: (store: DarkStore) => void;
  refetchStores: () => Promise<void>;
  isDemoMode: boolean;
  setIsDemoMode: (val: boolean) => void;
}

const StoreContext = createContext<StoreContextType | undefined>(undefined);

const SELECTED_STORE_KEY = 'quickcart_selected_store_id';
const DEMO_MODE_KEY = 'quickcart_demo_preview_mode';

// Isolated, client-side demo stores for preview exploration when production database is unseeded
export const DEMO_STORES: DarkStore[] = [
  {
    id: '11111111-1111-1111-1111-111111111111',
    name: 'Indiranagar Hub #04',
    latitude: 12.9716,
    longitude: 77.5946,
    is_active: true,
  },
  {
    id: '22222222-2222-2222-2222-222222222222',
    name: 'Koramangala Express #09',
    latitude: 12.9352,
    longitude: 77.6245,
    is_active: true,
  },
  {
    id: '33333333-3333-3333-3333-333333333333',
    name: 'HSR Layout FastStore #02',
    latitude: 12.9121,
    longitude: 77.6446,
    is_active: true,
  },
];

export const StoreProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [stores, setStores] = useState<DarkStore[]>([]);
  const [selectedStore, setSelectedStoreState] = useState<DarkStore | null>(null);
  const [isLoadingStores, setIsLoadingStores] = useState(true);
  const [storeError, setStoreError] = useState<string | null>(null);
  const [isDemoMode, setIsDemoModeState] = useState<boolean>(() => {
    return localStorage.getItem(DEMO_MODE_KEY) === 'true';
  });

  const setIsDemoMode = (val: boolean) => {
    setIsDemoModeState(val);
    localStorage.setItem(DEMO_MODE_KEY, val ? 'true' : 'false');
  };

  const fetchStores = useCallback(async () => {
    setIsLoadingStores(true);
    setStoreError(null);
    try {
      const data = await listStores();
      setStores(data);

      const savedStoreId = localStorage.getItem(SELECTED_STORE_KEY);
      if (savedStoreId && data.length > 0) {
        const found = data.find((s) => s.id === savedStoreId && s.is_active);
        if (found) {
          setSelectedStoreState(found);
          return;
        }
      }

      if (data.length > 0) {
        const activeStore = data.find((s) => s.is_active) || data[0];
        setSelectedStoreState(activeStore);
        localStorage.setItem(SELECTED_STORE_KEY, activeStore.id);
      } else {
        setSelectedStoreState(null);
      }
    } catch (err: unknown) {
      setStoreError('Unable to connect to stores catalog.');
    } finally {
      setIsLoadingStores(false);
    }
  }, []);

  useEffect(() => {
    fetchStores();
  }, [fetchStores]);

  const setSelectedStore = (store: DarkStore) => {
    setSelectedStoreState(store);
    localStorage.setItem(SELECTED_STORE_KEY, store.id);
  };

  // Determine active stores list depending on real backend catalog vs isolated demo preview
  const effectiveStores = stores.length > 0 ? stores : isDemoMode ? DEMO_STORES : [];
  const effectiveSelectedStore =
    selectedStore || (isDemoMode && effectiveStores.length > 0 ? effectiveStores[0] : null);

  return (
    <StoreContext.Provider
      value={{
        stores: effectiveStores,
        selectedStore: effectiveSelectedStore,
        isLoadingStores,
        storeError,
        setSelectedStore,
        refetchStores: fetchStores,
        isDemoMode,
        setIsDemoMode,
      }}
    >
      {children}
    </StoreContext.Provider>
  );
};

export function useStore(): StoreContextType {
  const context = useContext(StoreContext);
  if (!context) {
    throw new Error('useStore must be used within a StoreProvider');
  }
  return context;
}
