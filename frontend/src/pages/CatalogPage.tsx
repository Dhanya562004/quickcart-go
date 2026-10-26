import React, { useState, useEffect, useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import {
  Search,
  SlidersHorizontal,
  PackageOpen,
} from 'lucide-react';
import { useStore } from '../context/StoreContext';
import { Category, StoreProduct } from '../api/types';
import { listCategories, listStoreProducts } from '../api/catalog';
import { DEMO_CATEGORIES, DEMO_PRODUCTS } from '../data/demoCatalog';
import { ProductCard } from '../components/product/ProductCard';
import { ProductDetailModal } from '../components/product/ProductDetailModal';
import { CategoryPill } from '../components/product/CategoryPill';
import { ProductSkeleton } from '../components/ui/Skeleton';
import { EmptyState } from '../components/ui/EmptyState';
import { DemoSetupBanner } from '../components/demo/DemoSetupBanner';

export const CatalogPage: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const { stores, selectedStore, setSelectedStore, isDemoMode } = useStore();

  const [categories, setCategories] = useState<Category[]>([]);
  const [products, setProducts] = useState<StoreProduct[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [selectedProduct, setSelectedProduct] = useState<StoreProduct | null>(null);

  // Filter & Search states
  const categoryParam = searchParams.get('category') || 'all';
  const queryParam = searchParams.get('q') || '';
  const [searchQuery, setSearchQuery] = useState(queryParam);
  const [onlyInStock, setOnlyInStock] = useState(false);
  const [sortBy, setSortBy] = useState<'featured' | 'price-asc' | 'price-desc' | 'name'>('featured');

  // Sync search param changes
  useEffect(() => {
    setSearchQuery(queryParam);
  }, [queryParam]);

  // Load Categories
  useEffect(() => {
    async function fetchCategories() {
      try {
        const data = await listCategories();
        if (data && data.length > 0) {
          setCategories(data);
        } else if (isDemoMode) {
          setCategories(DEMO_CATEGORIES);
        } else {
          setCategories([]);
        }
      } catch {
        if (isDemoMode) setCategories(DEMO_CATEGORIES);
      }
    }
    fetchCategories();
  }, [isDemoMode]);

  // Load Products for current store
  useEffect(() => {
    async function fetchProducts() {
      setIsLoading(true);
      try {
        if (selectedStore?.id) {
          const data = await listStoreProducts(selectedStore.id, 100, 0);
          if (data && data.length > 0) {
            setProducts(data);
          } else if (isDemoMode) {
            setProducts(DEMO_PRODUCTS);
          } else {
            setProducts([]);
          }
        } else if (isDemoMode) {
          setProducts(DEMO_PRODUCTS);
        } else {
          setProducts([]);
        }
      } catch {
        if (isDemoMode) setProducts(DEMO_PRODUCTS);
      } finally {
        setIsLoading(false);
      }
    }
    fetchProducts();
  }, [selectedStore, isDemoMode]);

  // Client-side filtering & sorting
  const filteredProducts = useMemo(() => {
    return products
      .filter((p) => {
        // Category filter
        if (categoryParam !== 'all') {
          // If product has category_id
          if (p.category_id && p.category_id !== categoryParam) {
            return false;
          }
        }
        // In-stock filter
        if (onlyInStock && p.available_quantity <= 0) {
          return false;
        }
        // Search query
        if (searchQuery.trim()) {
          const q = searchQuery.toLowerCase();
          const matchName = p.name.toLowerCase().includes(q);
          const matchSku = p.sku.toLowerCase().includes(q);
          return matchName || matchSku;
        }
        return true;
      })
      .sort((a, b) => {
        if (sortBy === 'price-asc') return a.price_paise - b.price_paise;
        if (sortBy === 'price-desc') return b.price_paise - a.price_paise;
        if (sortBy === 'name') return a.name.localeCompare(b.name);
        return 0; // featured
      });
  }, [products, categoryParam, onlyInStock, searchQuery, sortBy]);

  const handleCategorySelect = (id: string) => {
    const nextParams = new URLSearchParams(searchParams);
    if (id === 'all') {
      nextParams.delete('category');
    } else {
      nextParams.set('category', id);
    }
    setSearchParams(nextParams);
  };

  const handleSearchChange = (val: string) => {
    setSearchQuery(val);
    const nextParams = new URLSearchParams(searchParams);
    if (val.trim()) {
      nextParams.set('q', val.trim());
    } else {
      nextParams.delete('q');
    }
    setSearchParams(nextParams);
  };

  return (
    <div className="space-y-6 sm:space-y-8">
      <DemoSetupBanner />

      {/* Header & Store Selector Bar */}
      <div className="bg-white rounded-3xl p-5 sm:p-7 border border-slate-100 shadow-card flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <span className="text-xs font-bold text-brand-600 uppercase tracking-widest block mb-1">
            Product Catalog
          </span>
          <h1 className="text-2xl sm:text-3xl font-black text-slate-900 tracking-tight">
            Browse Groceries & Essentials
          </h1>
          <p className="text-xs sm:text-sm text-slate-500 mt-1">
            Real-time dark store inventory with atomic checkout locking
          </p>
        </div>

        {/* Store Switcher */}
        {stores.length > 0 && (
          <div className="flex items-center gap-2 self-start md:self-center">
            <span className="text-xs font-semibold text-slate-400">Dark Store:</span>
            <select
              value={selectedStore?.id || ''}
              onChange={(e) => {
                const found = stores.find((s) => s.id === e.target.value);
                if (found) setSelectedStore(found);
              }}
              className="text-xs font-bold bg-slate-50 border border-slate-200 rounded-xl px-3 py-2 text-slate-800 focus:outline-none focus:ring-2 focus:ring-brand-500/20"
            >
              {stores.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          </div>
        )}
      </div>

      {/* Category Pills Slider */}
      <div className="flex items-center gap-2 overflow-x-auto pb-2 scrollbar-none">
        <button
          onClick={() => handleCategorySelect('all')}
          className={`px-4 py-2 rounded-xl text-xs sm:text-sm font-semibold whitespace-nowrap transition-all duration-200 shadow-2xs ${
            categoryParam === 'all'
              ? 'bg-brand-600 text-white scale-[1.02]'
              : 'bg-white text-slate-700 hover:bg-slate-50 border border-slate-200/80'
          }`}
        >
          All Items ({products.length})
        </button>
        {categories.map((c) => (
          <CategoryPill
            key={c.id}
            category={c}
            isActive={categoryParam === c.id}
            onClick={() => handleCategorySelect(c.id)}
          />
        ))}
      </div>

      {/* Filter and Search Bar */}
      <div className="bg-white rounded-2xl p-4 border border-slate-100 shadow-card flex flex-col md:flex-row items-stretch md:items-center justify-between gap-3">
        {/* Search Input */}
        <div className="relative flex-1">
          <Search className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => handleSearchChange(e.target.value)}
            placeholder="Search by product name, brand, or SKU..."
            className="w-full pl-10 pr-4 py-2 bg-slate-50 text-sm text-slate-900 rounded-xl border border-slate-200 focus:bg-white focus:outline-none focus:ring-2 focus:ring-brand-500/20 transition placeholder:text-slate-400"
          />
        </div>

        {/* Filters Controls */}
        <div className="flex items-center justify-between md:justify-end gap-3 flex-wrap">
          <label className="flex items-center gap-2 text-xs font-semibold text-slate-700 cursor-pointer select-none">
            <input
              type="checkbox"
              checked={onlyInStock}
              onChange={(e) => setOnlyInStock(e.target.checked)}
              className="rounded text-brand-600 focus:ring-brand-500 w-4 h-4"
            />
            <span>In Stock Only</span>
          </label>

          <div className="flex items-center gap-1.5">
            <SlidersHorizontal className="w-4 h-4 text-slate-400" />
            <select
              value={sortBy}
              onChange={(e) => setSortBy(e.target.value as any)}
              className="text-xs font-semibold bg-slate-50 border border-slate-200 rounded-xl px-2.5 py-1.5 text-slate-700 focus:outline-none"
            >
              <option value="featured">Featured</option>
              <option value="price-asc">Price: Low to High</option>
              <option value="price-desc">Price: High to Low</option>
              <option value="name">Name: A to Z</option>
            </select>
          </div>
        </div>
      </div>

      {/* Products Grid */}
      {isLoading ? (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 sm:gap-5">
          {Array.from({ length: 8 }).map((_, i) => (
            <ProductSkeleton key={i} />
          ))}
        </div>
      ) : filteredProducts.length === 0 ? (
        <EmptyState
          icon={<PackageOpen className="w-8 h-8 text-slate-400" />}
          title="No products match your criteria"
          description={
            searchQuery
              ? `No catalog items matched "${searchQuery}". Try a different search term or clear the filter.`
              : 'This store has no products available in the selected category.'
          }
          actionLabel="Clear Filters"
          onAction={() => {
            setSearchQuery('');
            setOnlyInStock(false);
            const nextParams = new URLSearchParams();
            setSearchParams(nextParams);
          }}
        />
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4 sm:gap-5">
          {filteredProducts.map((p) => (
            <ProductCard
              key={p.product_id}
              product={p}
              storeId={selectedStore?.id || 'demo-store'}
              onViewDetails={(prod) => setSelectedProduct(prod)}
            />
          ))}
        </div>
      )}

      {/* Product Detail Modal */}
      <ProductDetailModal
        product={selectedProduct}
        storeId={selectedStore?.id || 'demo-store'}
        isOpen={!!selectedProduct}
        onClose={() => setSelectedProduct(null)}
      />
    </div>
  );
};
