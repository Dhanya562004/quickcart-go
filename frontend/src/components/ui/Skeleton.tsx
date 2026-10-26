import React from 'react';

export const Skeleton: React.FC<{ className?: string }> = ({ className = 'h-4 w-full' }) => {
  return (
    <div
      className={`animate-pulse bg-slate-200/80 rounded-lg ${className}`}
      aria-hidden="true"
    />
  );
};

export const ProductSkeleton: React.FC = () => {
  return (
    <div className="bg-white rounded-2xl p-4 border border-slate-100 shadow-card flex flex-col gap-3">
      <Skeleton className="h-40 w-full rounded-xl" />
      <Skeleton className="h-4 w-2/3" />
      <Skeleton className="h-3 w-1/3" />
      <div className="flex items-center justify-between pt-2 mt-auto">
        <Skeleton className="h-6 w-16" />
        <Skeleton className="h-8 w-20 rounded-lg" />
      </div>
    </div>
  );
};
