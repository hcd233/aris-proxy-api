import { useState, useCallback } from "react"

export function useChartLegendHighlight() {
  const [activeLegend, setActiveLegend] = useState<string | null>(null)

  const onLegendHover = useCallback((dataKey: string | null) => {
    setActiveLegend(dataKey)
  }, [])

  const getStrokeOpacity = useCallback(
    (dataKey: string) => (activeLegend == null ? 1 : dataKey === activeLegend ? 1 : 0.2),
    [activeLegend]
  )

  return { activeLegend, onLegendHover, getStrokeOpacity }
}
