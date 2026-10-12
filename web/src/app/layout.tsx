import type { Metadata } from "next";
import { Source_Serif_4 } from "next/font/google";
// Maple Mono CN (monospace, CJK + Latin, unicode-range subset, loaded on demand).
// Only the two weights reachable through the "Maple Mono CN" family name are
// imported: the package registers Medium/SemiBold under their own family names
// ("Maple Mono CN Medium" / "Maple Mono CN SemiBold"), no font-family in this
// app references them, and src's local() matches on that same name — so those
// files can never be selected and importing them only ships ~18MB of dead fonts.
// font-medium/font-semibold therefore resolve to the 400/700 faces.
import "@chinese-fonts/maple-mono-cn/dist/MapleMono-CN-Regular/result.css";
import "@chinese-fonts/maple-mono-cn/dist/MapleMono-CN-Bold/result.css";
import { AuthProvider } from "@/lib/auth-context";
import { I18nProvider } from "@/lib/i18n";
import { THEME_INIT_SCRIPT, ThemeProvider } from "@/lib/theme";
import { MOTION_TIER_INIT_SCRIPT } from "@/lib/motion/tier";
import { HtmlLangUpdater } from "@/components/html-lang-updater";
import { Toaster } from "@/components/ui/sonner";
import { AmbientStage } from "@/components/ambient/ambient-stage";
import { ThemeSwitcher } from "@/components/theme/theme-switcher";
import "./globals.css";

const sourceSerif = Source_Serif_4({
  variable: "--font-source-serif",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Aris Proxy API",
  description: "Management interface for Aris Proxy API",
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
            <AmbientStage />
            <ThemeSwitcher />
            <Toaster />
          </ThemeProvider>
        </I18nProvider>
      </body>
    </html>
  );
}
