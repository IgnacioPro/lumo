import type { Metadata } from "next";
import { Inter, JetBrains_Mono, Manrope } from "next/font/google";
import "./globals.css";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains-mono",
  display: "swap",
});

const manrope = Manrope({
  subsets: ["latin"],
  variable: "--font-manrope",
  display: "swap",
  weight: ["700", "800"],
});

export const metadata: Metadata = {
  title: "Lumo - AI-Powered SRE Automation That Actually Works",
  description: "87% faster incident resolution. 4,400% ROI. Natural language diagnostics for your entire infrastructure. Open source, production-ready SRE automation platform.",
  keywords: ["SRE", "DevOps", "automation", "AI", "monitoring", "diagnostics", "kubernetes", "infrastructure"],
  authors: [{ name: "Lumo Team" }],
  openGraph: {
    title: "Lumo - AI-Powered SRE Automation",
    description: "87% faster incident resolution. Natural language diagnostics for your entire infrastructure.",
    type: "website",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={`${inter.variable} ${jetbrainsMono.variable} ${manrope.variable}`}>
      <body className="font-sans">
        {children}
      </body>
    </html>
  );
}
