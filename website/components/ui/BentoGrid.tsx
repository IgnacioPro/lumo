import { 
  Cpu, 
  ShieldCheck, 
  BrainCircuit, 
  Zap, 
  Layers, 
  Lock, 
  Server,
  Terminal
} from 'lucide-react';

export default function BentoGrid() {
  return (
    <div className="grid grid-cols-1 md:grid-cols-3 gap-6 max-w-6xl mx-auto">
      
      {/* CARD 1: AI Engine (Large) */}
      <div className="md:col-span-2 relative group overflow-hidden rounded-3xl border border-white/10 bg-[#0D1117] p-8 hover:border-lumo-blue/50 transition-all duration-500">
        <div className="absolute top-0 right-0 p-8 opacity-10 group-hover:opacity-20 transition-opacity">
          <BrainCircuit className="w-40 h-40 text-lumo-blue rotate-12" />
        </div>
        <div className="relative z-10">
          <div className="w-12 h-12 rounded-xl bg-lumo-blue/20 flex items-center justify-center mb-6 border border-lumo-blue/30">
            <BrainCircuit className="w-6 h-6 text-lumo-blue" />
          </div>
          <h3 className="text-2xl font-bold text-white mb-3">Model Agnostic Intelligence</h3>
          <p className="text-gray-400 mb-8 max-w-md">
            Don't get locked into one vendor. Lumo works with the best-in-class models or runs completely offline.
          </p>
          
          {/* Visualizing Providers */}
          <div className="flex flex-wrap gap-3">
            {['Claude 3.5', 'GPT-4o', 'Gemini Pro', 'Ollama (Local)'].map((model, i) => (
              <span key={i} className="px-3 py-1 rounded-full text-xs font-mono border border-white/10 bg-white/5 text-gray-300 group-hover:border-lumo-blue/30 group-hover:text-white transition-colors">
                {model}
              </span>
            ))}
          </div>
        </div>
      </div>

      {/* CARD 2: Security (Tall/Side) */}
      <div className="md:col-span-1 relative group overflow-hidden rounded-3xl border border-white/10 bg-[#0D1117] p-8 hover:border-electric-green/50 transition-all duration-500">
         <div className="absolute -bottom-4 -right-4 opacity-10 group-hover:opacity-20 transition-opacity">
          <Lock className="w-32 h-32 text-electric-green -rotate-12" />
        </div>
        <div className="relative z-10 h-full flex flex-col">
           <div className="w-12 h-12 rounded-xl bg-electric-green/20 flex items-center justify-center mb-6 border border-electric-green/30">
            <ShieldCheck className="w-6 h-6 text-electric-green" />
          </div>
          <h3 className="text-2xl font-bold text-white mb-3">Private & Secure</h3>
          <p className="text-gray-400 flex-grow">
            Your infrastructure data is sensitive. Lumo processes diagnostics locally and sanitizes logs before AI analysis.
          </p>
          <div className="mt-6 pt-6 border-t border-white/5">
            <div className="flex items-center gap-2 text-xs text-electric-green font-mono">
              <Lock className="w-3 h-3" />
              <span>SOC 2 Compliant Patterns</span>
            </div>
          </div>
        </div>
      </div>

      {/* CARD 3: Kubernetes */}
      <div className="md:col-span-1 relative group overflow-hidden rounded-3xl border border-white/10 bg-[#0D1117] p-8 hover:border-white/30 transition-all duration-500">
        <div className="absolute top-4 right-4 opacity-20">
            <Layers className="w-8 h-8 text-gray-500" />
        </div>
        <h3 className="text-xl font-bold text-white mb-2">Kubernetes Native</h3>
        <p className="text-sm text-gray-400 mb-4">
          Debug Pods, PVCs, and Services without context-switching hell.
        </p>
        <div className="bg-black/50 rounded p-3 font-mono text-[10px] text-gray-500 border border-white/5">
            $ lumo diag -n prod<br/>
            <span className="text-electric-green">✓ Found 3 crashing pods</span>
        </div>
      </div>

      {/* CARD 4: Auto-Remediation */}
      <div className="md:col-span-1 relative group overflow-hidden rounded-3xl border border-white/10 bg-gradient-to-br from-[#0D1117] to-lumo-blue/10 p-8 hover:border-lumo-blue/50 transition-all duration-500">
        <div className="absolute top-4 right-4 opacity-20">
            <Zap className="w-8 h-8 text-lumo-blue" />
        </div>
        <h3 className="text-xl font-bold text-white mb-2">Auto-Fix</h3>
        <p className="text-sm text-gray-400 mb-4">
          Turn actionable insights into executed commands safely.
        </p>
        <div className="flex items-center gap-2 mt-auto">
            <span className="w-2 h-2 rounded-full bg-electric-green animate-pulse"></span>
            <span className="text-xs font-mono text-lumo-blue">Human-in-the-loop</span>
        </div>
      </div>

       {/* CARD 5: Performance */}
       <div className="md:col-span-1 relative group overflow-hidden rounded-3xl border border-white/10 bg-[#0D1117] p-8 hover:border-white/30 transition-all duration-500">
        <div className="absolute top-4 right-4 opacity-20">
            <Cpu className="w-8 h-8 text-gray-500" />
        </div>
        <h3 className="text-xl font-bold text-white mb-2">Go Performance</h3>
        <p className="text-sm text-gray-400 mb-4">
          Single binary, zero dependencies. Runs on Linux, macOS, Windows.
        </p>
         <div className="flex gap-2 mt-auto">
            {['Linux', 'Darwin', 'Win32'].map(os => (
                <div key={os} className="px-2 py-1 bg-white/5 rounded text-[10px] text-gray-400 border border-white/5">
                    {os}
                </div>
            ))}
        </div>
      </div>

    </div>
  );
}