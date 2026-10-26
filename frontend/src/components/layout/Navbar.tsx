import React, { useState, useEffect } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import {
  ShoppingBag,
  User,
  MapPin,
  Search,
  Zap,
  Menu,
  X,
  Sparkles,
  ChevronDown,
  Activity,
  LogOut,
} from 'lucide-react';
import { useAuth } from '../../context/AuthContext';
import { useCart } from '../../context/CartContext';
import { useStore } from '../../context/StoreContext';
import { getHealth, getReadiness } from '../../api/system';

export const Navbar: React.FC = () => {
  const { user, isAuthenticated, logout } = useAuth();
  const { itemCount, openCart } = useCart();
  const { stores, selectedStore, setSelectedStore } = useStore();

  const [searchQuery, setSearchQuery] = useState('');
  const [isStorePickerOpen, setIsStorePickerOpen] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [backendStatus, setBackendStatus] = useState<'checking' | 'ready' | 'cold' | 'offline'>('checking');

  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    // Check backend health/readiness once on load
    async function checkStatus() {
      try {
        const ready = await getReadiness();
        if (ready.status === 'ready') {
          setBackendStatus('ready');
        } else {
          setBackendStatus('cold');
        }
      } catch (e) {
        try {
          await getHealth();
          setBackendStatus('cold');
        } catch {
          setBackendStatus('offline');
        }
      }
    }
    checkStatus();
  }, []);

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (searchQuery.trim()) {
      navigate(`/catalog?q=${encodeURIComponent(searchQuery.trim())}`);
      setIsMobileMenuOpen(false);
    }
  };

  return (
    <header className="sticky top-0 z-40 bg-white/95 backdrop-blur-md border-b border-slate-100 transition-all shadow-xs">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex items-center justify-between h-16 sm:h-20 gap-4">
          {/* Logo & Store Selector */}
          <div className="flex items-center gap-4 sm:gap-6">
            <Link to="/" className="flex items-center gap-2.5 group">
              <div className="w-10 h-10 rounded-2xl bg-gradient-to-tr from-brand-600 to-emerald-400 flex items-center justify-center text-white shadow-md shadow-brand-500/20 group-hover:scale-105 transition-transform duration-200">
                <Zap className="w-5 h-5 fill-white" />
              </div>
              <div className="flex flex-col">
                <span className="text-xl font-black tracking-tight text-slate-900 group-hover:text-brand-600 transition">
                  Quick<span className="text-brand-600">Cart</span>
                </span>
                <span className="text-[10px] font-bold text-slate-400 tracking-wider -mt-1 uppercase">
                  10-Min Delivery
                </span>
              </div>
            </Link>

            {/* Dark Store Selector */}
            <div className="relative hidden md:block">
              <button
                onClick={() => setIsStorePickerOpen(!isStorePickerOpen)}
                className="flex items-center gap-2 px-3 py-1.5 rounded-xl bg-slate-50 hover:bg-slate-100/80 border border-slate-200/80 transition text-left"
                aria-expanded={isStorePickerOpen}
              >
                <MapPin className="w-4 h-4 text-brand-600 flex-shrink-0" />
                <div className="flex flex-col max-w-[160px] truncate">
                  <span className="text-[10px] font-semibold text-slate-400 uppercase tracking-wider">
                    Dark Store
                  </span>
                  <span className="text-xs font-bold text-slate-800 truncate">
                    {selectedStore ? selectedStore.name : 'Select Store'}
                  </span>
                </div>
                <ChevronDown className="w-3.5 h-3.5 text-slate-400 ml-1" />
              </button>

              {/* Store Dropdown */}
              {isStorePickerOpen && (
                <div className="absolute left-0 mt-2 w-72 bg-white rounded-2xl shadow-xl border border-slate-100 p-2 z-50 animate-scale-in">
                  <div className="px-3 py-2 text-xs font-semibold text-slate-400 uppercase border-b border-slate-100">
                    Fulfillment Centers ({stores.length})
                  </div>
                  <div className="max-h-60 overflow-y-auto py-1">
                    {stores.length === 0 ? (
                      <div className="p-3 text-xs text-slate-500 text-center">
                        No active stores registered in production.
                      </div>
                    ) : (
                      stores.map((store) => (
                        <button
                          key={store.id}
                          onClick={() => {
                            setSelectedStore(store);
                            setIsStorePickerOpen(false);
                          }}
                          className={`w-full text-left p-2.5 rounded-xl text-xs transition flex items-center justify-between ${
                            selectedStore?.id === store.id
                              ? 'bg-brand-50 text-brand-800 font-bold'
                              : 'hover:bg-slate-50 text-slate-700'
                          }`}
                        >
                          <div className="truncate">
                            <div className="font-semibold">{store.name}</div>
                            <div className="text-[10px] text-slate-400 font-normal">
                              Lat: {store.latitude.toFixed(2)}, Lon: {store.longitude.toFixed(2)}
                            </div>
                          </div>
                          {selectedStore?.id === store.id && (
                            <span className="w-2 h-2 rounded-full bg-brand-600 flex-shrink-0" />
                          )}
                        </button>
                      ))
                    )}
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Search Bar */}
          <div className="hidden lg:flex flex-1 max-w-md mx-2">
            <form onSubmit={handleSearchSubmit} className="relative w-full">
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search milk, eggs, bananas, snacks..."
                className="w-full pl-10 pr-4 py-2.5 bg-slate-50 hover:bg-slate-100/70 focus:bg-white text-sm text-slate-900 rounded-xl border border-slate-200/80 focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500/20 transition-all placeholder:text-slate-400"
              />
              <Search className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
            </form>
          </div>

          {/* Right Navigation & Status Controls */}
          <div className="flex items-center gap-2 sm:gap-3">
            {/* Backend Status indicator */}
            <div
              className="hidden xl:flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] font-medium border"
              title="Render Web Service Connectivity"
            >
              {backendStatus === 'ready' && (
                <span className="flex items-center gap-1.5 text-emerald-700 bg-emerald-50 border-emerald-200/50 px-2 py-0.5 rounded-full">
                  <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
                  Engine Ready
                </span>
              )}
              {backendStatus === 'cold' && (
                <span className="flex items-center gap-1.5 text-amber-700 bg-amber-50 border-amber-200/50 px-2 py-0.5 rounded-full">
                  <Activity className="w-3 h-3 animate-spin text-amber-500" />
                  Warming Up
                </span>
              )}
              {backendStatus === 'offline' && (
                <span className="flex items-center gap-1.5 text-rose-700 bg-rose-50 border-rose-200/50 px-2 py-0.5 rounded-full">
                  <span className="w-2 h-2 rounded-full bg-rose-500" />
                  Offline
                </span>
              )}
            </div>

            {/* Catalog Link */}
            <Link
              to="/catalog"
              className={`hidden sm:inline-flex items-center gap-1 px-3 py-2 rounded-xl text-xs font-semibold transition ${
                location.pathname === '/catalog'
                  ? 'bg-brand-50 text-brand-700'
                  : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100'
              }`}
            >
              <Sparkles className="w-3.5 h-3.5 text-brand-600" />
              <span>Catalog</span>
            </Link>

            {/* Account / Login */}
            {isAuthenticated ? (
              <div className="flex items-center gap-1">
                <Link
                  to="/account"
                  className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-semibold text-slate-700 hover:bg-slate-100 transition"
                >
                  <User className="w-4 h-4 text-brand-600" />
                  <span className="max-w-[100px] truncate">{user?.full_name?.split(' ')[0]}</span>
                </Link>
                <button
                  onClick={() => logout()}
                  className="p-2 text-slate-400 hover:text-rose-600 rounded-xl hover:bg-slate-100 transition"
                  title="Sign Out"
                  aria-label="Sign out"
                >
                  <LogOut className="w-4 h-4" />
                </button>
              </div>
            ) : (
              <Link
                to="/login"
                className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl bg-slate-900 text-white hover:bg-slate-800 text-xs font-semibold transition shadow-xs"
              >
                <User className="w-3.5 h-3.5" />
                <span>Sign In</span>
              </Link>
            )}

            {/* Cart Button */}
            <button
              onClick={openCart}
              className="relative inline-flex items-center gap-2 px-3.5 py-2 rounded-xl bg-brand-600 hover:bg-brand-700 text-white text-xs font-bold transition shadow-sm hover:shadow-brand-500/20 active:scale-95"
              aria-label={`Open shopping cart with ${itemCount} items`}
            >
              <ShoppingBag className="w-4 h-4" />
              <span className="hidden sm:inline">Cart</span>
              {itemCount > 0 && (
                <span className="inline-flex items-center justify-center px-1.5 py-0.5 text-[11px] font-black bg-white text-brand-700 rounded-full min-w-[18px]">
                  {itemCount}
                </span>
              )}
            </button>

            {/* Mobile Menu Toggle */}
            <button
              onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
              className="lg:hidden p-2 text-slate-600 hover:text-slate-900 rounded-xl hover:bg-slate-100"
              aria-label="Toggle navigation menu"
            >
              {isMobileMenuOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
            </button>
          </div>
        </div>

        {/* Mobile Search & Navigation Dropdown */}
        {isMobileMenuOpen && (
          <div className="lg:hidden py-4 border-t border-slate-100 animate-slide-in space-y-3">
            <form onSubmit={handleSearchSubmit} className="relative w-full">
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="Search groceries..."
                className="w-full pl-10 pr-4 py-2 bg-slate-50 text-sm text-slate-900 rounded-xl border border-slate-200"
              />
              <Search className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
            </form>

            <div className="flex flex-col gap-1 pt-1">
              <Link
                to="/catalog"
                onClick={() => setIsMobileMenuOpen(false)}
                className="px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 rounded-xl flex items-center justify-between"
              >
                <span>Browse Products</span>
                <Sparkles className="w-4 h-4 text-brand-600" />
              </Link>
              {isAuthenticated ? (
                <>
                  <Link
                    to="/account"
                    onClick={() => setIsMobileMenuOpen(false)}
                    className="px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 rounded-xl flex items-center justify-between"
                  >
                    <span>My Orders & Account</span>
                    <User className="w-4 h-4 text-brand-600" />
                  </Link>
                  <button
                    onClick={() => {
                      setIsMobileMenuOpen(false);
                      logout();
                    }}
                    className="px-3 py-2 text-sm font-semibold text-rose-600 hover:bg-rose-50 rounded-xl text-left"
                  >
                    Sign Out
                  </button>
                </>
              ) : (
                <Link
                  to="/login"
                  onClick={() => setIsMobileMenuOpen(false)}
                  className="px-3 py-2 text-sm font-semibold text-brand-600 hover:bg-brand-50 rounded-xl"
                >
                  Sign In / Register
                </Link>
              )}
            </div>
          </div>
        )}
      </div>
    </header>
  );
};
