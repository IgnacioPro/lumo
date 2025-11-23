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
      example: 'lumo ask "check CPU usage on all servers"',
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
      example: 'Learns from every incident in your environment',
      highlight: 'Recommendations get smarter with each incident',
    },
    {
      icon: (
        <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
        </svg>
      ),
      title: 'Multi-Cloud Native',
      description: 'Kubernetes, VMs, cloud, on-prem — one interface',
      example: 'K8s • AWS • Azure • GCP • Proxmox',
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
      example: 'github.com/IgnacioPro/lumo',
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
                  <CodeBlock
                    code={feature.example}
                    className="text-xs"
                  />
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
