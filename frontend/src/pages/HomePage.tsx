import React, { useState, useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  Zap,
  Clock,
  ShieldCheck,
  TrendingUp,
  ArrowRight,
  ShoppingBag,
  Sparkles,
  MapPin,
} from 'lucide-react';
import { useStore } from '../context/StoreContext';
import { Category, StoreProduct } from '../api/types';
import { listCategories, listStoreProducts } from '../api/catalog';
import { DEMO_CATEGORIES, DEMO_PRODUCTS } from '../data/demoCatalog';
import { ProductCard } from '../components/product/ProductCard';
import { ProductDetailModal } from '../components/product/ProductDetailModal';
import { ProductSkeleton } from '../components/ui/Skeleton';
import { DemoSetupBanner } from '../components/demo/DemoSetupBanner';
import { Button } from '../components/ui/Button';

export const HomePage: React.FC = () => {
  const { selectedStore, stores, isDemoMode } = useStore();
  const [categories, setCategories] = useState<Category[]>([]);
  const [products, setProducts] = useState<StoreProduct[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [selectedProduct, setSelectedProduct] = useState<StoreProduct | null>(null);

  const navigate = useNavigate();

  useEffect(() => {
    async function loadData() {
      setIsLoading(true);
      try {
        const catData = await listCategories();
        if (catData && catData.length > 0) {
          setCategories(catData);
        } else if (isDemoMode) {
          setCategories(DEMO_CATEGORIES);
        } else {
          setCategories([]);
        }

        if (selectedStore?.id) {
          const prodData = await listStoreProducts(selectedStore.id, 8, 0);
          if (prodData && prodData.length > 0) {
            setProducts(prodData);
          } else if (isDemoMode) {
            setProducts(DEMO_PRODUCTS.slice(0, 8));
          } else {
            setProducts([]);
          }
        } else if (isDemoMode) {
          setProducts(DEMO_PRODUCTS.slice(0, 8));
        }
      } catch (err) {
        if (isDemoMode) {
          setCategories(DEMO_CATEGORIES);
          setProducts(DEMO_PRODUCTS.slice(0, 8));
        }
      } finally {
        setIsLoading(false);
      }
    }
    loadData();
  }, [selectedStore, isDemoMode]);

  return (
    <div className="space-y-10 sm:space-y-14">
      {/* Production Demo Notice / Banner */}
      <DemoSetupBanner />

      {/* Hero Section */}
      <section className="relative rounded-3xl overflow-hidden bg-gradient-to-br from-emerald-600 via-brand-600 to-teal-800 text-white p-6 sm:p-10 lg:p-14 shadow-xl">
        <div className="absolute top-0 right-0 -mr-16 -mt-16 w-80 h-80 rounded-full bg-white/10 blur-2xl pointer-events-none" />
        <div className="absolute bottom-0 left-1/3 -mb-20 w-96 h-96 rounded-full bg-emerald-400/20 blur-3xl pointer-events-none" />

        <div className="relative z-10 max-w-2xl">
          <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full bg-white/15 backdrop-blur-md border border-white/20 text-xs font-bold uppercase tracking-wider text-emerald-100 mb-4 animate-bounce-subtle">
            <Zap className="w-3.5 h-3.5 fill-emerald-300 text-emerald-300" />
            <span>Guaranteed 10-Minute Micro-Fulfillment</span>
          </div>

          <h1 className="text-3xl sm:text-5xl lg:text-6xl font-black tracking-tight leading-[1.1] mb-4">
            Instant groceries, <br />
            <span className="text-emerald-200">right when you need them.</span>
          </h1>

          <p className="text-sm sm:text-base text-emerald-50 mb-8 leading-relaxed max-w-xl">
            Order fresh produce, breakfast essentials, dairy, and cold snacks directly from climate-controlled dark stores with atomic zero-overselling inventory.
          </p>

          <div className="flex flex-wrap items-center gap-3">
            <Button
              size="lg"
              onClick={() => navigate('/catalog')}
              className="bg-white text-emerald-900 hover:bg-emerald-50 shadow-lg hover:shadow-xl font-bold"
              rightIcon={<ArrowRight className="w-4 h-4" />}
            >
              Start Shopping Now
            </Button>
            {selectedStore && (
              <div className="hidden sm:flex items-center gap-2 px-4 py-3 rounded-xl bg-black/20 backdrop-blur-md text-xs font-medium text-emerald-100 border border-white/10">
                <MapPin className="w-4 h-4 text-emerald-300 flex-shrink-0" />
                <span>Delivering from {selectedStore.name}</span>
              </div>
            )}
          </div>
        </div>
      </section>

      {/* Featured Categories */}
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl sm:text-2xl font-black text-slate-900 tracking-tight">
              Explore Categories
            </h2>
            <p className="text-xs sm:text-sm text-slate-500 mt-0.5">
              Curated everyday essentials for quick delivery
            </p>
          </div>
          <Link
            to="/catalog"
            className="text-xs sm:text-sm font-bold text-brand-600 hover:text-brand-700 flex items-center gap-1 transition"
          >
            <span>View All</span>
            <ArrowRight className="w-4 h-4" />
          </Link>
        </div>

        {isLoading ? (
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="h-24 bg-slate-200/70 rounded-2xl animate-pulse" />
            ))}
          </div>
        ) : categories.length === 0 ? (
          <div className="p-8 text-center bg-white rounded-2xl border border-slate-100 shadow-card text-xs text-slate-500">
            No categories available. Select "Enable Demo Preview Mode" above or seed categories via API.
          </div>
        ) : (
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-3 sm:gap-4">
            {categories.map((cat) => (
              <button
                key={cat.id}
                onClick={() => navigate(`/catalog?category=${cat.id}`)}
                className="p-4 rounded-2xl bg-white border border-slate-100 hover:border-brand-200 shadow-card hover:shadow-card-hover transition-all duration-200 flex flex-col items-center text-center group text-slate-700 hover:text-brand-700"
              >
                <div className="w-12 h-12 rounded-xl bg-brand-50 group-hover:bg-brand-100/80 flex items-center justify-center text-brand-600 mb-2.5 transition">
                  <ShoppingBag className="w-6 h-6" />
                </div>
                <span className="text-xs font-bold leading-snug line-clamp-2">
                  {cat.name}
                </span>
              </button>
            ))}
          </div>
        )}
      </section>

      {/* Featured Products Grid */}
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl sm:text-2xl font-black text-slate-900 tracking-tight">
              Featured In-Store Items
            </h2>
            <p className="text-xs sm:text-sm text-slate-500 mt-0.5">
              {selectedStore ? `Available at ${selectedStore.name}` : 'Select a store to view stock'}
            </p>
          </div>
          <Link
            to="/catalog"
            className="text-xs sm:text-sm font-bold text-brand-600 hover:text-brand-700 flex items-center gap-1 transition"
          >
            <span>See More</span>
            <ArrowRight className="w-4 h-4" />
          </Link>
        </div>

        {isLoading ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <ProductSkeleton key={i} />
            ))}
          </div>
        ) : products.length === 0 ? (
          <div className="p-10 text-center bg-white rounded-2xl border border-slate-100 shadow-card">
            <ShoppingBag className="w-10 h-10 text-slate-300 mx-auto mb-3" />
            <h3 className="text-base font-bold text-slate-900 mb-1">
              No products found in this store
            </h3>
            <p className="text-xs text-slate-500 max-w-sm mx-auto mb-4">
              The production catalog has no inventory allocated for this store. Toggle "Enable Demo Preview Mode" above or choose another dark store.
            </p>
            {stores.length > 1 && (
              <Button size="sm" variant="outline" onClick={() => navigate('/catalog')}>
                Change Dark Store
              </Button>
            )}
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4 sm:gap-5">
            {products.map((p) => (
              <ProductCard
                key={p.product_id}
                product={p}
                storeId={selectedStore?.id || 'demo-store'}
                onViewDetails={(prod) => setSelectedProduct(prod)}
              />
            ))}
          </div>
        )}
      </section>

      {/* Shopping Benefits & SLA Section */}
      <section className="rounded-3xl bg-white border border-slate-100 p-8 sm:p-12 shadow-card">
        <div className="max-w-2xl mx-auto text-center mb-10">
          <span className="text-xs font-bold text-brand-600 uppercase tracking-widest block mb-2">
            Why Shop QuickCart
          </span>
          <h2 className="text-2xl sm:text-3xl font-black text-slate-900 tracking-tight">
            Engineered for Ultra-Fast Micro-Fulfillment
          </h2>
          <p className="text-sm text-slate-500 mt-2">
            Powered by high-throughput concurrent Go engine with zero overselling guarantees.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
          <div className="p-5 rounded-2xl bg-slate-50/70 border border-slate-100">
            <div className="w-10 h-10 rounded-xl bg-emerald-100 text-emerald-700 flex items-center justify-center mb-3">
              <Clock className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-slate-900 text-sm mb-1">10-Minute Delivery</h3>
            <p className="text-xs text-slate-500 leading-relaxed">
              Dispatched from hyper-local fulfillment centers positioned within 2-3 km of your door.
            </p>
          </div>

          <div className="p-5 rounded-2xl bg-slate-50/70 border border-slate-100">
            <div className="w-10 h-10 rounded-xl bg-emerald-100 text-emerald-700 flex items-center justify-center mb-3">
              <ShieldCheck className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-slate-900 text-sm mb-1">Zero Overselling</h3>
            <p className="text-xs text-slate-500 leading-relaxed">
              Deterministic row locks ensure you never get charged for an item that is out of stock.
            </p>
          </div>

          <div className="p-5 rounded-2xl bg-slate-50/70 border border-slate-100">
            <div className="w-10 h-10 rounded-xl bg-emerald-100 text-emerald-700 flex items-center justify-center mb-3">
              <TrendingUp className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-slate-900 text-sm mb-1">Live Order Tracking</h3>
            <p className="text-xs text-slate-500 leading-relaxed">
              Real-time Server-Sent Events stream updates you as items are picked, packed, and en route.
            </p>
          </div>

          <div className="p-5 rounded-2xl bg-slate-50/70 border border-slate-100">
            <div className="w-10 h-10 rounded-xl bg-emerald-100 text-emerald-700 flex items-center justify-center mb-3">
              <Sparkles className="w-5 h-5" />
            </div>
            <h3 className="font-bold text-slate-900 text-sm mb-1">Guaranteed Freshness</h3>
            <p className="text-xs text-slate-500 leading-relaxed">
              All vegetables, fruits, and dairy stored in temperature-controlled dark store pods.
            </p>
          </div>
        </div>
      </section>

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
