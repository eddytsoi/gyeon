<script lang="ts">
  // Pure-SVG responsive line chart. No external dependencies. Optionally overlays
  // a second muted/dashed "compare" series (e.g. last month vs this month),
  // sharing a common y-domain so the two are visually comparable.
  interface Point { x: string; y: number }
  interface Props {
    data: Point[];
    height?: number;
    yLabel?: string;
    formatY?: (n: number) => string;
    compare?: Point[];
    label?: string;
    compareLabel?: string;
    color?: string;
    compareColor?: string;
  }

  let {
    data,
    height = 240,
    yLabel = '',
    formatY = (n: number) => String(n),
    compare = [],
    label = '',
    compareLabel = '',
    color = '#111827',
    compareColor = '#9ca3af'
  }: Props = $props();

  let containerWidth = $state(600);

  const padding = { top: 12, right: 16, bottom: 28, left: 56 };

  const innerW = $derived(Math.max(1, containerWidth - padding.left - padding.right));
  const innerH = height - padding.top - padding.bottom;

  // y-domain spans both series so they share a scale.
  const allY = $derived([...data.map((d) => d.y), ...compare.map((d) => d.y)]);
  const minY = $derived(allY.length ? Math.min(0, ...allY) : 0);
  const maxY = $derived(allY.length ? Math.max(...allY, 1) : 1);

  // x is indexed per-series so two months of differing length still span the
  // full width (the hero aligns them by day-of-month before passing them in).
  function xPosN(i: number, n: number) {
    if (n <= 1) return padding.left + innerW / 2;
    return padding.left + (i / (n - 1)) * innerW;
  }
  function xPos(i: number) {
    return xPosN(i, data.length);
  }
  function yPos(y: number) {
    if (maxY === minY) return padding.top + innerH / 2;
    return padding.top + innerH - ((y - minY) / (maxY - minY)) * innerH;
  }

  const path = $derived(
    data.length === 0
      ? ''
      : data.map((d, i) => `${i === 0 ? 'M' : 'L'} ${xPos(i)},${yPos(d.y)}`).join(' ')
  );
  const area = $derived(
    data.length === 0
      ? ''
      : `${path} L ${xPos(data.length - 1)},${yPos(0)} L ${xPos(0)},${yPos(0)} Z`
  );
  const comparePath = $derived(
    compare.length === 0
      ? ''
      : compare.map((d, i) => `${i === 0 ? 'M' : 'L'} ${xPosN(i, compare.length)},${yPos(d.y)}`).join(' ')
  );

  // Y-axis ticks (5 lines)
  const yTicks = $derived(
    Array.from({ length: 5 }, (_, i) => minY + ((maxY - minY) * i) / 4)
  );

  // X-axis ticks: first / middle / last when many points; all when few
  const xTickIdx = $derived(
    data.length <= 6
      ? data.map((_, i) => i)
      : [0, Math.floor(data.length / 2), data.length - 1]
  );

  // Tooltip state
  let hoverIdx = $state<number | null>(null);
  function onMove(e: MouseEvent) {
    const rect = (e.currentTarget as SVGSVGElement).getBoundingClientRect();
    const px = e.clientX - rect.left;
    if (data.length === 0) return;
    let best = 0;
    let bestDist = Infinity;
    for (let i = 0; i < data.length; i++) {
      const dist = Math.abs(xPos(i) - px);
      if (dist < bestDist) { best = i; bestDist = dist; }
    }
    hoverIdx = best;
  }
  function onLeave() { hoverIdx = null; }
</script>

<div bind:clientWidth={containerWidth} class="relative">
  {#if compareLabel}
    <div class="absolute right-1 top-0 z-10 flex items-center gap-3 text-[10px] text-gray-500">
      <span class="flex items-center gap-1"><span class="inline-block w-3 h-0.5 rounded" style="background:{color}"></span>{label}</span>
      <span class="flex items-center gap-1"><span class="inline-block w-3 border-t-2 border-dashed" style="border-color:{compareColor}"></span>{compareLabel}</span>
    </div>
  {/if}
  {#if data.length === 0}
    <div class="flex items-center justify-center text-xs text-gray-400" style="height: {height}px;">
      No data for this range
    </div>
  {:else}
    <svg width={containerWidth} {height}
         onmousemove={onMove} onmouseleave={onLeave} role="img" aria-label={yLabel}>
      <!-- Y gridlines + labels -->
      {#each yTicks as y, i}
        <line x1={padding.left} x2={padding.left + innerW} y1={yPos(y)} y2={yPos(y)}
              stroke="#f3f4f6" stroke-width="1" />
        <text x={padding.left - 8} y={yPos(y) + 3} text-anchor="end"
              fill="#9ca3af" font-size="10" font-family="ui-monospace, monospace">{formatY(y)}</text>
      {/each}

      <!-- Compare series (muted, dashed, no area / points) -->
      {#if comparePath}
        <path d={comparePath} fill="none" stroke={compareColor} stroke-width="2"
              stroke-dasharray="4,3" stroke-linejoin="round" />
      {/if}

      <!-- Area + line -->
      <path d={area} fill={color} fill-opacity="0.06" />
      <path d={path} fill="none" stroke={color} stroke-width="2" stroke-linejoin="round" />

      <!-- Points -->
      {#each data as d, i}
        <circle cx={xPos(i)} cy={yPos(d.y)} r={hoverIdx === i ? 4 : 2}
                fill={hoverIdx === i ? color : '#9ca3af'} />
      {/each}

      <!-- X labels -->
      {#each xTickIdx as i}
        <text x={xPos(i)} y={height - 8} text-anchor="middle"
              fill="#9ca3af" font-size="10">{data[i].x}</text>
      {/each}

      <!-- Hover guide -->
      {#if hoverIdx != null}
        <line x1={xPos(hoverIdx)} x2={xPos(hoverIdx)} y1={padding.top} y2={padding.top + innerH}
              stroke="#9ca3af" stroke-width="1" stroke-dasharray="3,3" />
      {/if}
    </svg>

    <!-- Tooltip -->
    {#if hoverIdx != null}
      <div class="absolute pointer-events-none px-2 py-1 rounded-lg bg-gray-900 text-white text-xs shadow"
           style="left: {xPos(hoverIdx) + 8}px; top: {yPos(data[hoverIdx].y) - 30}px; white-space: nowrap;">
        <div class="font-mono">{formatY(data[hoverIdx].y)}</div>
        <div class="text-gray-400 text-[10px]">{data[hoverIdx].x}</div>
      </div>
    {/if}
  {/if}
</div>
