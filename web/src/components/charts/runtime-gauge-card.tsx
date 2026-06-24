"use client";

import { Card, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface RuntimeGaugeCardProps {
  label: string;
  value: string | number;
  unit?: string;
  icon?: React.ReactNode;
  tone?: "primary" | "blue" | "green" | "violet" | "rose";
}

const toneClasses = {
  primary: "bg-primary/12 text-primary ring-primary/15",
  blue: "bg-[#5B8DB8]/12 text-[#4F7FA8] ring-[#5B8DB8]/15 dark:text-[#8AB4D6]",
  green: "bg-[#4A9E7D]/12 text-[#3D8769] ring-[#4A9E7D]/15 dark:text-[#78C3A3]",
  violet: "bg-[#7C6BA5]/12 text-[#6E5C98] ring-[#7C6BA5]/15 dark:text-[#A99AD1]",
  rose: "bg-[#C76B8A]/12 text-[#B85B7A] ring-[#C76B8A]/15 dark:text-[#E79AB4]",
};

export function RuntimeGaugeCard({
  label,
  value,
  unit,
  icon,
  tone = "primary",
}: RuntimeGaugeCardProps) {
  return (
    <Card className="bg-card/95 hover:-translate-y-0.5 hover:border-border/80 hover:shadow-[0_18px_50px_rgba(92,62,29,0.10)] dark:bg-card/90 dark:hover:shadow-[0_18px_50px_rgba(0,0,0,0.24)]">
      <CardContent className="p-4">
        <div className="flex items-start justify-between gap-3">
          <span className="max-w-28 text-xs font-medium uppercase tracking-[0.16em] text-muted-foreground sm:max-w-none">
            {label}
          </span>
          {icon && (
            <span className={cn("rounded-xl p-2 ring-1", toneClasses[tone])}>
              {icon}
            </span>
          )}
        </div>
        <div className="mt-4 flex items-baseline gap-1.5">
          <span className="font-display text-3xl font-semibold leading-none tracking-tight text-foreground tabular-nums">
            {value}
          </span>
          {unit && (
            <span className="font-mono text-xs font-medium text-muted-foreground">{unit}</span>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
