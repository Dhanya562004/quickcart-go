import React from 'react';
import { Link } from 'react-router-dom';
import { Zap, ArrowLeft, ShoppingBag } from 'lucide-react';
import { Button } from '../components/ui/Button';

export const NotFoundPage: React.FC = () => {
  return (
    <div className="max-w-md mx-auto py-16 text-center space-y-4">
      <div className="w-16 h-16 rounded-2xl bg-brand-50 text-brand-600 flex items-center justify-center mx-auto mb-2 shadow-xs">
        <Zap className="w-8 h-8 fill-brand-600" />
      </div>
      <h1 className="text-4xl font-black text-slate-900 tracking-tight">404</h1>
      <h2 className="text-lg font-bold text-slate-800">Page Not Found</h2>
      <p className="text-xs text-slate-500 max-w-xs mx-auto leading-relaxed">
        The page you are looking for does not exist or has been moved to a new route.
      </p>
      <div className="pt-4 flex items-center justify-center gap-3">
        <Link to="/">
          <Button variant="primary" size="sm" leftIcon={<ArrowLeft className="w-4 h-4" />}>
            Back to Home
          </Button>
        </Link>
        <Link to="/catalog">
          <Button variant="outline" size="sm" leftIcon={<ShoppingBag className="w-4 h-4" />}>
            Browse Store
          </Button>
        </Link>
      </div>
    </div>
  );
};
