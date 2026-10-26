import React, { useState, useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  Package,
  Clock,
  MapPin,
  ArrowRight,
  Radio,
  CreditCard,
  Ban,
  ShoppingBag,
} from 'lucide-react';
import { listOrders, cancelOrder } from '../api/orders';
import { Order, OrderStatus } from '../api/types';
import { formatPaise } from '../utils/currency';
import { Card } from '../components/ui/Card';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Modal } from '../components/ui/Modal';
import { EmptyState } from '../components/ui/EmptyState';
import { useToast } from '../context/ToastContext';
import { getApiErrorMessage } from '../api/client';

export const OrdersPage: React.FC = () => {
  const [orders, setOrders] = useState<Order[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [cancelModalOrder, setCancelModalOrder] = useState<Order | null>(null);
  const [cancelReason, setCancelReason] = useState('');
  const [isCancelling, setIsCancelling] = useState(false);

  const { success, error } = useToast();
  const navigate = useNavigate();

  const fetchOrders = async () => {
    setIsLoading(true);
    try {
      const data = await listOrders(50, 0);
      setOrders(data);
    } catch (err) {
      error(getApiErrorMessage(err), 'Failed to Load Orders');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchOrders();
  }, []);

  const handleCancelOrder = async () => {
    if (!cancelModalOrder) return;
    if (!cancelReason.trim()) {
      error('Please specify a cancellation reason.', 'Reason Required');
      return;
    }

    setIsCancelling(true);
    try {
      await cancelOrder(cancelModalOrder.id, cancelReason.trim());
      success('Order cancelled and reserved inventory restored to store.', 'Order Cancelled');
      setCancelModalOrder(null);
      setCancelReason('');
      await fetchOrders();
    } catch (err: unknown) {
      error(getApiErrorMessage(err), 'Cancellation Failed');
    } finally {
      setIsCancelling(false);
    }
  };

  const getStatusBadge = (status: OrderStatus) => {
    switch (status) {
      case 'PLACED':
        return <Badge variant="warning">Awaiting Payment</Badge>;
      case 'CONFIRMED':
        return <Badge variant="brand">Confirmed</Badge>;
      case 'PACKING':
        return <Badge variant="brand">Packing at Dark Store</Badge>;
      case 'OUT_FOR_DELIVERY':
        return <Badge variant="success" icon={<Radio className="w-3 h-3 animate-pulse" />}>Out for Delivery</Badge>;
      case 'DELIVERED':
        return <Badge variant="success">Delivered</Badge>;
      case 'CANCELLED':
        return <Badge variant="danger">Cancelled</Badge>;
      default:
        return <Badge variant="neutral">{status}</Badge>;
    }
  };

  return (
    <div className="max-w-4xl mx-auto py-6 sm:py-10 space-y-8">
      {/* Page Title */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <span className="text-xs font-bold text-brand-600 uppercase tracking-widest block mb-1">
            Activity & History
          </span>
          <h1 className="text-2xl sm:text-3xl font-black text-slate-900 tracking-tight">
            My Orders
          </h1>
          <p className="text-xs sm:text-sm text-slate-500 mt-1">
            Real-time delivery progress and order history
          </p>
        </div>
        <Link to="/catalog">
          <Button variant="outline" size="sm" leftIcon={<ShoppingBag className="w-4 h-4" />}>
            Browse More Items
          </Button>
        </Link>
      </div>

      {isLoading ? (
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-36 bg-slate-200/70 rounded-2xl animate-pulse" />
          ))}
        </div>
      ) : orders.length === 0 ? (
        <EmptyState
          icon={<Package className="w-8 h-8 text-slate-400" />}
          title="No orders placed yet"
          description="You haven't placed any orders. Browse our catalog and experience 10-minute micro-fulfillment delivery."
          actionLabel="Start Shopping"
          onAction={() => navigate('/catalog')}
        />
      ) : (
        <div className="space-y-4">
          {orders.map((order) => {
            const canCancel = order.status === 'PLACED' || order.status === 'CONFIRMED';
            const needsPayment = order.status === 'PLACED';

            return (
              <Card key={order.id} hoverable className="p-5 sm:p-6">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-slate-100">
                  <div className="space-y-1">
                    <div className="flex items-center gap-2.5 flex-wrap">
                      <span className="font-mono text-sm font-bold text-slate-900">
                        {order.order_number}
                      </span>
                      {getStatusBadge(order.status)}
                    </div>
                    <div className="flex items-center gap-3 text-xs text-slate-400">
                      <span className="flex items-center gap-1">
                        <Clock className="w-3.5 h-3.5" />
                        {new Date(order.created_at).toLocaleString('en-IN', {
                          month: 'short',
                          day: 'numeric',
                          hour: '2-digit',
                          minute: '2-digit',
                        })}
                      </span>
                    </div>
                  </div>

                  <div className="text-left sm:text-right">
                    <span className="text-xs text-slate-400 block font-medium">Total Paid</span>
                    <span className="text-lg font-black text-slate-900">
                      {formatPaise(order.total_amount_paise)}
                    </span>
                  </div>
                </div>

                <div className="py-3 text-xs text-slate-500 flex items-start gap-2">
                  <MapPin className="w-3.5 h-3.5 text-slate-400 mt-0.5 flex-shrink-0" />
                  <span className="line-clamp-1">{order.delivery_address}</span>
                </div>

                <div className="flex items-center justify-between pt-3 border-t border-slate-100 flex-wrap gap-2">
                  <div className="flex items-center gap-2">
                    {needsPayment && (
                      <Link to={`/orders/${order.id}/pay`}>
                        <Button
                          size="sm"
                          variant="primary"
                          leftIcon={<CreditCard className="w-3.5 h-3.5" />}
                        >
                          Complete Payment
                        </Button>
                      </Link>
                    )}

                    <Link to={`/orders/${order.id}/track`}>
                      <Button
                        size="sm"
                        variant={needsPayment ? 'outline' : 'primary'}
                        leftIcon={<Radio className="w-3.5 h-3.5" />}
                        rightIcon={<ArrowRight className="w-3.5 h-3.5" />}
                      >
                        Live Tracking
                      </Button>
                    </Link>
                  </div>

                  {canCancel && (
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => setCancelModalOrder(order)}
                      leftIcon={<Ban className="w-3.5 h-3.5 text-rose-500" />}
                      className="text-slate-500 hover:text-rose-600 hover:bg-rose-50"
                    >
                      Cancel Order
                    </Button>
                  )}
                </div>
              </Card>
            );
          })}
        </div>
      )}

      {/* Cancellation Modal Dialog */}
      <Modal
        isOpen={!!cancelModalOrder}
        onClose={() => setCancelModalOrder(null)}
        title="Cancel Order"
        description={`Cancel order ${cancelModalOrder?.order_number}. Atomically returns inventory back to dark store.`}
      >
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-1.5">
              Reason for Cancellation
            </label>
            <textarea
              rows={3}
              value={cancelReason}
              onChange={(e) => setCancelReason(e.target.value)}
              placeholder="e.g. Changed delivery location, ordered by mistake..."
              className="w-full p-3 bg-slate-50 text-sm text-slate-900 rounded-xl border border-slate-200 focus:bg-white focus:outline-none focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 resize-none"
            />
          </div>

          <div className="flex items-center justify-end gap-3 pt-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setCancelModalOrder(null)}
              disabled={isCancelling}
            >
              Keep Order
            </Button>
            <Button
              variant="danger"
              size="sm"
              onClick={handleCancelOrder}
              isLoading={isCancelling}
            >
              Confirm Cancellation
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
};
