'use client';

import Link from 'next/link';
import { Github } from 'lucide-react';
import { motion } from 'framer-motion';

export default function Navbar() {
  return (
    <motion.nav 
      initial={{ y: -100 }}
      animate={{ y: 0 }}
      className="fixed top-0 left-0 right-0 z-50 flex items-center justify-between px-6 py-4 border-b border-white/10 bg-obsidian/80 backdrop-blur-md"
    >
      <div className="flex items-center gap-2">
        <div className="w-8 h-8 bg-gradient-to-br from-lumo-blue to-electric-green rounded-lg flex items-center justify-center">
          <span className="font-mono font-bold text-obsidian text-lg">L</span>
        </div>
        <span className="text-xl font-bold tracking-tight font-display">Lumo</span>
      </div>

      <div className="hidden md:flex items-center gap-8 text-sm font-medium text-gray-400">
        <Link href="#features" className="hover:text-white transition-colors">Features</Link>
        <Link href="#how-it-works" className="hover:text-white transition-colors">How it Works</Link>
        <Link href="https://github.com/IgnacioPro/lumo" target="_blank" className="hover:text-white transition-colors">Docs</Link>
      </div>

      <div className="flex items-center gap-4">
        <Link 
          href="https://github.com/IgnacioPro/lumo" 
          target="_blank"
          className="hidden sm:flex items-center gap-2 text-gray-400 hover:text-white transition-colors"
        >
          <Github className="w-5 h-5" />
          <span className="hidden lg:inline">Star on GitHub</span>
        </Link>
        <Link
          href="https://github.com/IgnacioPro/lumo"
          className="px-4 py-2 bg-white text-obsidian font-semibold rounded-md hover:bg-gray-200 transition-colors text-sm"
        >
          Get Started
        </Link>
      </div>
    </motion.nav>
  );
}
