import type { Metadata } from "next";
import { Source_Serif_4 } from "next/font/google";
// Maple Mono CN (monospace, CJK + Latin, unicode-range subset, loaded on demand).
// The @font-face rules are generated from @chinese-fonts/maple-mono-cn by
// scripts/gen-font-css.mjs and point at Cloudflare R2 (static.lvlvko.top), so
// the woff2 files never enter this build or the Go binary. Only the two weights
// reachable through the "Maple Mono CN" family name are registered: the package
// keeps Medium/SemiBold under their own family names and src's local() matches
// on that same name, so nothing can select them; font-medium/font-semibold
// therefore resolve to the 400/700 faces. Subsets swap in after first paint;
// styles/font-fallback.css keeps the pre-swap text at identical widths.
import "../styles/maple-mono-cn.css";
import { AuthProvider } from "@/lib/auth-context";
import { I18nProvider } from "@/lib/i18n";
import { THEME_INIT_SCRIPT, ThemeProvider } from "@/lib/theme";
import { MOTION_TIER_INIT_SCRIPT } from "@/lib/motion/tier";
import { HtmlLangUpdater } from "@/components/html-lang-updater";
import { Toaster } from "@/components/ui/sonner";
import { InteractionLayer } from "@/components/motion/interaction-layer";
import "./globals.css";

const sourceSerif = Source_Serif_4({
  variable: "--font-source-serif",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Aris MaaS",
  description: "Management interface for Aris MaaS",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html
      lang="en"
      className={`h-full antialiased ${sourceSerif.variable}`}
      suppressHydrationWarning
    >
      <body className="min-h-full flex flex-col">
        {/* Maple Mono CN is fetched cross-origin from R2; warm DNS + TLS before
            the first woff2 request. `crossOrigin` must match the font request's
            CORS mode or the preconnected socket goes unused. React hoists this
            link into <head>. */}
        <link rel="preconnect" href="https://static.lvlvko.top" crossOrigin="anonymous" />
        {/* Raw inline script: runs synchronously during HTML parsing as
            the first element of <body>, before any visible content is
            parsed/painted, so [data-theme] is correct on first paint
            and there is no light→dark flash on full-page loads (login
            OAuth flow). Do NOT switch to next/script — its
            beforeInteractive strategy is deferred via the __next_s
            queue and only runs after the React bundle loads. The second
            script writes [data-motion-tier] (lib/motion/tier.ts). Keep
            them separate: THEME_INIT_SCRIPT comes from a "use client"
            module, so in this server component it is a client reference
            that only resolves when passed as a prop as-is — string
            concatenation would stringify the reference proxy instead. */}
        <script dangerouslySetInnerHTML={{ __html: THEME_INIT_SCRIPT }} />
        <script dangerouslySetInnerHTML={{ __html: MOTION_TIER_INIT_SCRIPT }} />
        <I18nProvider>
          <ThemeProvider>
            <HtmlLangUpdater />
            <AuthProvider>{children}</AuthProvider>
            <InteractionLayer />
            <Toaster />
          </ThemeProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
