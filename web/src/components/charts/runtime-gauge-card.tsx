"use client";

import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

interface RuntimeGaugeCardProps {
  label: string;
  value: string | number;
  unit?: string;
  icon?: React.ReactNode;
  tone?: "primary" | "blue" | "green" | "violet" | "rose";
  loading?: boolean;
}

const toneClasses = {
  primary: "text-primary",
  blue: "text-[#4F7FA8] dark:text-[#8AB4D6]",
  green: "text-[#3D8769] dark:text-[#78C3A3]",
  violet: "text-[#6E5C98] dark:text-[#A99AD1]",
  rose: "text-[#B85B7A] dark:text-[#E79AB4]",
};

export function RuntimeGaugeCard({
  label,
  value,
  unit,
  icon,
  tone = "primary",
  loading = false,
}: RuntimeGaugeCardProps) {
  return (
    <Card className="hover:border-border/60">
      <CardHeader className="flex flex-row items-center justify-between pb-2">
        <div className="flex items-center gap-2 text-muted-foreground">
          {icon && <span className={cn(toneClasses[tone])}>{icon}</span>}
          <span className="text-xs font-medium uppercase tracking-wider">{label}</span>
        </div>
      </CardHeader>
      <CardContent className="pt-0">
        {loading ? (
          <Skeleton className="h-9 w-20" />
        ) : (
          <div className="flex items-baseline gap-1.5">
            <span className="font-display text-3xl font-semibold text-foreground tabular-nums">
              {value}
            </span>
            {unit && (
              <span className="font-mono text-xs font-medium text-muted-foreground">{unit}</span>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
