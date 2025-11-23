import ValueProposition from '@/components/ValueProposition';
import GettingStarted from '@/components/GettingStarted';
import Footer from '@/components/Footer';

export default function Home() {
  return (
    <main className="min-h-screen">
      {/* Hero Section */}
      <section className="relative min-h-screen flex items-center justify-center bg-gradient-to-b from-deep-navy to-gray-900 text-white">
        <div className="container mx-auto px-4 py-20 max-w-6xl">
          <div className="text-center space-y-8">
            {/* Trust Indicators */}
            <div className="flex items-center justify-center gap-4 text-sm text-gray-400">
              <span className="flex items-center gap-2">
                <svg className="w-4 h-4 text-electric-green" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                </svg>
                Open Source
              </span>
              <span>•</span>
              <span>66%+ Test Coverage</span>
              <span>•</span>
              <span>Production Ready</span>
            </div>

            {/* Headline */}
            <h1 className="font-display text-5xl md:text-6xl lg:text-7xl font-bold tracking-tight text-balance">
              AI-Powered SRE Automation{" "}
              <span className="bg-gradient-to-r from-lumo-blue to-electric-green bg-clip-text text-transparent">
                That Actually Works
              </span>
            </h1>

            {/* Subheadline */}
            <p className="text-xl md:text-2xl text-gray-300 max-w-3xl mx-auto text-balance">
              87% faster incident resolution. 4,400% ROI. Natural language diagnostics for your entire infrastructure.
            </p>

            {/* CTAs */}
            <div className="flex flex-col sm:flex-row gap-4 justify-center items-center pt-4">
              <a
                href="https://github.com/IgnacioPro/lumo"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 px-8 py-4 bg-electric-green text-deep-navy font-semibold rounded-lg hover:bg-green-400 transition-all hover:scale-105 shadow-lg"
              >
                <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                  <path fillRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clipRule="evenodd" />
                </svg>
                Star on GitHub
              </a>
              <a
                href="#demo"
                className="inline-flex items-center gap-2 px-8 py-4 bg-transparent border-2 border-white text-white font-semibold rounded-lg hover:bg-white hover:text-deep-navy transition-all"
              >
                See Live Demo
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                </svg>
              </a>
            </div>
          </div>

          {/* Animated Terminal Preview Placeholder */}
          <div className="mt-16 max-w-4xl mx-auto">
            <div className="bg-gray-900 rounded-lg border border-gray-700 shadow-2xl overflow-hidden">
              <div className="flex items-center gap-2 px-4 py-3 bg-gray-800 border-b border-gray-700">
                <div className="flex gap-2">
                  <div className="w-3 h-3 rounded-full bg-red-500"></div>
                  <div className="w-3 h-3 rounded-full bg-yellow-500"></div>
                  <div className="w-3 h-3 rounded-full bg-green-500"></div>
                </div>
                <span className="ml-4 text-sm text-gray-400 font-mono">lumo</span>
              </div>
              <div className="p-6 font-mono text-sm space-y-3">
                <div className="text-electric-green">$ lumo ask &quot;why is my server slow?&quot;</div>
                <div className="text-gray-400">[Analyzing system... 2s]</div>
                <div className="mt-4 space-y-2 text-gray-300">
                  <p className="text-white font-semibold">AI Analysis:</p>
                  <p>Your server shows high I/O wait (45%).</p>
                  <p>Primary cause: PostgreSQL table bloat (12 GB).</p>
                  <p className="mt-3 text-white font-semibold">Recommendations:</p>
                  <p className="text-electric-green">✓ Run VACUUM FULL on users table (est. 15 min)</p>
                  <p className="text-electric-green">✓ Add index on created_at column</p>
                  <p className="mt-3"><span className="text-lumo-blue">Impact:</span> -80% I/O wait, 5x faster queries</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Value Proposition */}
      <ValueProposition />

      {/* Getting Started */}
      <GettingStarted />

      {/* Footer */}
      <Footer />
    </main>
  );
}
