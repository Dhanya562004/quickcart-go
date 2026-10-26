import React from 'react';
import { Plus, Minus, ShoppingBag, Eye } from 'lucide-react';
import { StoreProduct } from '../../api/types';
import { formatPaise } from '../../utils/currency';
import { useCart } from '../../context/CartContext';
import { Badge } from '../ui/Badge';
import { Card } from '../ui/Card';

interface ProductCardProps {
  product: StoreProduct;
  storeId: string;
  onViewDetails?: (product: StoreProduct) => void;
}

export const ProductCard: React.FC<ProductCardProps> = ({
  product,
  storeId,
  onViewDetails,
}) => {
  const { items, addItem, updateQuantity } = useCart();

  const cartItem = items.find((i) => i.product_id === product.product_id);
  const inCartQuantity = cartItem ? cartItem.quantity : 0;
  const isOutOfStock = product.available_quantity <= 0;
  const isLowStock = product.available_quantity > 0 && product.available_quantity <= 10;

  // Curated color themes for food product placeholders
  const getPlaceholderBg = (name: string) => {
    const char = name.charCodeAt(0) % 5;
    const gradients = [
      'from-emerald-50 to-teal-100/70 text-emerald-800',
      'from-amber-50 to-orange-100/70 text-amber-800',
      'from-sky-50 to-blue-100/70 text-sky-800',
      'from-rose-50 to-red-100/70 text-rose-800',
      'from-purple-50 to-indigo-100/70 text-purple-800',
    ];
    return gradients[char];
  };

  return (
    <Card hoverable className="flex flex-col h-full overflow-hidden group">
      {/* Product Image Box */}
      <div
        className={`relative h-44 w-full bg-gradient-to-br ${getPlaceholderBg(
          product.name
        )} p-4 flex flex-col justify-between items-center transition-transform duration-300 group-hover:scale-[1.01]`}
      >
        <div className="w-full flex items-center justify-between">
          <Badge
            variant={isOutOfStock ? 'danger' : isLowStock ? 'warning' : 'brand'}
            size="sm"
          >
            {isOutOfStock
              ? 'Out of Stock'
              : isLowStock
              ? `Only ${product.available_quantity} left`
              : `In Stock: ${product.available_quantity}`}
          </Badge>

          {onViewDetails && (
            <button
              onClick={() => onViewDetails(product)}
              className="p-1.5 rounded-full bg-white/80 hover:bg-white text-slate-600 hover:text-slate-900 shadow-xs transition backdrop-blur-xs opacity-0 group-hover:opacity-100"
              title="Quick View"
              aria-label="View product details"
            >
              <Eye className="w-4 h-4" />
            </button>
          )}
        </div>

        <div className="flex flex-col items-center justify-center my-auto">
          <div className="w-16 h-16 rounded-2xl bg-white/70 backdrop-blur-xs flex items-center justify-center shadow-xs border border-white/80 text-brand-600 mb-2">
            <ShoppingBag className="w-8 h-8 opacity-80" />
          </div>
          {product.unit && (
            <span className="text-xs font-semibold px-2 py-0.5 rounded-md bg-white/80 text-slate-700 shadow-2xs">
              {product.unit}
            </span>
          )}
        </div>

        <div className="w-full text-right">
          <span className="text-[10px] font-mono text-slate-500 uppercase tracking-wider">
            {product.sku}
          </span>
        </div>
      </div>

      {/* Card Content */}
      <div className="p-4 flex-1 flex flex-col justify-between">
        <div>
          <h4
            className="text-sm font-semibold text-slate-900 line-clamp-2 mb-1 group-hover:text-brand-600 transition"
            title={product.name}
          >
            {product.name}
          </h4>
        </div>

        <div className="flex items-center justify-between pt-3 mt-auto border-t border-slate-50">
          <div>
            <span className="text-xs text-slate-400 block font-medium">Price</span>
            <span className="text-base font-bold text-slate-900">
              {formatPaise(product.price_paise)}
            </span>
          </div>

          <div>
            {inCartQuantity > 0 ? (
              <div className="inline-flex items-center bg-brand-50 border border-brand-200 rounded-xl overflow-hidden shadow-2xs">
                <button
                  onClick={() => updateQuantity(product.product_id, inCartQuantity - 1)}
                  className="p-1.5 text-brand-700 hover:bg-brand-100 transition active:scale-90"
                  aria-label="Decrease quantity"
                >
                  <Minus className="w-3.5 h-3.5" />
                </button>
                <span className="px-2 text-xs font-bold text-brand-900 min-w-[1.5rem] text-center">
                  {inCartQuantity}
                </span>
                <button
                  onClick={() =>
                    inCartQuantity < product.available_quantity
                      ? updateQuantity(product.product_id, inCartQuantity + 1)
                      : null
                  }
                  disabled={inCartQuantity >= product.available_quantity}
                  className="p-1.5 text-brand-700 hover:bg-brand-100 transition disabled:opacity-40 active:scale-90"
                  aria-label="Increase quantity"
                >
                  <Plus className="w-3.5 h-3.5" />
                </button>
              </div>
            ) : (
              <button
                onClick={() => addItem(storeId, product.product_id, product.name, 1)}
                disabled={isOutOfStock}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-brand-50 text-brand-700 hover:bg-brand-600 hover:text-white border border-brand-200/80 hover:border-brand-600 text-xs font-bold transition-all duration-200 disabled:opacity-40 disabled:cursor-not-allowed active:scale-95 shadow-2xs"
              >
                <Plus className="w-3.5 h-3.5" />
                <span>ADD</span>
              </button>
            )}
          </div>
        </div>
      </div>
    </Card>
  );
};
