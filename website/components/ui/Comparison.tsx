import { XCircle, CheckCircle2, AlertTriangle, Clock } from 'lucide-react';

export default function Comparison() {
  return (
    <div className="grid md:grid-cols-2 gap-8 max-w-6xl mx-auto p-4">
      {/* The Old Way */}
      <div className="rounded-2xl border border-red-900/30 bg-red-950/10 p-8 relative overflow-hidden">
        <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-red-600 to-transparent" />
        <div className="flex items-center gap-3 mb-6">
          <div className="p-2 bg-red-500/10 rounded-lg">
            <XCircle className="w-6 h-6 text-red-500" />
          </div>
          <h3 className="text-2xl font-bold text-red-100">The Manual Way</h3>
        </div>
        
        <ul className="space-y-4 text-red-200/70">
          <li className="flex items-start gap-3">
            <Clock className="w-5 h-5 mt-1 shrink-0" />
            <span>45+ minutes to diagnose basic issues</span>
          </li>
          <li className="flex items-start gap-3">
            <AlertTriangle className="w-5 h-5 mt-1 shrink-0" />
            <span>Grepping through thousands of log lines</span>
          </li>
          <li className="flex items-start gap-3">
            <XCircle className="w-5 h-5 mt-1 shrink-0" />
            <span>Context switching between Grafana, AWS Console, and Terminal</span>
          </li>
        </ul>
      </div>

      {/* The Lumo Way */}
      <div className="rounded-2xl border border-electric-green/30 bg-electric-green/5 p-8 relative overflow-hidden">
        <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-electric-green to-transparent" />
        <div className="flex items-center gap-3 mb-6">
          <div className="p-2 bg-electric-green/10 rounded-lg">
            <CheckCircle2 className="w-6 h-6 text-electric-green" />
          </div>
          <h3 className="text-2xl font-bold text-white">The Lumo Way</h3>
        </div>
        
        <ul className="space-y-4 text-gray-300">
          <li className="flex items-start gap-3">
            <CheckCircle2 className="w-5 h-5 text-electric-green mt-1 shrink-0" />
            <span className="text-white font-medium">2 minutes from alert to resolution</span>
          </li>
          <li className="flex items-start gap-3">
            <CheckCircle2 className="w-5 h-5 text-electric-green mt-1 shrink-0" />
            <span>AI summarizes logs into actionable English</span>
          </li>
          <li className="flex items-start gap-3">
            <CheckCircle2 className="w-5 h-5 text-electric-green mt-1 shrink-0" />
            <span>One CLI tool for Diagnostics + Analysis + Fixes</span>
          </li>
        </ul>
      </div>
    </div>
  );
}
