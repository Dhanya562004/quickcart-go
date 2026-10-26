import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import * as z from 'zod';
import {
  MapPin,
  ShoppingBag,
  ArrowRight,
  AlertCircle,
  Clock,
  Lock,
} from 'lucide-react';
import { useCart } from '../context/CartContext';
import { useAuth } from '../context/AuthContext';
import { checkout } from '../api/orders';
import { formatPaise } from '../utils/currency';
import { generateIdempotencyKey } from '../utils/idempotency';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { getApiErrorMessage } from '../api/client';
import axios from 'axios';

const checkoutSchema = z.object({
  delivery_address: z
    .string()
    .min(10, 'Please provide a complete delivery address (flat/house no, street, locality, pin code)')
    .max(500, 'Address is too long'),
});

type CheckoutFormValues = z.infer<typeof checkoutSchema>;

export const CheckoutPage: React.FC = () => {
  const { items, totalAmountPaise, refetchCart } = useCart();
  const { user } = useAuth();
  const navigate = useNavigate();

  const [apiError, setApiError] = useState<string | null>(null);
  const [stockConflict, setStockConflict] = useState<{
    productName: string;
    requested: number;
    available: number;
  } | null>(null);

  // Generate or hold idempotency key for this checkout attempt
  const [idempotencyKey] = useState<string>(() =>
    generateIdempotencyKey(`idemp_${user?.id?.slice(0, 8) || 'user'}`)
  );

  const {
    register,
    handleSubmit,
    setValue,
    formState: { errors, isSubmitting },
  } = useForm<CheckoutFormValues>({
    resolver: zodResolver(checkoutSchema),
    defaultValues: {
      delivery_address: '',
    },
  });

  const onSubmit = async (values: CheckoutFormValues) => {
    setApiError(null);
    setStockConflict(null);

    if (items.length === 0) {
      setApiError('Your cart is empty. Please add items before checking out.');
      return;
    }

    try {
      const order = await checkout(idempotencyKey, {
        delivery_address: values.delivery_address.trim(),
      });

      // Refetch empty cart state after atomic checkout reservation
      await refetchCart();

      // Navigate to payment page for placed order
      navigate(`/orders/${order.id}/pay`);
    } catch (err: unknown) {
      if (axios.isAxiosError(err) && err.response?.status === 409) {
        const data = err.response.data as any;
        setStockConflict({
          productName: data?.product_name || 'An item in your cart',
          requested: data?.requested || 1,
          available: data?.available || 0,
        });
        return;
      }
      setApiError(getApiErrorMessage(err));
    }
  };

  const handleFillSampleAddress = () => {
    setValue(
      'delivery_address',
      'Flat 402, Sunshine Heights, 100ft Road, Indiranagar, Bengaluru 560038'
    );
  };

  if (items.length === 0) {
    return (
      <div className="max-w-md mx-auto py-12 text-center">
        <div className="w-16 h-16 rounded-2xl bg-brand-50 text-brand-600 flex items-center justify-center mx-auto mb-4">
          <ShoppingBag className="w-8 h-8" />
        </div>
        <h2 className="text-xl font-bold text-slate-900 mb-2">Your cart is empty</h2>
        <p className="text-sm text-slate-500 mb-6">
          Add items to your cart before proceeding to atomic checkout.
        </p>
        <Link to="/catalog">
          <Button variant="primary" size="md">
            Browse Product Catalog
          </Button>
        </Link>
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto py-6 sm:py-10 space-y-8">
      {/* Checkout Title */}
      <div>
        <span className="text-xs font-bold text-brand-600 uppercase tracking-widest block mb-1">
          Review & Reserve
        </span>
        <h1 className="text-2xl sm:text-3xl font-black text-slate-900 tracking-tight">
          Checkout & Inventory Reservation
        </h1>
        <p className="text-xs sm:text-sm text-slate-500 mt-1">
          Atomic PostgreSQL row locking reserves your stock for 10 minutes upon confirmation.
        </p>
      </div>

      {/* Stock Conflict Warning */}
      {stockConflict && (
        <div className="p-4 rounded-2xl bg-amber-50 border border-amber-200 text-amber-900 text-sm flex items-start gap-3">
          <AlertCircle className="w-5 h-5 text-amber-600 flex-shrink-0 mt-0.5" />
          <div className="space-y-1">
            <h4 className="font-bold">Insufficient Dark Store Stock</h4>
            <p className="text-xs leading-relaxed text-amber-800">
              Another customer reserved items during checkout.{' '}
              <strong>{stockConflict.productName}</strong> has only{' '}
              <strong>{stockConflict.available}</strong> available (you requested{' '}
              {stockConflict.requested}). Please adjust your cart quantity to proceed.
            </p>
            <div className="pt-2">
              <Link to="/catalog">
                <Button size="sm" variant="outline" className="bg-white">
                  Adjust Cart Quantities
                </Button>
              </Link>
            </div>
          </div>
        </div>
      )}

      {/* Generic API Error */}
      {apiError && (
        <div className="p-4 rounded-2xl bg-rose-50 border border-rose-200 text-rose-800 text-xs flex items-start gap-3">
          <AlertCircle className="w-5 h-5 text-rose-600 flex-shrink-0 mt-0.5" />
          <div className="leading-relaxed">{apiError}</div>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        {/* Left Form: Delivery Address */}
        <div className="lg:col-span-7 space-y-6">
          <Card className="p-6">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-sm font-bold text-slate-900 uppercase tracking-wider flex items-center gap-2">
                <MapPin className="w-4 h-4 text-brand-600" />
                Delivery Address
              </h2>
              <button
                type="button"
                onClick={handleFillSampleAddress}
                className="text-xs text-brand-600 hover:text-brand-700 font-semibold transition"
              >
                Use Sample Address
              </button>
            </div>

            <form id="checkout-form" onSubmit={handleSubmit(onSubmit)} className="space-y-4">
              <div>
                <label className="block text-xs font-semibold text-slate-700 mb-1.5">
                  Full Street Address & Pin Code
                </label>
                <textarea
                  rows={3}
                  {...register('delivery_address')}
                  placeholder="Flat 402, Sunshine Heights, 100ft Road, Indiranagar, Bengaluru 560038"
                  className="w-full p-3 bg-slate-50 text-sm text-slate-900 rounded-xl border border-slate-200 focus:bg-white focus:outline-none focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 transition resize-none placeholder:text-slate-400"
                />
                {errors.delivery_address && (
                  <p className="text-xs text-rose-600 mt-1">
                    {errors.delivery_address.message}
                  </p>
                )}
              </div>

              {/* Delivery SLA reassurance */}
              <div className="p-3.5 rounded-xl bg-slate-50 border border-slate-100 flex items-center gap-2.5 text-xs text-slate-600">
                <Clock className="w-4 h-4 text-emerald-600 flex-shrink-0" />
                <span>
                  Delivery SLA: <strong>10 minutes</strong> from nearest dark store
                </span>
              </div>

              <div className="text-[11px] font-mono text-slate-400 bg-slate-50 p-2.5 rounded-xl border border-slate-100 flex items-center justify-between">
                <span>Idempotency-Key:</span>
                <span className="truncate max-w-[220px] font-semibold text-slate-600">
                  {idempotencyKey}
                </span>
              </div>
            </form>
          </Card>
        </div>

        {/* Right Summary: Order Items & Pricing */}
        <div className="lg:col-span-5 space-y-4">
          <Card className="p-6">
            <h2 className="text-sm font-bold text-slate-900 uppercase tracking-wider mb-4 flex items-center gap-2">
              <ShoppingBag className="w-4 h-4 text-brand-600" />
              Cart Summary ({items.length} items)
            </h2>

            <div className="divide-y divide-slate-100 max-h-60 overflow-y-auto pr-1 space-y-2 mb-4">
              {items.map((item) => (
                <div key={item.product_id} className="pt-2 flex justify-between items-center text-xs">
                  <div>
                    <span className="font-semibold text-slate-800 block truncate max-w-[180px]">
                      {item.product_name}
                    </span>
                    <span className="text-slate-400">
                      {formatPaise(item.unit_price_paise)} x {item.quantity}
                    </span>
                  </div>
                  <span className="font-bold text-slate-900">
                    {formatPaise(item.total_price_paise)}
                  </span>
                </div>
              ))}
            </div>

            <div className="pt-4 border-t border-slate-100 space-y-2 text-xs text-slate-600 mb-6">
              <div className="flex justify-between">
                <span>Subtotal</span>
                <span className="font-semibold text-slate-900">{formatPaise(totalAmountPaise)}</span>
              </div>
              <div className="flex justify-between">
                <span>Delivery SLA Fee</span>
                <span className="text-emerald-700 font-bold">FREE</span>
              </div>
              <div className="flex justify-between">
                <span>Handling Fee</span>
                <span className="text-emerald-700 font-bold">FREE</span>
              </div>
              <div className="flex justify-between pt-2 border-t border-slate-200 text-base font-extrabold text-slate-900">
                <span>Total Amount</span>
                <span>{formatPaise(totalAmountPaise)}</span>
              </div>
            </div>

            <Button
              type="submit"
              form="checkout-form"
              variant="primary"
              size="lg"
              className="w-full font-bold shadow-md shadow-brand-500/20"
              isLoading={isSubmitting}
              rightIcon={<ArrowRight className="w-4 h-4" />}
            >
              Confirm & Reserve Stock
            </Button>

            <div className="mt-3 flex items-center justify-center gap-1.5 text-[11px] text-slate-400 text-center">
              <Lock className="w-3.5 h-3.5 text-emerald-600" />
              <span>Deterministic ACID lock prevents race conditions</span>
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
};
