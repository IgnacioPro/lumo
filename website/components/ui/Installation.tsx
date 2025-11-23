'use client';

import { useState } from 'react';
import { Check, Copy, Terminal, Monitor, Command } from 'lucide-react';
import { motion } from 'framer-motion';

const installOptions = [
  {
    id: 'go',
    name: 'Go Install',
    icon: Terminal,
    cmd: 'go install github.com/ignacio/lumo/cmd/lumo@latest',
    description: 'Requires Go 1.25+. Installs directly from source.'
  },
  {
    id: 'brew',
    name: 'Homebrew',
    icon: Command,
    cmd: 'brew tap ignacio/lumo\nbrew install lumo',
    description: 'Recommended for macOS users. Auto-updates available.'
  },
  {
    id: 'docker',
    name: 'Docker',
    icon: Monitor, // Using Monitor as a proxy for "Container" visual
    cmd: 'docker run -it --rm \
  -v ~/.ssh:/root/.ssh \
  ghcr.io/ignacio/lumo:latest diagnose',
    description: 'Zero installation. Runs in an ephemeral container.'
  },
  {
    id: 'curl',
    name: 'Linux Script',
    icon: Terminal,
    cmd: 'curl -fsSL https://lumo.run/install.sh | sh',
    description: 'Universal installation script for Linux distributions.'
  }
];

export default function Installation() {
  const [activeTab, setActiveTab] = useState('go');
  const [copied, setCopied] = useState(false);

  const activeOption = installOptions.find(opt => opt.id === activeTab) || installOptions[0];

  const handleCopy = () => {
    navigator.clipboard.writeText(activeOption.cmd);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="max-w-4xl mx-auto">
      {/* Tabs */}
      <div className="flex flex-wrap justify-center gap-2 mb-8">
        {installOptions.map((option) => (
          <button
            key={option.id}
            onClick={() => setActiveTab(option.id)}
            className={`flex items-center gap-2 px-6 py-3 rounded-full font-medium transition-all duration-200 border ${activeTab === option.id
                ? 'bg-white text-obsidian border-white shadow-[0_0_20px_rgba(255,255,255,0.3)]'
                : 'bg-white/5 text-gray-400 border-white/10 hover:bg-white/10 hover:text-white'
            }`}
          >
            <option.icon className="w-4 h-4" />
            {option.name}
          </button>
        ))}
      </div>

      {/* Code Window */}
      <motion.div
        key={activeTab}
        initial={{ opacity: 0, scale: 0.98 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.2 }}
        className="relative group rounded-xl overflow-hidden border border-white/10 bg-[#0D1117] shadow-2xl"
      >
        {/* Window Controls */}
        <div className="flex items-center justify-between px-4 py-3 bg-white/5 border-b border-white/5">
          <div className="flex gap-2">
            <div className="w-3 h-3 rounded-full bg-[#FF5F56]" />
            <div className="w-3 h-3 rounded-full bg-[#FFBD2E]" />
            <div className="w-3 h-3 rounded-full bg-[#27C93F]" />
          </div>
          <div className="text-xs text-gray-500 font-mono">bash</div>
        </div>

        {/* Content */}
        <div className="p-8 md:p-12 flex flex-col items-center text-center md:text-left md:flex-row md:justify-between gap-6">
            <pre className="font-mono text-sm md:text-base text-gray-300 whitespace-pre-wrap text-left flex-grow">
                {activeOption.id !== 'docker' && <span className="text-electric-green select-none mr-3">$</span>}
                {activeOption.cmd}
            </pre>
            
            <button
                onClick={handleCopy}
                className="shrink-0 flex items-center gap-2 px-4 py-2 rounded-lg bg-white/10 hover:bg-white/20 text-white transition-colors font-medium text-sm border border-white/10"
            >
                {copied ? <Check className="w-4 h-4 text-electric-green" /> : <Copy className="w-4 h-4" />}
                {copied ? 'Copied!' : 'Copy'}
            </button>
        </div>
      </motion.div>

      {/* Description */}
      <motion.p 
        key={activeTab + '-desc'}
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        className="text-center text-gray-500 mt-4 text-sm"
      >
        {activeOption.description}
      </motion.p>
    </div>
  );
}
