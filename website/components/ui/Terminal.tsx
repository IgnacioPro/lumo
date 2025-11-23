'use client';

import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';

const lines = [
  { text: "$ lumo diagnose --analyze", type: "command" },
  { text: "🔍 Connecting to production-db-01 (SSH)...", type: "info", delay: 500 },
  { text: "✅ CPU: 15% (Normal)", type: "success", delay: 1200 },
  { text: "❌ Memory: 92% (CRITICAL)", type: "error", delay: 1800 },
  { text: "✅ Disk: 45% used", type: "success", delay: 2000 },
  { text: "", type: "spacer", delay: 2200 },
  { text: "🤖 AI Analysis (Claude 3.5 Sonnet):", type: "ai-header", delay: 3000 },
  { text: "High memory usage detected in 'redis-server' process (12GB).", type: "ai-body", delay: 3500 },
  { text: "This appears to be an eviction policy misconfiguration.", type: "ai-body", delay: 4500 },
  { text: "", type: "spacer", delay: 4800 },
  { text: "💡 Suggested Remediation:", type: "fix-header", delay: 5500 },
  { text: "Run: redis-cli config set maxmemory-policy allkeys-lru", type: "fix-cmd", delay: 6000 },
  { text: "", type: "spacer", delay: 6500 },
  { text: "$ Apply fix? [Y/n] Y", type: "prompt", delay: 7500 },
  { text: "✅ Fix applied successfully.", type: "success", delay: 8500 },
  { text: "📉 Memory usage dropping... Now at 42%.", type: "info", delay: 9500 },
];

export default function Terminal() {
  const [visibleLines, setVisibleLines] = useState<number>(0);

  useEffect(() => {
    let timeouts: NodeJS.Timeout[] = [];
    
    // Reset
    setVisibleLines(0);

    // Schedule lines
    lines.forEach((line, index) => {
      const timeout = setTimeout(() => {
        setVisibleLines(prev => prev + 1);
      }, line.delay || index * 100); // Default staggered delay if none provided
      timeouts.push(timeout);
    });

    // Loop animation
    const loopTimeout = setTimeout(() => {
        setVisibleLines(0);
        // Rerun
        lines.forEach((line, index) => {
            const timeout = setTimeout(() => {
              setVisibleLines(prev => prev + 1);
            }, line.delay || index * 100);
            timeouts.push(timeout);
        });
    }, 14000); // Restart after 14s
    timeouts.push(loopTimeout);

    return () => timeouts.forEach(clearTimeout);
  }, []);

  return (
    <div className="w-full max-w-3xl mx-auto rounded-xl overflow-hidden bg-[#0D1117] border border-gray-800 shadow-2xl font-mono text-sm md:text-base relative group">
       {/* Window Controls */}
      <div className="flex items-center justify-between px-4 py-3 bg-[#161B22] border-b border-gray-800">
        <div className="flex gap-2">
          <div className="w-3 h-3 rounded-full bg-[#FF5F56]" />
          <div className="w-3 h-3 rounded-full bg-[#FFBD2E]" />
          <div className="w-3 h-3 rounded-full bg-[#27C93F]" />
        </div>
        <div className="text-gray-500 text-xs flex items-center gap-1">
          <span className="w-2 h-2 rounded-full bg-electric-green animate-pulse"></span>
          user@prod-server:~
        </div>
      </div>

      {/* Terminal Content */}
      <div className="p-6 min-h-[400px] text-gray-300 space-y-2">
        {lines.map((line, index) => (
          <motion.div
            key={index}
            initial={{ opacity: 0, x: -10 }}
            animate={{ opacity: visibleLines > index ? 1 : 0, x: visibleLines > index ? 0 : -10 }}
            transition={{ duration: 0.2 }}
            className={`${visibleLines > index ? 'block' : 'hidden'}`}
          >
            {renderLine(line)}
          </motion.div>
        ))}
        
        {/* Cursor */}
        <motion.div 
            className="inline-block w-2 h-4 bg-gray-500 ml-1"
            animate={{ opacity: [1, 0] }}
            transition={{ repeat: Infinity, duration: 0.8 }}
        />
      </div>
      
      {/* Reflection Gradient */}
      <div className="absolute inset-0 bg-gradient-to-tr from-white/5 to-transparent pointer-events-none" />
    </div>
  );
}

function renderLine(line: { text: string; type: string }) {
  switch (line.type) {
    case 'command':
      return <span className="text-white font-bold">{line.text}</span>;
    case 'error':
      return <span className="text-critical-red">{line.text}</span>;
    case 'success':
      return <span className="text-electric-green">{line.text}</span>;
    case 'ai-header':
      return <span className="text-electric-purple font-bold block mt-2">{line.text}</span>;
    case 'ai-body':
      return <span className="text-gray-300 pl-4 border-l-2 border-electric-purple/30 block">{line.text}</span>;
    case 'fix-header':
        return <span className="text-yellow-400 font-bold block mt-2">{line.text}</span>;
    case 'fix-cmd':
        return <span className="text-black bg-yellow-400/90 px-2 py-0.5 rounded ml-4 font-bold block w-fit">{line.text}</span>;
    case 'prompt':
        return <span className="text-white">{line.text}</span>;
    case 'spacer':
        return <div className="h-2"></div>;
    default:
      return <span>{line.text}</span>;
  }
}
