import React from 'react';
import { ShoppingBag, ShieldCheck, Zap, Package } from 'lucide-react';
import { StoreProduct } from '../../api/types';
import { formatPaise } from '../../utils/currency';
import { useCart } from '../../context/CartContext';
import { Modal } from '../ui/Modal';
import { Button } from '../ui/Button';
import { Badge } from '../ui/Badge';

interface ProductDetailModalProps {
  product: StoreProduct | null;
  storeId: string;
  isOpen: boolean;
  onClose: () => void;
}

export const ProductDetailModal: React.FC<ProductDetailModalProps> = ({
  product,
  storeId,
  isOpen,
  onClose,
}) => {
  const { items, addItem, updateQuantity } = useCart();

  if (!product) return null;

  const cartItem = items.find((i) => i.product_id === product.product_id);
  const inCartQuantity = cartItem ? cartItem.quantity : 0;
  const isOutOfStock = product.available_quantity <= 0;

  return (
    <Modal isOpen={isOpen} onClose={onClose} maxWidth="lg">
      <div className="flex flex-col md:flex-row gap-6">
        {/* Visual Box */}
        <div className="w-full md:w-1/2 h-56 md:h-64 rounded-2xl bg-gradient-to-br from-emerald-50 to-teal-100/60 p-6 flex flex-col items-center justify-center relative border border-emerald-100 shadow-xs">
          <div className="w-24 h-24 rounded-3xl bg-white/80 backdrop-blur-xs flex items-center justify-center shadow-card border border-white text-brand-600 mb-3">
            <ShoppingBag className="w-12 h-12" />
          </div>
          {product.unit && (
            <span className="text-xs font-semibold px-2.5 py-1 rounded-lg bg-white/90 text-slate-700 shadow-xs">
              {product.unit}
            </span>
          )}
        </div>

        {/* Details Box */}
        <div className="w-full md:w-1/2 flex flex-col justify-between">
          <div>
            <div className="flex items-center gap-2 mb-2">
              <Badge variant={isOutOfStock ? 'danger' : 'brand'} size="sm">
                {isOutOfStock ? 'Out of Stock' : `${product.available_quantity} available`}
              </Badge>
              <span className="text-xs font-mono text-slate-400">SKU: {product.sku}</span>
            </div>

            <h2 className="text-xl font-bold text-slate-900 mb-2 leading-snug">
              {product.name}
            </h2>

            <div className="text-2xl font-extrabold text-slate-900 mb-4">
              {formatPaise(product.price_paise)}
            </div>

            {/* Quick commerce benefits */}
            <div className="space-y-2 mb-6 text-xs text-slate-600">
              <div className="flex items-center gap-2">
                <Zap className="w-4 h-4 text-emerald-600" />
                <span>10-Minute Dark Store delivery SLA</span>
              </div>
              <div className="flex items-center gap-2">
                <ShieldCheck className="w-4 h-4 text-emerald-600" />
                <span>Zero overselling atomic stock reservation</span>
              </div>
              <div className="flex items-center gap-2">
                <Package className="w-4 h-4 text-emerald-600" />
                <span>Directly packed from climate-controlled dark store</span>
              </div>
            </div>
          </div>

          <div className="pt-4 border-t border-slate-100">
            {inCartQuantity > 0 ? (
              <div className="flex items-center justify-between gap-4">
                <span className="text-sm font-medium text-slate-600">In Cart:</span>
                <div className="inline-flex items-center bg-brand-50 border border-brand-200 rounded-xl overflow-hidden p-1">
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => updateQuantity(product.product_id, inCartQuantity - 1)}
                    className="h-8 w-8 p-0"
                  >
                    -
                  </Button>
                  <span className="px-3 text-sm font-bold text-brand-900">
                    {inCartQuantity}
                  </span>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => updateQuantity(product.product_id, inCartQuantity + 1)}
                    disabled={inCartQuantity >= product.available_quantity}
                    className="h-8 w-8 p-0"
                  >
                    +
                  </Button>
                </div>
              </div>
            ) : (
              <Button
                variant="primary"
                size="lg"
                className="w-full"
                disabled={isOutOfStock}
                onClick={() => {
                  addItem(storeId, product.product_id, product.name, 1);
                }}
              >
                {isOutOfStock ? 'Currently Unavailable' : 'Add to Shopping Cart'}
              </Button>
            )}
          </div>
        </div>
      </div>
    </Modal>
  );
};
