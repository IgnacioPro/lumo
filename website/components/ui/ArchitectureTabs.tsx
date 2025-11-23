'use client';

import { useState } from 'react';
import { motion } from 'framer-motion';
import { Terminal, LayoutGrid } from 'lucide-react';
import Architecture from './Architecture';
import ArchitectureK8s from './ArchitectureK8s';

export default function ArchitectureTabs() {
  const [activeTab, setActiveTab] = useState<'cli' | 'k8s'>('cli');

  return (
    <div className="space-y-8">
      {/* Toggle Switch */}
      <div className="flex justify-center">
        <div className="bg-white/5 p-1 rounded-full border border-white/10 flex items-center gap-1">
          <button
            onClick={() => setActiveTab('cli')}
            className={`flex items-center gap-2 px-6 py-2 rounded-full text-sm font-medium transition-all duration-300 ${
              activeTab === 'cli'
                ? 'bg-white text-obsidian shadow-lg scale-105'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            <Terminal className="w-4 h-4" />
            CLI (Standalone)
          </button>
          <button
            onClick={() => setActiveTab('k8s')}
            className={`flex items-center gap-2 px-6 py-2 rounded-full text-sm font-medium transition-all duration-300 ${
              activeTab === 'k8s'
                ? 'bg-white text-obsidian shadow-lg scale-105'
                : 'text-gray-400 hover:text-white'
            }`}
          >
            <LayoutGrid className="w-4 h-4" />
            Kubernetes (Agent)
          </button>
        </div>
      </div>

      {/* Diagrams with transitions */}
      <div className="relative min-h-[500px]">
         <div className={activeTab === 'cli' ? 'block' : 'hidden'}>
            <motion.div
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ duration: 0.4 }}
            >
                <Architecture />
            </motion.div>
         </div>
         
         <div className={activeTab === 'k8s' ? 'block' : 'hidden'}>
            <motion.div
                initial={{ opacity: 0, scale: 0.95 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ duration: 0.4 }}
            >
                <ArchitectureK8s />
            </motion.div>
         </div>
      </div>
      
      {/* Context Description */}
      <div className="text-center max-w-2xl mx-auto">
        <motion.p 
            key={activeTab}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            className="text-gray-400 text-sm"
        >
            {activeTab === 'cli' 
                ? "In CLI mode, Lumo runs on your local machine and acts as a bridge. It connects to remote servers via SSH, gathers telemetry, and sends sanitized reports to the AI for analysis. Zero installation required on the target servers."
                : "In Agent mode, Lumo runs directly inside your Kubernetes cluster as a DaemonSet. It continuously monitors node health and resource usage, reporting telemetry to the Lumo API for background analysis and alerting."
            }
        </motion.p>
      </div>

    </div>
  );
}
