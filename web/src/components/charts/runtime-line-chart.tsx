"use client";

import {
  CartesianGrid,
  Line,
  LineChart,
  XAxis,
  YAxis,
} from "recharts";

import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface RuntimeLineChartProps {
  data: { time: string; value: number }[];
  dataKey: string;
  label: string;
  color?: string;
  unit?: string;
  sampleLabel?: string;
  className?: string;
  heightClassName?: string;
  accent?: "primary" | "blue" | "green" | "violet" | "rose";
}

const accentClasses = {
  primary: "from-primary/16 via-primary/5",
  blue: "from-[#5B8DB8]/16 via-[#5B8DB8]/5",
  green: "from-[#4A9E7D]/16 via-[#4A9E7D]/5",
  violet: "from-[#7C6BA5]/16 via-[#7C6BA5]/5",
  rose: "from-[#C76B8A]/16 via-[#C76B8A]/5",
};

export function RuntimeLineChart({
  data,
  dataKey,
  label,
  color = "#D97757",
  unit,
  sampleLabel = "Last {count} samples",
  className,
  heightClassName = "h-[190px]",
  accent = "primary",
}: RuntimeLineChartProps) {
  const config: ChartConfig = {
    [dataKey]: { label, color },
  };

  const latest = data.at(-1)?.value;

  return (
    <Card className={cn("relative overflow-hidden bg-card/95 dark:bg-card/90", className)}>
      <div className={cn("pointer-events-none absolute inset-x-0 top-0 h-24 bg-gradient-to-b to-transparent", accentClasses[accent])} />
      <CardHeader className="relative flex flex-row items-start justify-between gap-4 pb-0">
        <div>
          <CardTitle className="font-display text-base">{label}</CardTitle>
          <p className="mt-1 text-xs text-muted-foreground">
            {sampleLabel.replace("{count}", String(data.length))}
          </p>
        </div>
        {latest !== undefined && (
          <div className="rounded-xl border border-border/70 bg-background/70 px-3 py-2 text-right shadow-sm supports-[backdrop-filter]:bg-background/55">
            <div className="font-display text-2xl font-semibold leading-none tabular-nums">
              {latest}
            </div>
            {unit && <div className="mt-1 font-mono text-[10px] text-muted-foreground">{unit.trim()}</div>}
          </div>
        )}
      </CardHeader>
      <CardContent className="relative pt-1">
        <ChartContainer config={config} className={cn("w-full", heightClassName)}>
          <LineChart data={data} margin={{ left: 8, right: 12, top: 18, bottom: 4 }}>
            <CartesianGrid vertical={false} strokeDasharray="3 3" />
            <XAxis
              dataKey="time"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              minTickGap={32}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              width={48}
            />
            <ChartTooltip
              content={
                <ChartTooltipContent
                  labelKey={dataKey}
                  indicator="line"
                  formatter={(value) => [
                    `${value}${unit ?? ""}`,
                    label,
                  ]}
                />
              }
            />
            <Line
              dataKey={dataKey}
              type="monotone"
              stroke={color}
              strokeWidth={2.5}
              dot={false}
              isAnimationActive={false}
            />
          </LineChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
