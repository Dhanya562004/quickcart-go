import React, { useState, useEffect, useRef } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  Radio,
  CheckCircle2,
  Clock,
  Package,
  Bike,
  Home,
  RefreshCw,
  MapPin,
  ShieldCheck,
  CreditCard,
  Ban,
  Zap,
} from 'lucide-react';
import { getOrder } from '../api/orders';
import { createOrderEventStream, SSEConnectionState } from '../api/sse';
import { Order, OrderStatus, SSEOrderEvent } from '../api/types';
import { formatPaise } from '../utils/currency';
import { Card } from '../components/ui/Card';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { useToast } from '../context/ToastContext';
import { getApiErrorMessage } from '../api/client';

export const OrderTrackingPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [order, setOrder] = useState<Order | null>(null);
  const [isLoadingOrder, setIsLoadingOrder] = useState(true);
  const [connectionState, setConnectionState] = useState<SSEConnectionState>('connecting');
  const [lastHeartbeat, setLastHeartbeat] = useState<Date | null>(null);
  const [eventLogs, setEventLogs] = useState<Array<{ text: string; time: string }>>([]);
  const [orderError, setOrderError] = useState<string | null>(null);

  const { info, success } = useToast();
  const streamRef = useRef<{ close: () => void } | null>(null);

  // Manual fallback refresh
  const handleManualRefresh = async () => {
    if (!id) return;
    try {
      const refreshed = await getOrder(id);
      setOrder(refreshed);
      info('Order status refreshed from server.', 'Refreshed');
    } catch (err) {
      // Ignore manual error
    }
  };

  // Initial Order fetch
  useEffect(() => {
    async function fetchOrder() {
      if (!id) return;
      setIsLoadingOrder(true);
      try {
        const data = await getOrder(id);
        setOrder(data);
      } catch (err) {
        setOrderError(getApiErrorMessage(err));
      } finally {
        setIsLoadingOrder(false);
      }
    }
    fetchOrder();
  }, [id]);

  // Connect Server-Sent Events stream
  useEffect(() => {
    if (!id) return;

    const stream = createOrderEventStream({
      orderId: id,
      onEvent: (event: SSEOrderEvent) => {
        setOrder((prev) => (prev ? { ...prev, status: event.status } : null));
        setEventLogs((prev) => [
          {
            text: event.message || `Status changed to ${event.status}`,
            time: new Date().toLocaleTimeString(),
          },
          ...prev,
        ]);
        if (event.status === 'DELIVERED') {
          success('Your order has been delivered! Enjoy your fresh groceries.', 'Delivered');
        } else {
          info(`Order update: ${event.status}`, 'Status Update');
        }
      },
      onHeartbeat: () => {
        setLastHeartbeat(new Date());
      },
      onStateChange: (state) => {
        setConnectionState(state);
      },
      onError: () => {
        // Handled through state transitions
      },
    });

    streamRef.current = stream;

    // Cleanup and stream termination on component unmount
    return () => {
      stream.close();
      streamRef.current = null;
    };
  }, [id, info, success]);

  if (isLoadingOrder) {
    return (
      <div className="max-w-md mx-auto py-16 text-center space-y-3">
        <div className="w-10 h-10 border-4 border-brand-500 border-t-transparent rounded-full animate-spin mx-auto" />
        <p className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
          Connecting to live order tracking...
        </p>
      </div>
    );
  }

  if (!order) {
    return (
      <div className="max-w-md mx-auto py-12 text-center">
        <h2 className="text-xl font-bold text-slate-900 mb-2">Order Not Found</h2>
        <p className="text-sm text-slate-500 mb-6">
          {orderError || 'Unable to find or connect to the requested order tracking stream.'}
        </p>
        <Link to="/orders">
          <Button variant="primary">Return to Orders</Button>
        </Link>
      </div>
    );
  }

  const steps: Array<{
    status: OrderStatus;
    title: string;
    description: string;
    icon: React.ReactNode;
  }> = [
    {
      status: 'PLACED',
      title: 'Order Placed',
      description: 'Stock allocated in dark store',
      icon: <Clock className="w-5 h-5" />,
    },
    {
      status: 'CONFIRMED',
      title: 'Confirmed',
      description: 'Payment authorized & verified',
      icon: <ShieldCheck className="w-5 h-5" />,
    },
    {
      status: 'PACKING',
      title: 'Packing Order',
      description: 'Items collected in dark store',
      icon: <Package className="w-5 h-5" />,
    },
    {
      status: 'OUT_FOR_DELIVERY',
      title: 'Out for Delivery',
      description: 'Rider en route to your location',
      icon: <Bike className="w-5 h-5" />,
    },
    {
      status: 'DELIVERED',
      title: 'Delivered',
      description: 'Package handed over safely',
      icon: <Home className="w-5 h-5" />,
    },
  ];

  const statusOrder: Record<OrderStatus, number> = {
    PLACED: 0,
    CONFIRMED: 1,
    PACKING: 2,
    OUT_FOR_DELIVERY: 3,
    DELIVERED: 4,
    CANCELLED: -1,
  };

  const currentStepIndex = statusOrder[order.status];
  const isCancelled = order.status === 'CANCELLED';

  return (
    <div className="max-w-3xl mx-auto py-6 sm:py-10 space-y-6">
      {/* Tracker Header */}
      <div className="bg-white rounded-3xl p-6 sm:p-8 border border-slate-100 shadow-card flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <span className="font-mono text-sm font-bold text-slate-900">
              {order.order_number}
            </span>
            {isCancelled ? (
              <Badge variant="danger">Cancelled</Badge>
            ) : (
              <Badge
                variant={connectionState === 'connected' ? 'success' : 'warning'}
                icon={<Radio className="w-3 h-3 animate-pulse" />}
              >
                {connectionState === 'connected'
                  ? 'Live Stream Connected'
                  : connectionState === 'reconnecting'
                  ? 'Reconnecting...'
                  : 'Connecting...'}
              </Badge>
            )}
          </div>
          <h1 className="text-2xl sm:text-3xl font-black text-slate-900 tracking-tight">
            Live Delivery Tracking
          </h1>
          <p className="text-xs text-slate-500 mt-0.5">
            10-Minute SLA • Real-time Server-Sent Events updates
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={handleManualRefresh}
            leftIcon={<RefreshCw className="w-3.5 h-3.5" />}
          >
            Refresh Status
          </Button>
          {order.status === 'PLACED' && (
            <Link to={`/orders/${order.id}/pay`}>
              <Button size="sm" variant="primary" leftIcon={<CreditCard className="w-3.5 h-3.5" />}>
                Pay Now
              </Button>
            </Link>
          )}
        </div>
      </div>

      {/* Cancelled Banner */}
      {isCancelled && (
        <div className="p-5 rounded-2xl bg-rose-50 border border-rose-200 text-rose-900 flex items-start gap-3">
          <Ban className="w-5 h-5 text-rose-600 flex-shrink-0 mt-0.5" />
          <div>
            <h4 className="font-bold text-sm">This order has been cancelled</h4>
            <p className="text-xs text-rose-800 mt-1 leading-relaxed">
              Reserved stock was restored back to the dark store inventory. If you were charged, a refund is issued automatically.
            </p>
          </div>
        </div>
      )}

      {/* Progress Timeline Stepper */}
      {!isCancelled && (
        <Card className="p-6 sm:p-8">
          <div className="relative">
            <div className="space-y-8">
              {steps.map((step, idx) => {
                const isPassed = currentStepIndex > idx;
                const isCurrent = currentStepIndex === idx;

                return (
                  <div key={step.status} className="relative flex items-start gap-4">
                    {/* Connecting line */}
                    {idx < steps.length - 1 && (
                      <div
                        className={`absolute left-5 top-10 -ml-px w-0.5 h-12 transition-colors duration-500 ${
                          isPassed ? 'bg-brand-500' : 'bg-slate-200'
                        }`}
                        aria-hidden="true"
                      />
                    )}

                    {/* Step Icon */}
                    <div
                      className={`relative z-10 w-10 h-10 rounded-2xl flex items-center justify-center transition-all duration-300 shadow-xs ${
                        isPassed
                          ? 'bg-brand-600 text-white'
                          : isCurrent
                          ? 'bg-brand-500 text-white ring-4 ring-brand-100 scale-105'
                          : 'bg-slate-100 text-slate-400'
                      }`}
                    >
                      {isPassed ? <CheckCircle2 className="w-5 h-5" /> : step.icon}
                    </div>

                    {/* Step Text */}
                    <div className="flex-1 pt-1">
                      <div className="flex items-center gap-2">
                        <h4
                          className={`text-sm font-bold ${
                            isCurrent
                              ? 'text-brand-700'
                              : isPassed
                              ? 'text-slate-900'
                              : 'text-slate-400'
                          }`}
                        >
                          {step.title}
                        </h4>
                        {isCurrent && (
                          <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-black uppercase bg-brand-100 text-brand-800 animate-pulse">
                            Current Stage
                          </span>
                        )}
                      </div>
                      <p className="text-xs text-slate-500 mt-0.5">{step.description}</p>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </Card>
      )}

      {/* Delivery & Address Details Card */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Card className="p-6">
          <h3 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-3">
            Delivery Destination
          </h3>
          <div className="flex items-start gap-3 text-xs">
            <MapPin className="w-4 h-4 text-brand-600 mt-0.5 flex-shrink-0" />
            <div className="leading-relaxed font-semibold text-slate-800">
              {order.delivery_address}
            </div>
          </div>
        </Card>

        <Card className="p-6">
          <h3 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-3">
            Order Summary
          </h3>
          <div className="space-y-1.5 text-xs">
            <div className="flex justify-between">
              <span className="text-slate-500">Order ID</span>
              <span className="font-mono text-slate-700 truncate max-w-[150px]">{order.id}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500">Total Amount</span>
              <span className="font-bold text-slate-900 text-sm">
                {formatPaise(order.total_amount_paise)}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500">Delivery Guarantee</span>
              <span className="text-emerald-700 font-semibold">10-Minute SLA</span>
            </div>
          </div>
        </Card>
      </div>

      {/* Real-time SSE Live Event Stream Log */}
      <Card className="p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-xs font-bold text-slate-900 uppercase tracking-wider flex items-center gap-2">
            <Zap className="w-4 h-4 text-brand-600" />
            Live SSE Event Log
          </h3>
          {lastHeartbeat && (
            <span className="text-[10px] text-slate-400 font-mono">
              Last heartbeat: {lastHeartbeat.toLocaleTimeString()}
            </span>
          )}
        </div>

        <div className="bg-slate-950 text-slate-300 rounded-xl p-4 font-mono text-xs max-h-48 overflow-y-auto space-y-2">
          <div className="text-emerald-400 text-[11px]">
            [SSE Connected] Listening to /api/v1/orders/{order.id}/events via Bearer Header
          </div>
          {eventLogs.length === 0 ? (
            <div className="text-slate-500 text-[11px] italic">
              Awaiting state transitions from background worker engine...
            </div>
          ) : (
            eventLogs.map((log, idx) => (
              <div key={idx} className="flex items-start gap-2">
                <span className="text-slate-500">[{log.time}]</span>
                <span className="text-slate-200">{log.text}</span>
              </div>
            ))
          )}
        </div>
      </Card>
    </div>
  );
};
