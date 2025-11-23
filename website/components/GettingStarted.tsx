import Container from './ui/Container';
import CodeBlock from './ui/CodeBlock';

export default function GettingStarted() {
  return (
    <section id="getting-started" className="py-20 bg-white">
      <Container>
        <div className="text-center mb-16">
          <h2 className="font-display text-4xl md:text-5xl font-bold text-deep-navy mb-4">
            Get Started in 5 Minutes
          </h2>
          <p className="text-xl text-gray-600 max-w-3xl mx-auto">
            Install Lumo, configure your AI provider, and run your first diagnostic
          </p>
        </div>

        <div className="max-w-3xl mx-auto space-y-12">
          {/* Step 1: Install */}
          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 bg-lumo-blue text-white rounded-full flex items-center justify-center font-bold">
                1
              </div>
              <h3 className="font-display text-2xl font-bold text-deep-navy">
                Install Lumo
              </h3>
            </div>
            <div className="ml-13 space-y-4">
              <p className="text-gray-600">Choose your installation method:</p>

              <div className="space-y-3">
                <div>
                  <p className="text-sm text-gray-500 mb-2 font-semibold">Quick Install (Recommended)</p>
                  <CodeBlock
                    title="bash"
                    code="curl -sSL https://lumo.dev/install.sh | bash"
                  />
                </div>

                <div>
                  <p className="text-sm text-gray-500 mb-2 font-semibold">Via Go</p>
                  <CodeBlock
                    title="bash"
                    code="go install github.com/IgnacioPro/lumo/cmd/lumo@latest"
                  />
                </div>

                <div>
                  <p className="text-sm text-gray-500 mb-2 font-semibold">From Source</p>
                  <CodeBlock
                    title="bash"
                    code={`git clone https://github.com/IgnacioPro/lumo.git
cd lumo
make build`}
                  />
                </div>
              </div>
            </div>
          </div>

          {/* Step 2: Configure */}
          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 bg-electric-green text-deep-navy rounded-full flex items-center justify-center font-bold">
                2
              </div>
              <h3 className="font-display text-2xl font-bold text-deep-navy">
                Configure AI Provider
              </h3>
            </div>
            <div className="ml-13 space-y-4">
              <p className="text-gray-600">
                Set your AI provider API key (supports Anthropic, OpenAI, Gemini, Ollama, OpenRouter):
              </p>
              <CodeBlock
                title="bash"
                code={`export LUMO_AI_PROVIDER=anthropic
export LUMO_ANTHROPIC_API_KEY=sk-ant-...`}
              />
              <p className="text-sm text-gray-500">
                → See{' '}
                <a
                  href="https://github.com/IgnacioPro/lumo/blob/main/docs/getting-started.md"
                  className="text-lumo-blue hover:underline"
                >
                  configuration guide
                </a>{' '}
                for all options
              </p>
            </div>
          </div>

          {/* Step 3: Run */}
          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 bg-warning-orange text-white rounded-full flex items-center justify-center font-bold">
                3
              </div>
              <h3 className="font-display text-2xl font-bold text-deep-navy">
                Run Your First Diagnostic
              </h3>
            </div>
            <div className="ml-13 space-y-4">
              <p className="text-gray-600">Try the natural language interface:</p>
              <CodeBlock
                title="bash"
                code='lumo ask "check CPU and memory usage"'
              />
              <p className="text-gray-600 mt-4">Or run traditional diagnostics:</p>
              <CodeBlock
                title="bash"
                code="lumo diagnose localhost --analyze"
              />
              <div className="flex items-start gap-2 bg-green-50 border border-green-200 rounded-lg p-4 mt-4">
                <svg className="w-5 h-5 text-green-600 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
                </svg>
                <div>
                  <p className="text-sm font-semibold text-green-900">Success!</p>
                  <p className="text-sm text-green-700">
                    You'll see AI-powered diagnostics and recommendations in seconds.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div className="mt-16 text-center flex flex-col sm:flex-row gap-4 justify-center">
          <a
            href="https://github.com/IgnacioPro/lumo"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center justify-center gap-2 px-8 py-4 bg-electric-green text-deep-navy font-semibold rounded-lg hover:bg-green-400 transition-all hover:scale-105 shadow-lg"
          >
            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
              <path fillRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clipRule="evenodd" />
            </svg>
            View on GitHub
          </a>
          <a
            href="https://github.com/IgnacioPro/lumo/tree/main/docs"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center justify-center gap-2 px-8 py-4 bg-white border-2 border-lumo-blue text-lumo-blue font-semibold rounded-lg hover:bg-lumo-blue hover:text-white transition-all"
          >
            Read Full Documentation
          </a>
        </div>
      </Container>
    </section>
  );
}
