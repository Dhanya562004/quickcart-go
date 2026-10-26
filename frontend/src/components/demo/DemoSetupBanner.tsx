import React, { useState } from 'react';
import { Sparkles, Terminal, Copy, Check, ChevronDown, ChevronUp, Database } from 'lucide-react';
import { useStore } from '../../context/StoreContext';
import { Button } from '../ui/Button';
import { Badge } from '../ui/Badge';

export const DemoSetupBanner: React.FC = () => {
  const { stores, isDemoMode, setIsDemoMode } = useStore();
  const [isExpanded, setIsExpanded] = useState(false);
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);

  const isProductionEmpty = stores.length === 0;

  const curlSnippets = [
    {
      title: '1. Register Admin User',
      command: `curl -X POST https://quickcart-go.onrender.com/api/v1/auth/register \\
  -H "Content-Type: application/json" \\
  -d '{"email":"admin@quickcart.dev","password":"AdminSecurePass!2026","full_name":"Admin Lead","phone":"+919876543210"}'`,
    },
    {
      title: '2. Create Category (Admin)',
      command: `curl -X POST https://quickcart-go.onrender.com/api/v1/admin/categories \\
  -H "Authorization: Bearer <ADMIN_JWT>" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"Dairy & Breakfast","slug":"dairy-breakfast","description":"Fresh milk, curd and breakfast essentials"}'`,
    },
    {
      title: '3. Create Dark Store & Products',
      command: `curl -X POST https://quickcart-go.onrender.com/api/v1/admin/stores \\
  -H "Authorization: Bearer <ADMIN_JWT>" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"Indiranagar Dark Store #04","address":"100ft Rd, Indiranagar, Bengaluru","latitude":12.9716,"longitude":77.5946}'`,
    },
  ];

  const handleCopy = (text: string, index: number) => {
    navigator.clipboard.writeText(text);
    setCopiedIndex(index);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  if (!isProductionEmpty && !isDemoMode) {
    return null;
  }

  return (
    <div className="bg-gradient-to-r from-emerald-950 via-slate-900 to-emerald-950 text-white rounded-2xl p-5 mb-8 border border-emerald-800/40 shadow-xl">
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div className="flex items-start gap-3.5">
          <div className="p-2.5 rounded-xl bg-emerald-500/20 border border-emerald-500/30 text-emerald-400 mt-0.5">
            <Database className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center gap-2 flex-wrap mb-1">
              <span className="font-semibold text-white tracking-tight">Production Backend Environment</span>
              {isDemoMode ? (
                <Badge variant="warning" size="sm">Preview Mode Active</Badge>
              ) : (
                <Badge variant="neutral" size="sm">Live Connected (Empty Catalog)</Badge>
              )}
            </div>
            <p className="text-sm text-slate-300 max-w-2xl leading-relaxed">
              {isDemoMode
                ? 'Displaying safe client-side preview catalog so you can test and explore the entire consumer shopping flow. Production database remains untouched.'
                : 'The production PostgreSQL database has no dark stores or categories registered yet.'}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2.5 flex-wrap self-end md:self-center">
          <Button
            size="sm"
            variant={isDemoMode ? 'outline' : 'primary'}
            onClick={() => setIsDemoMode(!isDemoMode)}
            leftIcon={<Sparkles className="w-4 h-4" />}
            className={isDemoMode ? 'bg-white/10 text-white border-white/20 hover:bg-white/20' : ''}
          >
            {isDemoMode ? 'Switch to Real Backend Only' : 'Enable Demo Preview Mode'}
          </Button>
          <Button
            size="sm"
            variant="ghost"
            onClick={() => setIsExpanded(!isExpanded)}
            leftIcon={<Terminal className="w-4 h-4" />}
            rightIcon={isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
            className="text-slate-300 hover:text-white hover:bg-white/10"
          >
            {isExpanded ? 'Hide API Seeding' : 'Admin Seeding Guide'}
          </Button>
        </div>
      </div>

      {isExpanded && (
        <div className="mt-5 pt-5 border-t border-slate-800 animate-fade-in">
          <p className="text-xs text-slate-400 mb-3">
            Authorized administrators can seed real data directly into the Render PostgreSQL production database using these standard REST endpoints:
          </p>
          <div className="grid gap-3">
            {curlSnippets.map((snippet, idx) => (
              <div key={idx} className="bg-slate-950/80 rounded-xl p-3 border border-slate-800">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-xs font-mono text-emerald-400 font-semibold">{snippet.title}</span>
                  <button
                    onClick={() => handleCopy(snippet.command, idx)}
                    className="flex items-center gap-1 text-xs text-slate-400 hover:text-white py-1 px-2 rounded hover:bg-slate-800 transition"
                  >
                    {copiedIndex === idx ? (
                      <>
                        <Check className="w-3.5 h-3.5 text-emerald-400" />
                        <span className="text-emerald-400">Copied</span>
                      </>
                    ) : (
                      <>
                        <Copy className="w-3.5 h-3.5" />
                        <span>Copy cURL</span>
                      </>
                    )}
                  </button>
                </div>
                <pre className="text-xs font-mono text-slate-300 overflow-x-auto p-2 bg-slate-900 rounded-lg">
                  {snippet.command}
                </pre>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};
