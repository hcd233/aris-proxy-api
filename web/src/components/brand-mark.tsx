/**
 * BrandMark — the Aris starburst.
 *
 * Anthropic-style radial asterisk: eight hand-drawn-feel rays, alternating
 * long/short, rendered in currentColor so it can sit on ink squares or
 * directly on the parchment. Replaces the old letter-"A" tiles.
 */
export function BrandMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" className={className} aria-hidden="true">
      {Array.from({ length: 8 }).map((_, i) => (
        <line
          key={i}
          x1="12"
          y1="12"
          x2="12"
          y2={i % 2 === 0 ? "3.2" : "6.2"}
          stroke="currentColor"
          strokeWidth="1.7"
          strokeLinecap="round"
          transform={`rotate(${i * 45 + 22.5} 12 12)`}
        />
      ))}
    </svg>
  );
}
