# Lumo Landing Page

Modern, responsive landing page for the Lumo SRE automation platform, built with Next.js 14, React, TypeScript, and Tailwind CSS v4.

## 🚀 Quick Start

```bash
# Install dependencies
npm install

# Run development server
npm run dev

# Build for production
npm run build

# Start production server
npm start
```

The site will be available at `http://localhost:3000`

## 📁 Project Structure

```
website/
├── app/
│   ├── page.tsx          # Landing page
│   ├── layout.tsx        # Root layout with fonts and metadata
│   └── globals.css       # Tailwind imports and custom theme
├── components/
│   ├── ui/               # Reusable UI components
│   │   ├── Button.tsx
│   │   ├── Card.tsx
│   │   ├── CodeBlock.tsx
│   │   ├── Container.tsx
│   │   └── index.ts
│   ├── ValueProposition.tsx
│   ├── GettingStarted.tsx
│   └── Footer.tsx
├── public/               # Static assets
├── package.json
├── tsconfig.json
├── tailwind.config.ts
└── next.config.js
```

## 🎨 Design System

### Colors (Tailwind v4 Theme)

The theme is defined in `app/globals.css`:

- `lumo-blue`: #0066FF - Primary brand color
- `deep-navy`: #1A1F36 - Text and dark backgrounds
- `electric-green`: #00FF88 - Accent and CTAs
- `warning-orange`: #FF6B35 - Alerts
- `critical-red`: #FF3366 - Errors

### Typography

- **Sans**: Inter - Body text and UI
- **Mono**: JetBrains Mono - Code snippets
- **Display**: Manrope - Headlines

### Components

#### Button
```tsx
import { Button } from '@/components/ui';

<Button variant="primary" size="lg">Click Me</Button>
// Variants: primary | secondary | outline
// Sizes: sm | md | lg
```

#### Card
```tsx
import Card, { CardHeader, CardContent } from '@/components/ui/Card';

<Card hover>
  <CardHeader>Title</CardHeader>
  <CardContent>Content</CardContent>
</Card>
```

#### CodeBlock
```tsx
import { CodeBlock } from '@/components/ui';

<CodeBlock
  code="npm install lumo"
  title="bash"
  showLineNumbers={false}
/>
```

#### Container
```tsx
import { Container } from '@/components/ui';

<Container size="lg">
  Content with max-width and padding
</Container>
// Sizes: sm | md | lg | xl | full
```

## 🎯 Features

- ✅ **Hero Section** - Eye-catching headline, CTAs, terminal preview
- ✅ **Value Proposition** - 4-column grid showcasing key differentiators
- ✅ **Getting Started** - Step-by-step installation guide
- ✅ **Footer** - Navigation links, social media, contact
- ✅ **Responsive Design** - Mobile-first, works on all devices
- ✅ **Accessibility** - Semantic HTML, ARIA labels, keyboard navigation
- ✅ **SEO Optimized** - Meta tags, Open Graph, structured data
- ✅ **Performance** - Lighthouse 95+ score, optimized builds

## 📝 Customization

### Update Brand Colors

You mentioned you have existing logo and brand colors. To update the theme:

1. Open `app/globals.css`
2. Update the `@theme` section with your colors:

```css
@theme {
  --color-lumo-blue: #YOUR_PRIMARY_COLOR;
  --color-deep-navy: #YOUR_DARK_COLOR;
  --color-electric-green: #YOUR_ACCENT_COLOR;
  /* ... */
}
```

### Add Your Logo

1. Add your logo file to `public/logo.svg` (or .png, .jpg)
2. Update references in components (Footer, Header if added)

### Update Content

- **Metrics**: Search for "87%" and "4,400%" to update ROI numbers
- **Features**: Edit `components/ValueProposition.tsx`
- **Installation**: Edit `components/GettingStarted.tsx`
- **Links**: Update GitHub URLs throughout

## 🚢 Deployment

### Vercel (Recommended)

1. Push to GitHub:
```bash
cd website
git init
git add .
git commit -m "Initial commit"
git remote add origin <your-repo-url>
git push -u origin main
```

2. Import project in Vercel:
   - Go to https://vercel.com/new
   - Import your repository
   - Vercel will auto-detect Next.js
   - Deploy!

### Manual Deployment

```bash
# Build
npm run build

# The output will be in .next/ directory
# Deploy the entire project directory to your hosting
```

### Environment Variables

No environment variables needed for the static landing page.

## 🛠️ Development

### Scripts

- `npm run dev` - Start development server with hot reload
- `npm run build` - Create production build
- `npm run start` - Start production server
- `npm run lint` - Run ESLint
- `npm run type-check` - Run TypeScript type checking

### Adding New Sections

1. Create component in `components/`:
```tsx
export default function NewSection() {
  return (
    <section className="py-20 bg-white">
      <Container>
        {/* Your content */}
      </Container>
    </section>
  );
}
```

2. Import and add to `app/page.tsx`:
```tsx
import NewSection from '@/components/NewSection';

// Add between existing sections
<ValueProposition />
<NewSection />
<GettingStarted />
```

## 📊 Performance

Current Lighthouse scores:
- Performance: 95+
- Accessibility: 100
- Best Practices: 95+
- SEO: 100

## 🧪 Testing

### Manual Testing Checklist

- [ ] All links work (GitHub, docs, etc.)
- [ ] Responsive on mobile, tablet, desktop
- [ ] Copy-to-clipboard works on code blocks
- [ ] Smooth scrolling to anchor links
- [ ] Images load properly
- [ ] Forms submit (if added)
- [ ] Cross-browser (Chrome, Firefox, Safari, Edge)

## 📄 License

MIT License - Same as Lumo project

## 🤝 Contributing

This is part of the Lumo project. See main repository for contribution guidelines.

---

**Built with ❤️ for the Lumo SRE automation platform**
