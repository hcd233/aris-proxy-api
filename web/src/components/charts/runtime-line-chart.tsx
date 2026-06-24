"use client";

import {
  Area,
  AreaChart,
  CartesianGrid,
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

const accentDot = {
  primary: "bg-primary",
  blue: "bg-[#5B8DB8]",
  green: "bg-[#4A9E7D]",
  violet: "bg-[#7C6BA5]",
  rose: "bg-[#C76B8A]",
};

export function RuntimeLineChart({
  data,
  dataKey,
  label,
  color = "#D97757",
  unit,
  sampleLabel = "Last {count} samples",
  className,
  heightClassName = "h-[220px]",
  accent = "primary",
}: RuntimeLineChartProps) {
  const config: ChartConfig = {
    [dataKey]: { label, color },
  };

  const latest = data.at(-1)?.value;
  const gradientId = `monitor-grad-${dataKey}`;
  const isEmpty = data.length === 0;

  return (
    <Card className={cn(className)}>
      <CardHeader className="flex flex-row items-center justify-between">
        <div>
          <CardTitle className="font-display">{label}</CardTitle>
          <p className="mt-1 text-xs text-muted-foreground">
            {sampleLabel.replace("{count}", String(data.length))}
          </p>
        </div>
        {!isEmpty && latest !== undefined && (
          <div className="flex items-center gap-2 rounded-full border border-border/70 bg-muted/50 px-2.5 py-1">
            <span className="relative flex size-2">
              <span className={cn("absolute inline-flex size-full animate-ping rounded-full opacity-60", accentDot[accent])} />
              <span className={cn("relative inline-flex size-2 rounded-full", accentDot[accent])} />
            </span>
            <span className="text-xs font-medium text-muted-foreground">Live</span>
            <span className="font-mono text-sm font-semibold text-foreground tabular-nums">
              {latest}
              {unit ? unit.trim() : ""}
            </span>
          </div>
        )}
      </CardHeader>
      <CardContent>
        {isEmpty ? (
          <div className={cn("flex items-center justify-center text-sm text-muted-foreground", heightClassName)}>
            Collecting data…
          </div>
        ) : (
          <ChartContainer config={config} className={cn("w-full", heightClassName)}>
            <AreaChart data={data} margin={{ left: 8, right: 12, top: 8, bottom: 4 }}>
              <defs>
                <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={color} stopOpacity={0.28} />
                  <stop offset="100%" stopColor={color} stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid vertical={false} strokeDasharray="3 3" />
              <XAxis
                dataKey="time"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                minTickGap={32}
                fontSize={12}
              />
              <YAxis
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                width={44}
                fontSize={12}
              />
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    labelKey={dataKey}
                    indicator="line"
                    formatter={(value) => {
                      const indicatorColor = color;
                      return (
                        <>
                          <div
                            className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                            style={{ backgroundColor: indicatorColor }}
                          />
                          <div className="flex flex-1 items-center justify-between leading-none">
                            <span className="text-muted-foreground">{label}</span>
                            <span className="font-mono font-medium text-foreground tabular-nums">
                              {value}{unit ?? ""}
                            </span>
                          </div>
                        </>
                      );
                    }}
                  />
                }
              />
              <Area
                dataKey={dataKey}
                type="monotone"
                stroke={color}
                strokeWidth={2}
                fill={`url(#${gradientId})`}
                dot={false}
                isAnimationActive={false}
              />
            </AreaChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
