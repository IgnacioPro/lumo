import Navbar from '@/components/ui/Navbar';
import Terminal from '@/components/ui/Terminal';
import BentoGrid from '@/components/ui/BentoGrid';
import Comparison from '@/components/ui/Comparison';
import FAQ from '@/components/ui/FAQ';
import Installation from '@/components/ui/Installation';
import ArchitectureTabs from '@/components/ui/ArchitectureTabs'; // Import ArchitectureTabs
import Link from 'next/link';
import { 
  ArrowRight, 
  Terminal as TerminalIcon, 
  Download, 
  ServerCog, 
  Container, 
  Server,    
  Cloud,     
  Code,      
  Database   
} from 'lucide-react';

export default function Home() {
  return (
    <main className="min-h-screen bg-obsidian selection:bg-electric-green/30 selection:text-white overflow-x-hidden">
      <Navbar />

      {/* HERO SECTION */}
      <section className="relative min-h-screen flex flex-col items-center justify-center pt-32 pb-20 px-4">
        {/* Background Glow */}
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-full h-[600px] bg-gradient-to-b from-lumo-blue/10 via-transparent to-transparent blur-3xl -z-10" />
        
        <div className="text-center max-w-4xl mx-auto space-y-8 mb-16">
          <div className="inline-flex items-center gap-2 px-4 py-2 rounded-full bg-white/5 border border-white/10 text-sm text-gray-400 backdrop-blur-sm">
            <span className="w-2 h-2 rounded-full bg-electric-green animate-pulse" />
            v0.9.1 is now available
          </div>
          
          <h1 className="font-display text-5xl md:text-7xl font-bold tracking-tighter text-balance bg-clip-text text-transparent bg-gradient-to-b from-white to-white/60">
            The AI SRE that <br />
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-lumo-blue via-electric-purple to-electric-green">
              fixes your infrastructure.
            </span>
          </h1>
          
          <p className="text-xl text-gray-400 max-w-2xl mx-auto leading-relaxed">
            Diagnose, Analyze, and Remediate incidents in seconds. 
            The open-source CLI that turns alerts into solutions without the context switching.
          </p>

          <div className="flex flex-col sm:flex-row gap-4 justify-center items-center">
            <div className="flex items-center gap-3 px-6 py-4 bg-[#161B22] rounded-lg border border-gray-700 font-mono text-sm text-gray-300 shadow-lg min-w-[280px]">
              <span className="text-electric-green">$</span>
              <span>go install github.com/ignacio/lumo...</span>
              <button className="ml-auto text-gray-500 hover:text-white transition-colors">
                <Download className="w-4 h-4" />
              </button>
            </div>
            <Link
              href="https://github.com/IgnacioPro/lumo"
              className="px-8 py-4 bg-white text-obsidian font-bold rounded-lg hover:bg-gray-200 transition-all flex items-center gap-2 shadow-lg hover:shadow-white/10 hover:scale-105"
            >
              Get Started
              <ArrowRight className="w-4 h-4" />
            </Link>
          </div>
        </div>

        <div className="w-full px-4 md:px-0">
          <Terminal />
        </div>
      </section>

      {/* SOCIAL PROOF */}
      <section className="py-10 border-y border-white/5 bg-black/20">
        <div className="max-w-6xl mx-auto px-4 text-center">
          <p className="text-sm text-gray-500 font-mono mb-6 uppercase tracking-widest">Designed for modern stacks</p>
          <div className="flex flex-wrap justify-center items-center gap-x-12 gap-y-6 opacity-50 grayscale hover:grayscale-0 transition-all duration-500">
             {/* Icons for tech stack */}
             <div className="flex flex-col items-center gap-2">
               <ServerCog className="w-8 h-8 text-white" />
               <span className="text-xs text-white/60">Kubernetes</span>
             </div>
             <div className="flex flex-col items-center gap-2">
               <Container className="w-8 h-8 text-white" />
               <span className="text-xs text-white/60">Docker</span>
             </div>
             <div className="flex flex-col items-center gap-2">
               <Server className="w-8 h-8 text-white" />
               <span className="text-xs text-white/60">Linux</span>
             </div>
             <div className="flex flex-col items-center gap-2">
               <Cloud className="w-8 h-8 text-white" />
               <span className="text-xs text-white/60">AWS</span>
             </div>
             <div className="flex flex-col items-center gap-2">
               <Code className="w-8 h-8 text-white" />
               <span className="text-xs text-white/60">Go</span>
             </div>
             <div className="flex flex-col items-center gap-2">
               <Database className="w-8 h-8 text-white" />
               <span className="text-xs text-white/60">PostgreSQL</span>
             </div>
          </div>
        </div>
      </section>

      {/* PROBLEM VS SOLUTION */}
      <section className="py-32 px-4">
        <div className="text-center mb-16">
          <h2 className="text-3xl md:text-5xl font-bold font-display mb-6">Stop Debugging. Start Fixing.</h2>
          <p className="text-gray-400 max-w-2xl mx-auto">
            The average incident takes 45 minutes to resolve manually. Lumo does it in under 2 minutes.
          </p>
        </div>
        <Comparison />
      </section>
      
      {/* HOW IT WORKS (ARCHITECTURE TABS) */}
      <section id="how-it-works" className="py-20 px-4 bg-black/30 border-y border-white/5">
         <div className="text-center mb-16">
          <h2 className="text-3xl md:text-5xl font-bold font-display mb-6">How Lumo Works</h2>
          <p className="text-gray-400 max-w-xl mx-auto">
            Flexible deployment options. Run Lumo locally as a CLI tool or deploy it as a persistent agent in your Kubernetes cluster.
          </p>
        </div>
        <ArchitectureTabs />
      </section>

      {/* FEATURES GRID */}
      <section id="features" className="py-20 px-4 bg-gradient-to-b from-transparent to-lumo-blue/5">
        <div className="text-center mb-16">
           <h2 className="text-3xl md:text-5xl font-bold font-display mb-6">Everything you need to manage scale</h2>
        </div>
        <BentoGrid />
      </section>

      {/* INSTALLATION HUB */}
      <section id="install" className="py-20 px-4">
         <div className="text-center mb-12">
          <h2 className="text-3xl md:text-5xl font-bold font-display mb-6">Get Started in Seconds</h2>
          <p className="text-gray-400 max-w-xl mx-auto">
            Lumo is a single binary. No complex dependencies. No agents required.
          </p>
        </div>
        <Installation />
      </section>

      {/* FAQ SECTION */}
      <section className="py-20 px-4 bg-black/20">
        <div className="text-center mb-16">
          <h2 className="text-3xl md:text-5xl font-bold font-display mb-6">Questions?</h2>
          <p className="text-gray-400 max-w-xl mx-auto">
            Common questions about safety, privacy, and deployment.
          </p>
        </div>
        <FAQ />
      </section>

      {/* CTA */}
      <section className="py-32 px-4 relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-r from-lumo-blue/20 to-electric-green/20 blur-[100px] -z-10" />
        
        <div className="max-w-4xl mx-auto text-center bg-black/40 backdrop-blur-xl border border-white/10 rounded-3xl p-12 md:p-20">
          <h2 className="text-4xl md:text-5xl font-bold font-display mb-8">Ready to upgrade your workflow?</h2>
          <p className="text-xl text-gray-300 mb-10 max-w-2xl mx-auto">
            Join hundreds of engineers using Lumo to keep their systems healthy and their weekends free.
          </p>
          <div className="flex flex-col sm:flex-row gap-6 justify-center">
             <Link
              href="#install" // Anchor link to the new Install section
              className="px-10 py-5 bg-white text-obsidian font-bold rounded-xl hover:bg-gray-200 transition-all flex items-center justify-center gap-3 text-lg shadow-xl"
            >
              <Download className="w-5 h-5" />
              Install CLI Tool
            </Link>
            <Link
              href="https://github.com/IgnacioPro/lumo"
              className="px-10 py-5 bg-transparent border border-white/20 text-white font-bold rounded-xl hover:bg-white/10 transition-all flex items-center justify-center gap-3 text-lg"
            >
              <TerminalIcon className="w-5 h-5" />
              View Documentation
            </Link>
          </div>
        </div>
      </section>

      {/* FOOTER */}
      <footer className="py-12 border-t border-white/10 bg-black/40">
        <div className="max-w-6xl mx-auto px-4 flex flex-col md:flex-row justify-between items-center gap-6">
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 bg-gradient-to-br from-lumo-blue to-electric-green rounded-md"></div>
            <span className="font-bold text-lg">Lumo</span>
          </div>
          <div className="text-gray-500 text-sm">
            © {new Date().getFullYear()} Lumo. Open Source (MIT).
          </div>
          <div className="flex gap-6 text-gray-400">
            <Link href="#" className="hover:text-white transition-colors">Privacy</Link>
            <Link href="#" className="hover:text-white transition-colors">Terms</Link>
            <Link href="https://github.com/IgnacioPro/lumo" className="hover:text-white transition-colors">GitHub</Link>
          </div>
        </div>
      </footer>
    </main>
  );
}
