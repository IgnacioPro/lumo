'use client';

import { motion } from 'framer-motion';
import { Server, BrainCircuit, Cloud, LayoutGrid, Activity, Database, HardDrive, Box, Globe, Lock } from 'lucide-react';

export default function ArchitectureK8s() {
  return (
    <div className="relative w-full max-w-4xl mx-auto h-[550px] bg-[#0D1117] rounded-3xl border border-white/10 overflow-hidden shadow-2xl">
      {/* Background Grid */}
      <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:24px_24px]" />
      
      {/* Diagram Container */}
      <div className="relative z-10 h-full w-full">
        
        {/* 1. KUBERNETES CLUSTER (Container) */}
        <div className="absolute left-[5%] top-[10%] bottom-[10%] right-[35%] rounded-3xl border-2 border-dashed border-white/10 bg-white/5 p-6 flex flex-col justify-between">
           <div className="absolute -top-4 left-6 bg-[#0D1117] px-3 py-1 text-xs font-mono text-blue-400 border border-blue-400/30 rounded-full flex items-center gap-2">
             <LayoutGrid className="w-3 h-3" />
             Kubernetes Cluster
           </div>

           {/* Node 1 */}
           <div className="relative h-[48%] w-full bg-[#1A1F36] rounded-xl border border-white/10 p-3 flex flex-col gap-2 group">
              {/* Header */}
              <div className="flex items-center justify-between border-b border-white/5 pb-1.5">
                 <div className="flex items-center gap-2 text-gray-400 text-xs font-mono">
                    <Server className="w-3.5 h-3.5" />
                    Node 01 (Worker)
                 </div>
                 <div className="flex gap-1">
                    <div className="w-1.5 h-1.5 bg-green-500 rounded-full animate-pulse" />
                    <div className="w-1.5 h-1.5 bg-green-500 rounded-full delay-75" />
                 </div>
              </div>
              
              {/* App Pods Row (Top) */}
              <div className="grid grid-cols-3 gap-2 h-8">
                  <div className="rounded border border-white/10 bg-white/5 flex items-center justify-center gap-1.5 group/pod hover:bg-white/10 transition-colors">
                      <Globe className="w-3 h-3 text-blue-300" />
                      <span className="text-[8px] text-gray-400 font-mono hidden sm:inline">web</span>
                  </div>
                  <div className="rounded border border-white/10 bg-white/5 flex items-center justify-center gap-1.5 group/pod hover:bg-white/10 transition-colors">
                      <Box className="w-3 h-3 text-orange-300" />
                      <span className="text-[8px] text-gray-400 font-mono hidden sm:inline">api</span>
                  </div>
                  <div className="rounded border border-white/10 bg-white/5 flex items-center justify-center gap-1.5 group/pod hover:bg-white/10 transition-colors">
                      <Lock className="w-3 h-3 text-purple-300" />
                      <span className="text-[8px] text-gray-400 font-mono hidden sm:inline">auth</span>
                  </div>
              </div>

              {/* Lumo Agent Row (Bottom - Full Width) */}
              <div className="flex-1 bg-black/40 rounded border border-electric-green/50 p-2 flex flex-col items-center justify-center gap-1 shadow-[0_0_10px_rgba(0,255,136,0.05)] relative group/agent">
                  <div className="flex items-center gap-2">
                      <Activity className="w-4 h-4 text-electric-green" />
                      <span className="text-[10px] text-electric-green font-bold uppercase tracking-wider">Lumo Agent</span>
                  </div>
                  <div className="flex gap-1">
                      <div className="w-8 h-1 bg-electric-green/30 rounded-full overflow-hidden">
                          <div className="w-full h-full bg-electric-green animate-progress-indeterminate origin-left" />
                      </div>
                  </div>
                  {/* Telemetry Exit Point */}
                  <div className="absolute -right-1.5 top-1/2 -translate-y-1/2 w-1.5 h-1.5 bg-electric-green rounded-full shadow-[0_0_5px_#00FF88]" />
              </div>
           </div>

           {/* Node 2 */}
            <div className="relative h-[48%] w-full bg-[#1A1F36] rounded-xl border border-white/10 p-3 flex flex-col gap-2 group">
              {/* Header */}
              <div className="flex items-center justify-between border-b border-white/5 pb-1.5">
                 <div className="flex items-center gap-2 text-gray-400 text-xs font-mono">
                    <Server className="w-3.5 h-3.5" />
                    Node 02 (Worker)
                 </div>
                 <div className="flex gap-1">
                    <div className="w-1.5 h-1.5 bg-green-500 rounded-full" />
                    <div className="w-1.5 h-1.5 bg-green-500 rounded-full" />
                 </div>
              </div>
              
               {/* App Pods Row (Top) */}
              <div className="grid grid-cols-3 gap-2 h-8">
                  <div className="rounded border border-white/10 bg-white/5 flex items-center justify-center gap-1.5 group/pod hover:bg-white/10 transition-colors">
                      <Database className="w-3 h-3 text-yellow-300" />
                      <span className="text-[8px] text-gray-400 font-mono hidden sm:inline">db-01</span>
                  </div>
                  <div className="rounded border border-white/10 bg-white/5 flex items-center justify-center gap-1.5 group/pod hover:bg-white/10 transition-colors">
                      <HardDrive className="w-3 h-3 text-red-300" />
                      <span className="text-[8px] text-gray-400 font-mono hidden sm:inline">redis</span>
                  </div>
                  <div className="rounded border border-white/10 bg-white/5 flex items-center justify-center gap-1.5 opacity-30 border-dashed">
                      <span className="text-[8px] text-gray-600 font-mono">empty</span>
                  </div>
              </div>

              {/* Lumo Agent Row (Bottom - Full Width) */}
              <div className="flex-1 bg-black/40 rounded border border-electric-green/50 p-2 flex flex-col items-center justify-center gap-1 shadow-[0_0_10px_rgba(0,255,136,0.05)] relative group/agent">
                   <div className="flex items-center gap-2">
                      <Activity className="w-4 h-4 text-electric-green" />
                      <span className="text-[10px] text-electric-green font-bold uppercase tracking-wider">Lumo Agent</span>
                  </div>
                  <div className="flex gap-1">
                      <div className="w-8 h-1 bg-electric-green/30 rounded-full overflow-hidden">
                          <div className="w-full h-full bg-electric-green animate-progress-indeterminate origin-left" />
                      </div>
                  </div>
                  {/* Telemetry Exit Point */}
                  <div className="absolute -right-1.5 top-1/2 -translate-y-1/2 w-1.5 h-1.5 bg-electric-green rounded-full shadow-[0_0_5px_#00FF88]" />
              </div>
           </div>
        </div>

        {/* 2. LUMO API (Cloud Service) */}
        <div className="absolute right-[5%] top-[20%] z-20 flex flex-col items-center gap-4 w-64">
            <div className="relative group w-full">
                 <motion.div 
                    animate={{ 
                      boxShadow: ["0 0 0 0px rgba(0, 102, 255, 0.1)", "0 0 0 20px rgba(0, 102, 255, 0)"],
                    }}
                    transition={{ repeat: Infinity, duration: 3 }}
                    className="w-full bg-[#1A1F36] rounded-2xl border-2 border-lumo-blue flex flex-col items-center justify-center relative z-10 p-4 gap-3"
                 >
                    <div className="flex items-center gap-2">
                        <Cloud className="w-6 h-6 text-lumo-blue" />
                        <span className="text-white font-bold">Lumo API</span>
                    </div>
                    
                    {/* Internal Components */}
                    <div className="grid grid-cols-2 gap-2 w-full">
                        <div className="bg-black/30 rounded border border-white/10 p-2 flex flex-col items-center">
                            <Database className="w-4 h-4 text-yellow-400 mb-1" />
                            <span className="text-[8px] text-gray-400">PostgreSQL</span>
                        </div>
                        <div className="bg-black/30 rounded border border-white/10 p-2 flex flex-col items-center">
                            <HardDrive className="w-4 h-4 text-red-400 mb-1" />
                            <span className="text-[8px] text-gray-400">Redis</span>
                        </div>
                    </div>
                 </motion.div>
            </div>
        </div>

        {/* 3. AI BRAIN (Connected to API) */}
        <div className="absolute right-[5%] bottom-[15%] z-20 w-64 flex flex-col items-center">
            <div className="w-full bg-[#1A1F36] rounded-xl border border-electric-purple/50 p-4 flex items-center justify-center gap-3">
                 <BrainCircuit className="w-8 h-8 text-electric-purple" />
                 <div className="flex flex-col">
                     <span className="text-white text-xs font-bold">AI Analysis</span>
                     <span className="text-[9px] text-gray-400 font-mono">LLM Provider</span>
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

            {/* Connection: Node 1 -> API */}
            {/* Adjusted origin Y because Agent is now at the bottom of the node card */}
            <path 
                d="M 280 210 C 400 210, 500 160, 620 160" 
                fill="none" 
                stroke="rgba(0, 255, 136, 0.1)" 
                strokeWidth="2" 
            />
            <circle r="3" fill="#00FF88" filter="url(#glow-blue)">
                <animateMotion 
                    dur="3s" 
                    repeatCount="indefinite"
                    path="M 280 210 C 400 210, 500 160, 620 160"
                />
            </circle>

            {/* Connection: Node 2 -> API */}
            {/* Adjusted origin Y because Agent is now at the bottom of the node card */}
            <path 
                d="M 280 420 C 400 420, 500 160, 620 160" 
                fill="none" 
                stroke="rgba(0, 255, 136, 0.1)" 
                strokeWidth="2" 
            />
             <circle r="3" fill="#00FF88" filter="url(#glow-blue)">
                <animateMotion 
                    dur="3s" 
                    begin="1.5s"
                    repeatCount="indefinite"
                    path="M 280 420 C 400 420, 500 160, 620 160"
                />
            </circle>

            {/* Connection: API <-> AI */}
            <path 
                d="M 840 280 L 840 410" 
                fill="none" 
                stroke="rgba(189, 0, 255, 0.2)" 
                strokeWidth="2" 
                strokeDasharray="4,4"
            />
             <circle r="2" fill="#BD00FF">
                <animateMotion 
                    dur="2s" 
                    repeatCount="indefinite"
                    path="M 840 280 L 840 410"
                    keyPoints="0;1"
                    keyTimes="0;1"
                />
            </circle>
             <circle r="2" fill="#BD00FF">
                <animateMotion 
                    dur="2s" 
                    begin="1s"
                    repeatCount="indefinite"
                    path="M 840 410 L 840 280"
                    keyPoints="0;1"
                    keyTimes="0;1"
                />
            </circle>

        </svg>

        {/* Legend */}
        <div className="absolute bottom-6 left-[20%] bg-black/40 backdrop-blur-md px-4 py-2 rounded-full border border-white/10 flex items-center gap-4 z-30">
             <div className="flex items-center gap-2 text-[10px] text-gray-400 font-mono uppercase tracking-wider">
                <div className="w-2 h-2 rounded-full bg-electric-green shadow-[0_0_10px_#00FF88]" />
                <span>DaemonSet Telemetry</span>
            </div>
             <div className="flex items-center gap-2 text-[10px] text-gray-400 font-mono uppercase tracking-wider">
                <div className="w-2 h-2 rounded-full bg-lumo-blue shadow-[0_0_10px_#0066FF]" />
                <span>API Aggregation</span>
            </div>
        </div>

      </div>
    </div>
  );
}
