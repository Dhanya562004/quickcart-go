import React from 'react';
import { Category } from '../../api/types';

interface CategoryPillProps {
  category: Category;
  isActive: boolean;
  onClick: () => void;
}

export const CategoryPill: React.FC<CategoryPillProps> = ({
  category,
  isActive,
  onClick,
}) => {
  return (
    <button
      onClick={onClick}
      className={`px-4 py-2 rounded-xl text-xs sm:text-sm font-semibold whitespace-nowrap transition-all duration-200 select-none shadow-2xs ${
        isActive
          ? 'bg-brand-600 text-white shadow-brand-500/20 scale-[1.02]'
          : 'bg-white text-slate-700 hover:bg-slate-50 border border-slate-200/80 hover:border-slate-300'
      }`}
    >
      {category.name}
    </button>
  );
};
