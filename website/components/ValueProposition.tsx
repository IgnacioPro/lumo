import Container from './ui/Container';
import Card, { CardHeader, CardContent } from './ui/Card';
import CodeBlock from './ui/CodeBlock';

export default function ValueProposition() {
  const features = [
    {
      icon: (
        <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
        </svg>
      ),
      title: 'Natural Language Interface',
      description: 'Ask in plain English, get instant answers',
      type: 'code' as const,
      content: 'lumo ask "check CPU usage on all servers"',
      highlight: 'No more memorizing 15 different CLI syntaxes',
    },
    {
      icon: (
        <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
        </svg>
      ),
      title: 'RAG-Powered Intelligence',
      description: '87% MTTR reduction through contextual learning',
      type: 'metric' as const,
      content: '87%',
      subtitle: 'MTTR Reduction',
      highlight: 'Learns from every incident to provide smarter recommendations',
    },
    {
      icon: (
        <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
        </svg>
      ),
      title: 'Multi-Cloud Native',
      description: 'Kubernetes, VMs, cloud, on-prem — one interface',
      type: 'badges' as const,
      content: ['Kubernetes', 'AWS', 'Azure', 'GCP', 'Proxmox'],
      highlight: 'Works everywhere your infrastructure lives',
    },
    {
      icon: (
        <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
        </svg>
      ),
      title: 'Open Source Transparency',
      description: 'MIT licensed, fully auditable code',
      type: 'link' as const,
      content: 'View on GitHub',
      url: 'https://github.com/IgnacioPro/lumo',
      highlight: 'No vendor lock-in. Extend it yourself.',
    },
  ];

  return (
    <section className="py-20 bg-gray-50">
      <Container>
        <div className="text-center mb-16">
          <h2 className="font-display text-4xl md:text-5xl font-bold text-deep-navy mb-4">
            The Lumo Difference
          </h2>
          <p className="text-xl text-gray-600 max-w-3xl mx-auto">
            Four unique capabilities that set Lumo apart from traditional monitoring tools
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          {features.map((feature, index) => (
            <Card key={index} hover className="h-full">
              <CardHeader>
                <div className="w-12 h-12 bg-lumo-blue/10 rounded-lg flex items-center justify-center text-lumo-blue mb-4">
                  {feature.icon}
                </div>
                <h3 className="font-display text-xl font-bold text-deep-navy mb-2">
                  {feature.title}
                </h3>
                <p className="text-gray-600 text-sm">
                  {feature.description}
                </p>
              </CardHeader>
              <CardContent>
                <div className="space-y-3">
                  {/* Code example for Natural Language */}
                  {feature.type === 'code' && (
                    <CodeBlock code={feature.content} className="text-xs" />
                  )}

                  {/* Metric display for RAG Intelligence */}
                  {feature.type === 'metric' && (
                    <div className="bg-gradient-to-br from-lumo-blue to-electric-green p-6 rounded-lg text-center">
                      <div className="text-5xl font-bold text-white mb-1">
                        {feature.content}
                      </div>
                      <div className="text-sm font-semibold text-white/90">
                        {feature.subtitle}
                      </div>
                    </div>
                  )}

                  {/* Platform badges for Multi-Cloud */}
                  {feature.type === 'badges' && (
                    <div className="flex flex-wrap gap-2">
                      {(feature.content as string[]).map((platform) => (
                        <span
                          key={platform}
                          className="px-3 py-1 bg-deep-navy/5 border border-deep-navy/10 rounded-full text-xs font-semibold text-deep-navy"
                        >
                          {platform}
                        </span>
                      ))}
                    </div>
                  )}

                  {/* GitHub link for Open Source */}
                  {feature.type === 'link' && (
                    <a
                      href={feature.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center justify-center gap-2 px-4 py-3 bg-deep-navy text-white rounded-lg hover:bg-deep-navy/90 transition-colors font-semibold text-sm"
                    >
                      <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                        <path fillRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clipRule="evenodd" />
                      </svg>
                      {feature.content}
                    </a>
                  )}

                  <p className="text-sm text-gray-700 italic">
                    → {feature.highlight}
                  </p>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>

        <div className="mt-12 text-center">
          <p className="text-gray-600">
            Trusted by SRE teams worldwide •{' '}
            <a
              href="https://github.com/IgnacioPro/lumo"
              target="_blank"
              rel="noopener noreferrer"
              className="text-lumo-blue hover:underline font-semibold"
            >
              Star on GitHub
            </a>
          </p>
        </div>
      </Container>
    </section>
  );
}
