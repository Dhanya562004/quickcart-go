import React from 'react';
import { Link, useNavigate } from 'react-router-dom';
import {
  User,
  Mail,
  Phone,
  Shield,
  Clock,
  Package,
  LogOut,
  ExternalLink,
} from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';

export const AccountPage: React.FC = () => {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const handleLogout = async () => {
    await logout();
    navigate('/');
  };

  if (!user) return null;

  return (
    <div className="max-w-3xl mx-auto py-6 sm:py-10 space-y-8">
      {/* Account Overview Header */}
      <div className="bg-white rounded-3xl p-6 sm:p-8 border border-slate-100 shadow-card flex flex-col sm:flex-row items-start sm:items-center justify-between gap-6">
        <div className="flex items-center gap-4">
          <div className="w-16 h-16 rounded-2xl bg-gradient-to-tr from-brand-600 to-emerald-400 text-white flex items-center justify-center font-bold text-2xl shadow-md shadow-brand-500/20">
            {user.full_name?.charAt(0).toUpperCase()}
          </div>
          <div>
            <div className="flex items-center gap-2 mb-1">
              <h1 className="text-xl sm:text-2xl font-black text-slate-900 tracking-tight">
                {user.full_name}
              </h1>
              <Badge variant={user.role === 'admin' ? 'purple' : 'brand'} size="sm">
                {user.role}
              </Badge>
            </div>
            <p className="text-xs text-slate-500">
              Account ID: <span className="font-mono text-slate-600">{user.id}</span>
            </p>
          </div>
        </div>

        <Button
          variant="outline"
          size="sm"
          onClick={handleLogout}
          leftIcon={<LogOut className="w-4 h-4 text-rose-600" />}
          className="hover:border-rose-200 hover:bg-rose-50 hover:text-rose-700"
        >
          Sign Out
        </Button>
      </div>

      {/* Account Details & Quick Links Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Contact Information */}
        <Card className="p-6">
          <h2 className="text-sm font-bold text-slate-900 uppercase tracking-wider mb-4 flex items-center gap-2">
            <User className="w-4 h-4 text-brand-600" />
            Contact & Profile Details
          </h2>
          <div className="space-y-4 text-xs">
            <div className="flex items-start gap-3">
              <Mail className="w-4 h-4 text-slate-400 mt-0.5" />
              <div>
                <span className="text-slate-400 block font-medium">Email Address</span>
                <span className="font-semibold text-slate-800 text-sm">{user.email}</span>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <Phone className="w-4 h-4 text-slate-400 mt-0.5" />
              <div>
                <span className="text-slate-400 block font-medium">Phone Number</span>
                <span className="font-semibold text-slate-800 text-sm">{user.phone}</span>
              </div>
            </div>

            <div className="flex items-start gap-3">
              <Clock className="w-4 h-4 text-slate-400 mt-0.5" />
              <div>
                <span className="text-slate-400 block font-medium">Member Since</span>
                <span className="font-semibold text-slate-800">
                  {user.created_at ? new Date(user.created_at).toLocaleDateString('en-IN', {
                    year: 'numeric',
                    month: 'long',
                    day: 'numeric',
                  }) : 'Active Member'}
                </span>
              </div>
            </div>
          </div>
        </Card>

        {/* Quick Actions */}
        <Card className="p-6 flex flex-col justify-between">
          <div>
            <h2 className="text-sm font-bold text-slate-900 uppercase tracking-wider mb-4 flex items-center gap-2">
              <Package className="w-4 h-4 text-brand-600" />
              Orders & Activity
            </h2>
            <p className="text-xs text-slate-500 mb-6 leading-relaxed">
              Track active 10-minute deliveries in real-time, view past orders, or review payment receipts.
            </p>
          </div>

          <div className="space-y-3">
            <Link
              to="/orders"
              className="flex items-center justify-between p-3.5 rounded-xl bg-brand-50/70 hover:bg-brand-100/70 text-brand-800 text-xs font-bold border border-brand-200/60 transition"
            >
              <div className="flex items-center gap-2">
                <Package className="w-4 h-4 text-brand-600" />
                <span>View Order History & Live Tracking</span>
              </div>
              <ExternalLink className="w-3.5 h-3.5" />
            </Link>

            <Link
              to="/catalog"
              className="flex items-center justify-between p-3.5 rounded-xl bg-slate-50 hover:bg-slate-100 text-slate-700 text-xs font-bold border border-slate-200 transition"
            >
              <span>Continue Shopping Catalog</span>
              <ExternalLink className="w-3.5 h-3.5 text-slate-400" />
            </Link>
          </div>
        </Card>
      </div>

      {/* Security Architecture Information */}
      <Card className="p-6 bg-slate-900 text-white border-slate-800">
        <div className="flex items-start gap-3.5">
          <div className="p-2.5 rounded-xl bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
            <Shield className="w-5 h-5" />
          </div>
          <div className="space-y-2 text-xs text-slate-300">
            <h3 className="font-bold text-white text-sm">Security & Token Architecture</h3>
            <p className="leading-relaxed">
              QuickCart implements <strong>atomic single-use refresh token rotation</strong> with short-lived 15-minute JWT access tokens and 7-day refresh tokens. Tokens are stored in memory and synchronized to local storage for persistent browser sessions.
            </p>
            <p className="text-slate-400 leading-relaxed text-[11px]">
              Note: Browser storage is vulnerable to client-side XSS if malicious scripts are injected; sensitive payment cards and passwords are never cached or exposed in URLs or SSE query parameters.
            </p>
          </div>
        </div>
      </Card>
    </div>
  );
};
