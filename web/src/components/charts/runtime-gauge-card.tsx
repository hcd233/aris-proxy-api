"use client";

import { Card, CardContent } from "@/components/ui/card";

interface RuntimeGaugeCardProps {
  label: string;
  value: string | number;
  unit?: string;
  icon?: React.ReactNode;
}

export function RuntimeGaugeCard({
  label,
  value,
  unit,
  icon,
}: RuntimeGaugeCardProps) {
  return (
    <Card>
      <CardContent className="p-4">
        <div className="flex items-center justify-between">
          <span className="text-sm text-muted-foreground">{label}</span>
          {icon && <span className="text-muted-foreground">{icon}</span>}
        </div>
        <div className="mt-2 flex items-baseline gap-1">
          <span className="text-2xl font-semibold tabular-nums">
            {value}
          </span>
          {unit && (
            <span className="text-sm text-muted-foreground">{unit}</span>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
