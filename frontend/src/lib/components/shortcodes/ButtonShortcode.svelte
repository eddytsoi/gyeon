<script lang="ts">
  import Button from '$lib/components/shop/Button.svelte';
  import type { ShortcodeAttrs } from '$lib/shortcodes/types';

  let { attrs }: { attrs: ShortcodeAttrs } = $props();

  function warnIfBad(key: string, raw: string | undefined, resolved: unknown) {
    if (import.meta.env.DEV && raw !== undefined && String(resolved) !== raw) {
      // eslint-disable-next-line no-console
      console.warn(`[button] invalid ${key}="${raw}", falling back to "${resolved}"`);
    }
  }

  const HEX_RE = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/;

  const style = $derived(attrs.style === 'secondary' ? 'secondary' : 'primary');
  const rounded = $derived(
    attrs.rounded === 'sm' || attrs.rounded === 'md' || attrs.rounded === 'xl'
      ? attrs.rounded
      : 'xl'
  );
  const size = $derived.by(() => {
    const n = Number(attrs.size);
    return Number.isFinite(n) && n >= 8 && n <= 96 ? n : 14;
  });
  const fontWeight = $derived.by(() => {
    const n = Number(attrs['font-weight']);
    return Number.isFinite(n) && n >= 100 && n <= 900 ? n : 600;
  });
  const color = $derived(
    typeof attrs.color === 'string' && HEX_RE.test(attrs.color) ? attrs.color : undefined
  );

  $effect(() => {
    warnIfBad('rounded', attrs.rounded, rounded);
    warnIfBad('size', attrs.size, size);
    warnIfBad('font-weight', attrs['font-weight'], fontWeight);
    if (attrs.color !== undefined && color === undefined) {
      // eslint-disable-next-line no-console
      console.warn(`[button] invalid color="${attrs.color}" — must be #RGB or #RRGGBB hex`);
    }
  });
</script>

{#if attrs.href && attrs.label}
  <div class="my-6 {attrs.class ?? ''}">
    <Button
      href={attrs.href}
      label={attrs.label}
      {style}
      {rounded}
      {size}
      {fontWeight}
      {color}
    />
  </div>
{/if}
