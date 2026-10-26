import React, { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { X, ShoppingBag, Plus, Minus, Trash2, ArrowRight, ShieldCheck, Zap } from 'lucide-react';
import { useCart } from '../../context/CartContext';
import { useAuth } from '../../context/AuthContext';
import { formatPaise } from '../../utils/currency';
import { Button } from '../ui/Button';
import { Modal } from '../ui/Modal';

export const CartDrawer: React.FC = () => {
  const {
    items,
    itemCount,
    totalAmountPaise,
    isLoading,
    isCartOpen,
    closeCart,
    updateQuantity,
    removeItem,
    clearCart,
    crossStoreConflict,
    resolveConflict,
  } = useCart();

  const { isAuthenticated } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isCartOpen) {
        closeCart();
      }
    };
    if (isCartOpen) {
      document.body.style.overflow = 'hidden';
      window.addEventListener('keydown', handleKeyDown);
    }
    return () => {
      document.body.style.overflow = '';
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isCartOpen, closeCart]);

  const handleProceedToCheckout = () => {
    closeCart();
    if (!isAuthenticated) {
      navigate('/login?redirect=/checkout');
    } else {
      navigate('/checkout');
    }
  };

  return (
    <>
      {/* Slide-over Drawer Backdrop */}
      {isCartOpen && (
        <div
          className="fixed inset-0 bg-slate-900/50 backdrop-blur-xs z-50 transition-opacity animate-fade-in"
          onClick={closeCart}
          aria-hidden="true"
        />
      )}

      {/* Cart Drawer Panel */}
      <div
        className={`fixed inset-y-0 right-0 max-w-full w-full sm:w-[420px] bg-white z-50 shadow-2xl flex flex-col transform transition-transform duration-300 ease-in-out ${
          isCartOpen ? 'translate-x-0' : 'translate-x-full'
        }`}
        role="dialog"
        aria-label="Shopping Cart"
      >
        {/* Header */}
        <div className="p-4 sm:p-5 border-b border-slate-100 flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="p-2 bg-brand-50 rounded-xl text-brand-600">
              <ShoppingBag className="w-5 h-5" />
            </div>
            <div>
              <h3 className="font-bold text-slate-900 leading-tight">My Cart</h3>
              <p className="text-xs text-slate-500">
                {itemCount} {itemCount === 1 ? 'item' : 'items'}
              </p>
            </div>
          </div>
          <button
            onClick={closeCart}
            className="p-2 text-slate-400 hover:text-slate-600 rounded-xl hover:bg-slate-100 transition"
            aria-label="Close cart"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content Body */}
        <div className="flex-1 overflow-y-auto p-4 sm:p-5 divide-y divide-slate-100">
          {items.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-center p-6">
              <div className="w-20 h-20 rounded-3xl bg-brand-50/80 border border-brand-100 text-brand-600 flex items-center justify-center mb-4">
                <ShoppingBag className="w-10 h-10 opacity-70" />
              </div>
              <h4 className="font-bold text-slate-900 text-base mb-1">Your cart is empty</h4>
              <p className="text-xs text-slate-500 max-w-xs mb-6">
                Explore our catalog to add fresh groceries and household essentials delivered in 10 minutes.
              </p>
              <Button
                size="sm"
                variant="primary"
                onClick={() => {
                  closeCart();
                  navigate('/catalog');
                }}
              >
                Browse Catalog
              </Button>
            </div>
          ) : (
            <div className="space-y-3 py-2">
              <div className="flex items-center justify-between pb-2">
                <span className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
                  Store Items
                </span>
                <button
                  onClick={clearCart}
                  className="text-xs text-rose-600 hover:text-rose-700 font-medium flex items-center gap-1 transition"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                  Clear Cart
                </button>
              </div>

              {items.map((item) => (
                <div
                  key={item.product_id}
                  className="flex items-center justify-between gap-3 p-3 rounded-xl bg-slate-50/70 border border-slate-100 hover:border-slate-200 transition"
                >
                  <div className="flex-1 min-w-0">
                    <h5 className="text-sm font-semibold text-slate-900 truncate">
                      {item.product_name}
                    </h5>
                    <div className="text-xs text-slate-500 mt-0.5">
                      {formatPaise(item.unit_price_paise)} each
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    <div className="inline-flex items-center bg-white border border-slate-200 rounded-lg overflow-hidden shadow-2xs">
                      <button
                        onClick={() => updateQuantity(item.product_id, item.quantity - 1)}
                        className="p-1 text-slate-600 hover:bg-slate-100 transition active:scale-90"
                        aria-label="Decrease quantity"
                      >
                        <Minus className="w-3 h-3" />
                      </button>
                      <span className="px-2 text-xs font-bold text-slate-800 min-w-[1.25rem] text-center">
                        {item.quantity}
                      </span>
                      <button
                        onClick={() => updateQuantity(item.product_id, item.quantity + 1)}
                        className="p-1 text-slate-600 hover:bg-slate-100 transition active:scale-90"
                        aria-label="Increase quantity"
                      >
                        <Plus className="w-3 h-3" />
                      </button>
                    </div>

                    <div className="text-right min-w-[4.5rem]">
                      <span className="text-sm font-bold text-slate-900 block">
                        {formatPaise(item.total_price_paise)}
                      </span>
                      <button
                        onClick={() => removeItem(item.product_id)}
                        className="text-[11px] text-slate-400 hover:text-rose-600 transition"
                      >
                        Remove
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Footer & Bill Summary */}
        {items.length > 0 && (
          <div className="p-4 sm:p-5 border-t border-slate-100 bg-slate-50/60 flex flex-col gap-3">
            {/* Delivery Assurance */}
            <div className="p-2.5 rounded-xl bg-emerald-50 border border-emerald-100 flex items-center gap-2 text-xs text-emerald-800">
              <Zap className="w-4 h-4 text-emerald-600 flex-shrink-0" />
              <span className="font-medium">Free 10-Minute Instant Delivery Applied</span>
            </div>

            <div className="space-y-1.5 text-xs text-slate-600">
              <div className="flex justify-between">
                <span>Items Subtotal</span>
                <span className="font-semibold text-slate-900">{formatPaise(totalAmountPaise)}</span>
              </div>
              <div className="flex justify-between">
                <span>Delivery SLA Fee</span>
                <span className="text-emerald-700 font-semibold">FREE</span>
              </div>
              <div className="flex justify-between">
                <span>Packaging & Handling</span>
                <span className="text-emerald-700 font-semibold">FREE</span>
              </div>
              <div className="flex justify-between pt-2 border-t border-slate-200 text-sm font-bold text-slate-900">
                <span>To Pay</span>
                <span>{formatPaise(totalAmountPaise)}</span>
              </div>
            </div>

            <Button
              variant="primary"
              size="lg"
              className="w-full mt-1"
              isLoading={isLoading}
              onClick={handleProceedToCheckout}
              rightIcon={<ArrowRight className="w-4 h-4" />}
            >
              Proceed to Checkout
            </Button>
            <div className="flex items-center justify-center gap-1.5 text-[11px] text-slate-400">
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-600" />
              <span>Atomic inventory lock guaranteed upon checkout</span>
            </div>
          </div>
        )}
      </div>

      {/* Cross-Store Conflict Resolution Modal */}
      <Modal
        isOpen={!!crossStoreConflict}
        onClose={() => resolveConflict(false)}
        title="Single-Store Order Requirement"
        description="Quick-commerce orders are packed directly from a single fulfillment dark store."
      >
        <div className="space-y-4">
          <p className="text-sm text-slate-600 leading-relaxed">
            Your cart currently contains items from a different dark store. Adding{' '}
            <strong className="text-slate-900">{crossStoreConflict?.productName}</strong> requires
            clearing your existing items to fulfill from this new store.
          </p>

          <div className="flex items-center justify-end gap-3 pt-3">
            <Button variant="outline" size="sm" onClick={() => resolveConflict(false)}>
              Keep Existing Cart
            </Button>
            <Button variant="danger" size="sm" onClick={() => resolveConflict(true)}>
              Clear Cart & Add Item
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
};
