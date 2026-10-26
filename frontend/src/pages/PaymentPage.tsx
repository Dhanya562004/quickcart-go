import React, { useState, useEffect } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import {
  CreditCard,
  QrCode,
  Building,
  Wallet,
  ShieldCheck,
  AlertCircle,
  ArrowRight,
  Lock,
} from 'lucide-react';
import { getOrder } from '../api/orders';
import { payOrder } from '../api/payment';
import { Order, PaymentMethod } from '../api/types';
import { formatPaise } from '../utils/currency';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { Badge } from '../components/ui/Badge';
import { useToast } from '../context/ToastContext';
import { getApiErrorMessage } from '../api/client';

export const PaymentPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { success, error } = useToast();

  const [order, setOrder] = useState<Order | null>(null);
  const [isLoadingOrder, setIsLoadingOrder] = useState(true);
  const [selectedMethod, setSelectedMethod] = useState<PaymentMethod>('UPI');
  const [isProcessing, setIsProcessing] = useState(false);
  const [paymentError, setPaymentError] = useState<string | null>(null);

  useEffect(() => {
    async function fetchOrder() {
      if (!id) return;
      setIsLoadingOrder(true);
      try {
        const ord = await getOrder(id);
        setOrder(ord);

        // If order already paid or delivered, can route to tracker
        if (ord.status !== 'PLACED') {
          // already confirmed or cancelled
        }
      } catch (err) {
        setPaymentError(getApiErrorMessage(err));
      } finally {
        setIsLoadingOrder(false);
      }
    }
    fetchOrder();
  }, [id]);

  const handlePay = async () => {
    if (!order) return;
    setIsProcessing(true);
    setPaymentError(null);

    try {
      const payment = await payOrder(order.id, { method: selectedMethod });
      success(
        `Payment authorized successfully (Ref: ${payment.gateway_reference})!`,
        'Payment Captured'
      );
      navigate(`/orders/${order.id}/track`);
    } catch (err: unknown) {
      const msg = getApiErrorMessage(err);
      setPaymentError(msg);
      error(msg, 'Payment Failed');
    } finally {
      setIsProcessing(false);
    }
  };

  if (isLoadingOrder) {
    return (
      <div className="max-w-md mx-auto py-16 text-center space-y-3">
        <div className="w-10 h-10 border-4 border-brand-500 border-t-transparent rounded-full animate-spin mx-auto" />
        <p className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
          Retrieving order details...
        </p>
      </div>
    );
  }

  if (!order) {
    return (
      <div className="max-w-md mx-auto py-12 text-center">
        <h2 className="text-xl font-bold text-slate-900 mb-2">Order Not Found</h2>
        <p className="text-sm text-slate-500 mb-6">
          {paymentError || 'Unable to retrieve order details for payment.'}
        </p>
        <Link to="/orders">
          <Button variant="primary">Go to My Orders</Button>
        </Link>
      </div>
    );
  }

  const methods = [
    {
      id: 'UPI' as PaymentMethod,
      name: 'UPI (Google Pay / PhonePe / QR)',
      description: 'Instant zero-fee simulation with direct bank authorization',
      icon: <QrCode className="w-5 h-5 text-emerald-600" />,
    },
    {
      id: 'CARD' as PaymentMethod,
      name: 'Credit or Debit Card',
      description: 'Simulated sandbox card payment authorization',
      icon: <CreditCard className="w-5 h-5 text-sky-600" />,
    },
    {
      id: 'NETBANKING' as PaymentMethod,
      name: 'Net Banking',
      description: 'Supports major Indian commercial banks (HDFC, ICICI, SBI)',
      icon: <Building className="w-5 h-5 text-indigo-600" />,
    },
    {
      id: 'WALLET' as PaymentMethod,
      name: 'QuickCart Wallet',
      description: 'Fast one-click checkout balance simulation',
      icon: <Wallet className="w-5 h-5 text-amber-600" />,
    },
  ];

  return (
    <div className="max-w-2xl mx-auto py-6 sm:py-10 space-y-6">
      {/* Simulation Banner Notice */}
      <div className="p-4 rounded-2xl bg-amber-50 border border-amber-200/80 text-amber-900 text-xs flex items-start gap-3">
        <ShieldCheck className="w-5 h-5 text-amber-600 flex-shrink-0 mt-0.5" />
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="font-bold text-amber-950">Simulated Payment Environment</span>
            <Badge variant="warning" size="sm">Demo Mode</Badge>
          </div>
          <p className="leading-relaxed text-amber-800">
            This checkout simulates idempotent payment capture against QuickCart's fake payment gateway backend with full jitter and retries. No real monetary transactions will occur and no real card numbers are required.
          </p>
        </div>
      </div>

      {paymentError && (
        <div className="p-4 rounded-2xl bg-rose-50 border border-rose-200 text-rose-800 text-xs flex items-start gap-3">
          <AlertCircle className="w-5 h-5 text-rose-600 flex-shrink-0 mt-0.5" />
          <div className="leading-relaxed">{paymentError}</div>
        </div>
      )}

      {/* Order Info Summary */}
      <Card className="p-6">
        <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between pb-4 border-b border-slate-100 gap-3">
          <div>
            <span className="text-xs font-mono text-slate-400">Order Number</span>
            <h2 className="text-lg font-black text-slate-900">{order.order_number}</h2>
          </div>
          <div className="text-left sm:text-right">
            <span className="text-xs text-slate-400">Amount Due</span>
            <div className="text-2xl font-black text-slate-900">
              {formatPaise(order.total_amount_paise)}
            </div>
          </div>
        </div>

        <div className="pt-4 text-xs text-slate-500">
          <span className="font-medium text-slate-400 block mb-0.5">Delivering to:</span>
          <span className="text-slate-800">{order.delivery_address}</span>
        </div>
      </Card>

      {/* Payment Method Selector */}
      <Card className="p-6">
        <h3 className="text-sm font-bold text-slate-900 uppercase tracking-wider mb-4">
          Select Payment Method
        </h3>

        <div className="space-y-3">
          {methods.map((m) => (
            <label
              key={m.id}
              onClick={() => setSelectedMethod(m.id)}
              className={`flex items-start gap-3.5 p-4 rounded-xl border cursor-pointer transition select-none ${
                selectedMethod === m.id
                  ? 'border-brand-500 bg-brand-50/50 shadow-xs'
                  : 'border-slate-200 hover:border-slate-300 bg-white'
              }`}
            >
              <input
                type="radio"
                name="payment_method"
                checked={selectedMethod === m.id}
                onChange={() => setSelectedMethod(m.id)}
                className="mt-1 text-brand-600 focus:ring-brand-500 w-4 h-4"
              />
              <div className="p-2 rounded-xl bg-white border border-slate-100 shadow-2xs">
                {m.icon}
              </div>
              <div className="flex-1">
                <span className="font-bold text-slate-900 text-sm block">{m.name}</span>
                <span className="text-xs text-slate-500 leading-relaxed block mt-0.5">
                  {m.description}
                </span>
              </div>
            </label>
          ))}
        </div>

        <div className="mt-6 pt-5 border-t border-slate-100">
          <Button
            variant="primary"
            size="lg"
            className="w-full font-bold shadow-md shadow-brand-500/20"
            isLoading={isProcessing}
            onClick={handlePay}
            rightIcon={<ArrowRight className="w-4 h-4" />}
          >
            Authorize Payment of {formatPaise(order.total_amount_paise)}
          </Button>

          <div className="mt-3 flex items-center justify-center gap-1.5 text-[11px] text-slate-400 text-center">
            <Lock className="w-3.5 h-3.5 text-emerald-600" />
            <span>Coordinated with stable idempotency key and exponential backoff</span>
          </div>
        </div>
      </Card>
    </div>
  );
};
