'use client';

import { useState } from 'react';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';

const faqs = [
  {
    question: "Is it safe to let AI run commands on my infrastructure?",
    answer: "Yes. Lumo follows a 'Human-in-the-loop' philosophy. It never executes remediation commands automatically without your explicit confirmation. You always review the suggested fix, the explanation, and the exact command before it runs."
  },
  {
    question: "Does my sensitive data leave my server?",
    answer: "Diagnostics and health checks run entirely locally on your machine or server. If you enable AI analysis, only the specific error logs and metrics required for diagnosis are sent to the AI provider (like Anthropic or OpenAI) via encrypted channels. For 100% privacy, you can use Lumo with Ollama to run local LLMs completely offline."
  },
  {
    question: "Do I need to install an agent on every server?",
    answer: "No. Lumo is designed as a CLI-first tool. You can run it from your laptop and connect to any number of remote servers via SSH (`lumo connect user@host`). However, if you want continuous monitoring and background healing, you *can* optionally install it as a systemd service or Kubernetes DaemonSet."
  },
  {
    question: "How much does it cost?",
    answer: "Lumo itself is 100% Open Source (MIT License) and free to use. The only cost you might incur is from the AI provider you choose (e.g., OpenAI or Anthropic API fees) if you use the cloud-based analysis features."
  },
  {
    question: "What happens if the AI hallucinates a bad command?",
    answer: "Lumo includes a validation layer that cross-references suggested commands against a whitelist of safe operations. It also warns you if a command is considered 'destructive' (like `rm -rf` or `DROP TABLE`). Ultimately, you are the final gatekeeper."
  }
];

export default function FAQ() {
  const [openIndex, setOpenIndex] = useState<number | null>(0);

  return (
    <div className="max-w-3xl mx-auto">
      <div className="space-y-4">
        {faqs.map((faq, index) => (
          <motion.div
            key={index}
            initial={{ opacity: 0, y: 10 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ delay: index * 0.1 }}
            className="border border-white/10 rounded-lg bg-white/5 overflow-hidden"
          >
            <button
              onClick={() => setOpenIndex(openIndex === index ? null : index)}
              className="w-full flex items-center justify-between p-6 text-left hover:bg-white/5 transition-colors"
            >
              <span className="font-semibold text-white text-lg">{faq.question}</span>
              {openIndex === index ? (
                <ChevronUp className="w-5 h-5 text-lumo-blue" />
              ) : (
                <ChevronDown className="w-5 h-5 text-gray-500" />
              )}
            </button>
            
            <AnimatePresence>
              {openIndex === index && (
                <motion.div
                  initial={{ height: 0, opacity: 0 }}
                  animate={{ height: "auto", opacity: 1 }}
                  exit={{ height: 0, opacity: 0 }}
                  transition={{ duration: 0.3 }}
                >
                  <div className="px-6 pb-6 text-gray-400 leading-relaxed border-t border-white/5 pt-4">
                    {faq.answer}
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </motion.div>
        ))}
      </div>
    </div>
  );
}
