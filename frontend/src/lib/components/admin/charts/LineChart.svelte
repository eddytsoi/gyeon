<script lang="ts">
  // Pure-SVG responsive line chart. No external dependencies.
  interface Point { x: string; y: number }
  interface Props {
    data: Point[];
    height?: number;
    yLabel?: string;
    formatY?: (n: number) => string;
  }

  let { data, height = 240, yLabel = '', formatY = (n: number) => String(n) }: Props = $props();

  let containerWidth = $state(600);

  const padding = { top: 12, right: 16, bottom: 28, left: 56 };

  const innerW = $derived(Math.max(1, containerWidth - padding.left - padding.right));
  const innerH = height - padding.top - padding.bottom;

  const minY = $derived(data.length ? Math.min(0, ...data.map((d) => d.y)) : 0);
  const maxY = $derived(data.length ? Math.max(...data.map((d) => d.y), 1) : 1);

  function xPos(i: number) {
    if (data.length <= 1) return padding.left + innerW / 2;
    return padding.left + (i / (data.length - 1)) * innerW;
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

      <!-- Area + line -->
      <path d={area} fill="#111827" fill-opacity="0.06" />
      <path d={path} fill="none" stroke="#111827" stroke-width="2" stroke-linejoin="round" />

      <!-- Points -->
      {#each data as d, i}
        <circle cx={xPos(i)} cy={yPos(d.y)} r={hoverIdx === i ? 4 : 2}
                fill={hoverIdx === i ? '#111827' : '#6b7280'} />
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
