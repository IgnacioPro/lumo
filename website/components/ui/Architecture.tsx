'use client';

import { motion } from 'framer-motion';
import { Terminal, Server, BrainCircuit, Cloud, Layers, Activity, Network } from 'lucide-react';

export default function Architecture() {
  return (
    <div className="relative w-full max-w-4xl mx-auto h-[550px] bg-[#0D1117] rounded-3xl border border-white/10 overflow-hidden shadow-2xl">
      {/* Background Grid */}
      <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:24px_24px]" />
      
      {/* Diagram Container */}
      <div className="relative z-10 h-full w-full">
        
        {/* 1. CENTRAL HUB: The CLI (Orchestrator) */}
        <div className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 z-20 flex flex-col items-center gap-4">
           <div className="relative group">
             <motion.div 
                animate={{ 
                  boxShadow: ["0 0 0 0px rgba(0, 255, 136, 0.1)", "0 0 0 20px rgba(0, 255, 136, 0)"],
                  borderColor: ["#333", "#00FF88", "#333"]
                }}
                transition={{ repeat: Infinity, duration: 3 }}
                className="w-32 h-32 bg-[#1A1F36] rounded-2xl border-2 border-gray-700 flex flex-col items-center justify-center relative z-10 p-2"
             >
                <Terminal className="w-8 h-8 text-electric-green mb-1" />
                <div className="text-[9px] text-gray-400 font-mono text-center leading-tight">
                    <div className="bg-white/5 px-1 rounded mb-0.5">SSH Client</div>
                    <div className="bg-white/5 px-1 rounded mb-0.5">Diagnostics</div>
                    <div className="bg-white/5 px-1 rounded">AI Adapter</div>
                </div>
             </motion.div>
             <div className="absolute -top-3 -right-3 bg-electric-green text-black text-[10px] font-bold px-3 py-1 rounded-full shadow-lg z-20">YOU</div>
             <div className="absolute -bottom-8 left-1/2 -translate-x-1/2 text-white font-bold tracking-wide whitespace-nowrap">Lumo CLI</div>
           </div>
        </div>

        {/* 2. AI PROVIDER (Top Center) */}
        <div className="absolute left-1/2 top-[8%] -translate-x-1/2 z-20 flex flex-col items-center gap-2">
           <div className="w-36 h-24 bg-[#1A1F36] rounded-2xl border border-lumo-blue/50 flex flex-col items-center justify-center shadow-[0_0_30px_rgba(0,102,255,0.15)] p-2">
              <BrainCircuit className="w-8 h-8 text-lumo-blue mb-2" />
              <div className="flex gap-1">
                  <div className="w-1.5 h-1.5 bg-lumo-blue rounded-full animate-bounce" />
                  <div className="w-1.5 h-1.5 bg-lumo-blue rounded-full animate-bounce delay-75" />
                  <div className="w-1.5 h-1.5 bg-lumo-blue rounded-full animate-bounce delay-150" />
              </div>
              <span className="text-lumo-blue text-[9px] font-mono mt-1">Context Window</span>
           </div>
           <span className="text-lumo-blue font-mono text-xs font-bold tracking-widest uppercase">AI Analysis</span>
        </div>

        {/* 3. INFRASTRUCTURE (Bottom Left: Linux) */}
        <div className="absolute left-[10%] bottom-[10%] z-20 flex flex-col items-center gap-2">
            <div className="w-40 h-28 bg-[#1A1F36] rounded-xl border border-white/20 p-3 flex flex-col justify-between group hover:border-white/50 transition-colors">
                <div className="flex items-center gap-2 text-white text-xs font-bold">
                    <Server className="w-4 h-4" />
                    Linux / SSH
                </div>
                <div className="space-y-1">
                    <div className="flex items-center gap-2 text-[10px] text-gray-400 font-mono bg-black/30 px-2 py-1 rounded">
                        <Activity className="w-3 h-3 text-green-400" /> systemd
                    </div>
                    <div className="flex items-center gap-2 text-[10px] text-gray-400 font-mono bg-black/30 px-2 py-1 rounded">
                        <Layers className="w-3 h-3 text-yellow-400" /> procfs
                    </div>
                     <div className="flex items-center gap-2 text-[10px] text-gray-400 font-mono bg-black/30 px-2 py-1 rounded">
                        <Network className="w-3 h-3 text-blue-400" /> netstat
                    </div>
                </div>
            </div>
        </div>

        {/* 4. INFRASTRUCTURE (Bottom Right: K8s) */}
        <div className="absolute right-[10%] bottom-[10%] z-20 flex flex-col items-center gap-2">
            <div className="w-40 h-28 bg-[#1A1F36] rounded-xl border border-white/20 p-3 flex flex-col justify-between group hover:border-white/50 transition-colors">
                <div className="flex items-center gap-2 text-white text-xs font-bold">
                    <Cloud className="w-4 h-4" />
                    Kubernetes
                </div>
                <div className="space-y-1">
                    <div className="flex items-center gap-2 text-[10px] text-gray-400 font-mono bg-black/30 px-2 py-1 rounded">
                        <Layers className="w-3 h-3 text-blue-400" /> API Server
                    </div>
                    <div className="flex items-center gap-2 text-[10px] text-gray-400 font-mono bg-black/30 px-2 py-1 rounded">
                        <Activity className="w-3 h-3 text-green-400" /> Pods
                    </div>
                     <div className="flex items-center gap-2 text-[10px] text-gray-400 font-mono bg-black/30 px-2 py-1 rounded">
                        <Network className="w-3 h-3 text-yellow-400" /> Services
                    </div>
                </div>
            </div>
        </div>

        {/* SVG PATHS & ANIMATIONS */}
        <svg className="absolute inset-0 w-full h-full pointer-events-none overflow-visible">
            <defs>
                <filter id="glow-blue" x="-20%" y="-20%" width="140%" height="140%">
                  <feGaussianBlur stdDeviation="2" result="blur" />
                  <feComposite in="SourceGraphic" in2="blur" operator="over" />
                </filter>
            </defs>

            {/* --- Connection: CLI <-> AI --- */}
            {/* Path */}
            <path 
                d="M 450 200 L 450 120" 
                fill="none" 
                stroke="rgba(0, 102, 255, 0.2)" 
                strokeWidth="2" 
                strokeDasharray="4,4"
            />
            {/* Particle Up (Request) */}
            <circle r="3" fill="#0066FF" filter="url(#glow-blue)">
                <animateMotion 
                    dur="1.5s" 
                    repeatCount="indefinite"
                    path="M 450 200 L 450 120"
                    keyPoints="0;1"
                    keyTimes="0;1"
                />
            </circle>
            {/* Particle Down (Response) */}
            <circle r="3" fill="#BD00FF" filter="url(#glow-blue)">
                <animateMotion 
                    dur="1.5s" 
                    begin="0.75s"
                    repeatCount="indefinite"
                    path="M 450 120 L 450 200"
                    keyPoints="0;1"
                    keyTimes="0;1"
                />
            </circle>


            {/* --- Connection: CLI <-> Linux (Left) --- */}
            <path 
                d="M 400 310 L 200 420" 
                fill="none" 
                stroke="rgba(255, 255, 255, 0.1)" 
                strokeWidth="2" 
            />
            {/* Particle Out (Probe) */}
            <circle r="2" fill="#ffffff">
                <animateMotion 
                    dur="2s" 
                    repeatCount="indefinite"
                    path="M 400 310 L 200 420"
                />
            </circle>
            {/* Particle In (Telemetry) */}
            <circle r="3" fill="#00FF88" filter="url(#glow-blue)">
                <animateMotion 
                    dur="2s" 
                    begin="1s"
                    repeatCount="indefinite"
                    path="M 200 420 L 400 310"
                />
            </circle>


            {/* --- Connection: CLI <-> Kubernetes (Right) --- */}
             <path 
                d="M 500 310 L 700 420" 
                fill="none" 
                stroke="rgba(255, 255, 255, 0.1)" 
                strokeWidth="2" 
            />
             {/* Particle Out (API Call) */}
             <circle r="2" fill="#ffffff">
                <animateMotion 
                    dur="2s" 
                    repeatCount="indefinite"
                    path="M 500 310 L 700 420"
                />
            </circle>
            {/* Particle In (Cluster State) */}
            <circle r="3" fill="#00FF88" filter="url(#glow-blue)">
                <animateMotion 
                    dur="2s" 
                    begin="1s"
                    repeatCount="indefinite"
                    path="M 700 420 L 500 310"
                />
            </circle>
        </svg>
        
        {/* Legend */}
        <div className="absolute bottom-6 left-1/2 -translate-x-1/2 flex flex-wrap justify-center gap-6 bg-black/40 backdrop-blur-md px-6 py-3 rounded-full border border-white/10">
            <div className="flex items-center gap-2 text-[10px] text-gray-400 font-mono uppercase tracking-wider">
                <div className="w-2 h-2 rounded-full bg-white" />
                <span>Probe</span>
            </div>
            <div className="flex items-center gap-2 text-[10px] text-gray-400 font-mono uppercase tracking-wider">
                <div className="w-2 h-2 rounded-full bg-electric-green shadow-[0_0_10px_#00FF88]" />
                <span>Telemetry</span>
            </div>
            <div className="flex items-center gap-2 text-[10px] text-gray-400 font-mono uppercase tracking-wider">
                <div className="w-2 h-2 rounded-full bg-lumo-blue shadow-[0_0_10px_#0066FF]" />
                <span>TOON Data</span>
            </div>
             <div className="flex items-center gap-2 text-[10px] text-gray-400 font-mono uppercase tracking-wider">
                <div className="w-2 h-2 rounded-full bg-electric-purple shadow-[0_0_10px_#BD00FF]" />
                <span>Fixes</span>
            </div>
        </div>

      </div>
    </div>
  );
}
