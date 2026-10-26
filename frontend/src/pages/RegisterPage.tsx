import React, { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import * as z from 'zod';
import { Lock, Mail, User, Phone, ArrowRight, Zap, AlertCircle } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { getApiErrorMessage } from '../api/client';

const registerSchema = z
  .object({
    full_name: z.string().min(2, 'Full name must be at least 2 characters'),
    email: z.string().email('Please enter a valid email address'),
    phone: z.string().min(10, 'Please enter a valid phone number (at least 10 digits)'),
    password: z.string().min(8, 'Password must be at least 8 characters'),
    confirm_password: z.string().min(8, 'Confirm password must be at least 8 characters'),
  })
  .refine((data) => data.password === data.confirm_password, {
    message: "Passwords don't match",
    path: ['confirm_password'],
  });

type RegisterFormValues = z.infer<typeof registerSchema>;

export const RegisterPage: React.FC = () => {
  const { register: registerAuth } = useAuth();
  const navigate = useNavigate();
  const [apiError, setApiError] = useState<string | null>(null);

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<RegisterFormValues>({
    resolver: zodResolver(registerSchema),
    defaultValues: {
      full_name: '',
      email: '',
      phone: '',
      password: '',
      confirm_password: '',
    },
  });

  const onSubmit = async (values: RegisterFormValues) => {
    setApiError(null);
    try {
      await registerAuth({
        full_name: values.full_name,
        email: values.email,
        phone: values.phone.startsWith('+') ? values.phone : `+91${values.phone.replace(/\D/g, '')}`,
        password: values.password,
      });
      navigate('/login');
    } catch (err: unknown) {
      setApiError(getApiErrorMessage(err));
    }
  };

  return (
    <div className="max-w-md mx-auto py-8 sm:py-12">
      <div className="text-center mb-8">
        <div className="inline-flex items-center justify-center w-12 h-12 rounded-2xl bg-brand-600 text-white shadow-lg shadow-brand-500/25 mb-3">
          <Zap className="w-6 h-6 fill-white" />
        </div>
        <h1 className="text-2xl sm:text-3xl font-black text-slate-900 tracking-tight">
          Create an Account
        </h1>
        <p className="text-sm text-slate-500 mt-1">
          Join QuickCart for lightning-fast 10-minute grocery delivery
        </p>
      </div>

      <Card className="p-6 sm:p-8">
        {apiError && (
          <div className="mb-5 p-3.5 rounded-xl bg-rose-50 border border-rose-200 text-rose-800 text-xs flex items-start gap-2.5 animate-shake">
            <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5 text-rose-600" />
            <div className="leading-relaxed">{apiError}</div>
          </div>
        )}

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-1.5">
              Full Name
            </label>
            <div className="relative">
              <User className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
              <input
                type="text"
                {...register('full_name')}
                placeholder="Deeksha Sharma"
                className="w-full pl-10 pr-4 py-2.5 bg-slate-50 text-sm text-slate-900 rounded-xl border border-slate-200 focus:bg-white focus:outline-none focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 transition"
              />
            </div>
            {errors.full_name && (
              <p className="text-xs text-rose-600 mt-1">{errors.full_name.message}</p>
            )}
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-1.5">
              Email Address
            </label>
            <div className="relative">
              <Mail className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
              <input
                type="email"
                {...register('email')}
                placeholder="deeksha@example.com"
                className="w-full pl-10 pr-4 py-2.5 bg-slate-50 text-sm text-slate-900 rounded-xl border border-slate-200 focus:bg-white focus:outline-none focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 transition"
              />
            </div>
            {errors.email && (
              <p className="text-xs text-rose-600 mt-1">{errors.email.message}</p>
            )}
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-1.5">
              Phone Number
            </label>
            <div className="relative">
              <Phone className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
              <input
                type="tel"
                {...register('phone')}
                placeholder="+919876543210"
                className="w-full pl-10 pr-4 py-2.5 bg-slate-50 text-sm text-slate-900 rounded-xl border border-slate-200 focus:bg-white focus:outline-none focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 transition"
              />
            </div>
            {errors.phone && (
              <p className="text-xs text-rose-600 mt-1">{errors.phone.message}</p>
            )}
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-1.5">
              Password (min 8 characters)
            </label>
            <div className="relative">
              <Lock className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
              <input
                type="password"
                {...register('password')}
                placeholder="••••••••"
                className="w-full pl-10 pr-4 py-2.5 bg-slate-50 text-sm text-slate-900 rounded-xl border border-slate-200 focus:bg-white focus:outline-none focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 transition"
              />
            </div>
            {errors.password && (
              <p className="text-xs text-rose-600 mt-1">{errors.password.message}</p>
            )}
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-700 uppercase tracking-wider mb-1.5">
              Confirm Password
            </label>
            <div className="relative">
              <Lock className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
              <input
                type="password"
                {...register('confirm_password')}
                placeholder="••••••••"
                className="w-full pl-10 pr-4 py-2.5 bg-slate-50 text-sm text-slate-900 rounded-xl border border-slate-200 focus:bg-white focus:outline-none focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 transition"
              />
            </div>
            {errors.confirm_password && (
              <p className="text-xs text-rose-600 mt-1">{errors.confirm_password.message}</p>
            )}
          </div>

          <Button
            type="submit"
            variant="primary"
            size="lg"
            className="w-full mt-2 font-bold"
            isLoading={isSubmitting}
            rightIcon={<ArrowRight className="w-4 h-4" />}
          >
            Create Customer Account
          </Button>
        </form>

        <div className="mt-6 pt-6 border-t border-slate-100 text-center">
          <p className="text-xs text-slate-500">
            Already have an account?{' '}
            <Link to="/login" className="font-bold text-brand-600 hover:text-brand-700">
              Sign In
            </Link>
          </p>
        </div>
      </Card>
    </div>
  );
};
