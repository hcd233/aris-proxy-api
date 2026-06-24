"use client";

import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  XAxis,
  YAxis,
} from "recharts";

import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart";
import { Card, CardContent } from "@/components/ui/card";

interface RuntimeLineChartProps {
  data: { time: string; value: number }[];
  dataKey: string;
  label: string;
  color?: string;
  unit?: string;
}

export function RuntimeLineChart({
  data,
  dataKey,
  label,
  color = "#D97757",
  unit,
}: RuntimeLineChartProps) {
  const config: ChartConfig = {
    [dataKey]: { label, color },
  };

  return (
    <Card>
      <CardContent className="p-4">
        <span className="mb-3 block text-sm font-medium">{label}</span>
        <ChartContainer config={config} className="h-[160px] w-full">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data} margin={{ left: 8, right: 8, top: 8 }}>
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
                strokeWidth={2}
                dot={false}
                isAnimationActive={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
