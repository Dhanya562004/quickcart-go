import React from 'react';
import { Link } from 'react-router-dom';
import { Zap, Github, BookOpen, FileCode, Shield, Clock } from 'lucide-react';

export const Footer: React.FC = () => {
  return (
    <footer className="bg-slate-900 text-slate-300 border-t border-slate-800 mt-auto">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12 lg:py-16">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-8 lg:gap-12">
          {/* Brand Info */}
          <div className="md:col-span-1 space-y-4">
            <Link to="/" className="flex items-center gap-2.5">
              <div className="w-9 h-9 rounded-xl bg-gradient-to-tr from-brand-500 to-emerald-400 flex items-center justify-center text-white shadow-md">
                <Zap className="w-5 h-5 fill-white" />
              </div>
              <span className="text-xl font-black tracking-tight text-white">
                Quick<span className="text-brand-400">Cart</span>
              </span>
            </Link>
            <p className="text-xs text-slate-400 leading-relaxed">
              Concurrent Quick-Commerce Engine delivering farm-fresh groceries and household essentials within 10 minutes.
            </p>
            <div className="flex items-center gap-2 text-xs text-slate-400">
              <Clock className="w-4 h-4 text-emerald-400" />
              <span>Sub-10-Minute Micro-Fulfillment SLA</span>
            </div>
            <div className="flex items-center gap-2 text-xs text-slate-400">
              <Shield className="w-4 h-4 text-emerald-400" />
              <span>ACID Row-Locked Zero-Overselling</span>
            </div>
          </div>

          {/* Quick Commerce Pages */}
          <div>
            <h4 className="text-xs font-bold text-white uppercase tracking-wider mb-4">
              Explore Store
            </h4>
            <ul className="space-y-2.5 text-xs text-slate-400">
              <li>
                <Link to="/catalog" className="hover:text-emerald-400 transition">
                  Browse Product Catalog
                </Link>
              </li>
              <li>
                <Link to="/orders" className="hover:text-emerald-400 transition">
                  Order History & Tracking
                </Link>
              </li>
              <li>
                <Link to="/account" className="hover:text-emerald-400 transition">
                  My Profile & Settings
                </Link>
              </li>
              <li>
                <Link to="/login" className="hover:text-emerald-400 transition">
                  Customer Sign In
                </Link>
              </li>
            </ul>
          </div>

          {/* Backend Architecture & Engineering */}
          <div>
            <h4 className="text-xs font-bold text-white uppercase tracking-wider mb-4">
              Engine Architecture
            </h4>
            <ul className="space-y-2.5 text-xs text-slate-400">
              <li className="flex items-center gap-2">
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                <span>Go 1.25 Modular Monolith</span>
              </li>
              <li className="flex items-center gap-2">
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                <span>Neon PostgreSQL (pgxpool)</span>
              </li>
              <li className="flex items-center gap-2">
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                <span>Aiven Valkey (Redis-compatible)</span>
              </li>
              <li className="flex items-center gap-2">
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                <span>Server-Sent Events (SSE) Live Hub</span>
              </li>
              <li className="flex items-center gap-2">
                <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                <span>Atomic Refresh Token Rotation</span>
              </li>
            </ul>
          </div>

          {/* Developer Resources & API Docs */}
          <div>
            <h4 className="text-xs font-bold text-white uppercase tracking-wider mb-4">
              Developer APIs
            </h4>
            <div className="space-y-3">
              <a
                href="https://quickcart-go.onrender.com/docs"
                target="_blank"
                rel="noreferrer"
                className="flex items-center gap-2 px-3 py-2 rounded-xl bg-slate-800 hover:bg-slate-700/80 text-xs text-emerald-400 transition border border-slate-700"
              >
                <BookOpen className="w-4 h-4" />
                <span>Interactive Swagger UI</span>
              </a>
              <a
                href="https://quickcart-go.onrender.com/api/openapi.yaml"
                target="_blank"
                rel="noreferrer"
                className="flex items-center gap-2 px-3 py-2 rounded-xl bg-slate-800 hover:bg-slate-700/80 text-xs text-slate-300 transition border border-slate-700"
              >
                <FileCode className="w-4 h-4 text-emerald-400" />
                <span>OpenAPI 3.0 Specification</span>
              </a>
              <a
                href="https://github.com/Dhanya562004/quickcart-go"
                target="_blank"
                rel="noreferrer"
                className="flex items-center gap-2 px-3 py-2 rounded-xl bg-slate-800 hover:bg-slate-700/80 text-xs text-slate-300 transition border border-slate-700"
              >
                <Github className="w-4 h-4 text-slate-200" />
                <span>GitHub Repository</span>
              </a>
            </div>
          </div>
        </div>

        <div className="mt-12 pt-8 border-t border-slate-800 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-slate-500">
          <div>
            © {new Date().getFullYear()} QuickCart — High-Throughput Concurrent Quick-Commerce Engine. All rights reserved.
          </div>
          <div className="flex items-center gap-1 text-slate-400">
            <span>Built with precision in Go & React</span>
          </div>
        </div>
      </div>
    </footer>
  );
};
