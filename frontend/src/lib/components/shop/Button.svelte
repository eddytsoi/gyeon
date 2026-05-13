<script lang="ts">
  let {
    href,
    label,
    style = 'primary',
    rounded = 'xl',
    size = 14,
    fontWeight = 600,
    color
  }: {
    href: string;
    label: string;
    style?: 'primary' | 'secondary';
    rounded?: 'sm' | 'md' | 'xl';
    size?: number;
    fontWeight?: number;
    color?: string;
  } = $props();

  const isExternal = $derived(/^https?:\/\//i.test(href));

  // Static map so Tailwind JIT picks up the full class names.
  const ROUNDED_CLASS = { sm: 'rounded-sm', md: 'rounded-md', xl: 'rounded-xl' };
  const roundedClass = $derived(ROUNDED_CLASS[rounded] ?? 'rounded-xl');

  // Inline style overrides Tailwind defaults for arbitrary numerics/hex.
  // `color` is opt-in — when unset the primary/secondary class wins.
  const styleAttr = $derived(
    [
      `font-size:${size}px`,
      `font-weight:${fontWeight}`,
      color ? `color:${color}` : ''
    ]
      .filter(Boolean)
      .join(';')
  );
</script>

<a
  {href}
  target={isExternal ? '_blank' : undefined}
  rel={isExternal ? 'noopener noreferrer' : undefined}
  style={styleAttr}
  class="inline-flex items-center justify-center px-6 py-3 {roundedClass} font-display
         uppercase tracking-[0.12em] transition-colors
         {style === 'primary'
           ? 'bg-navy-500 text-white hover:bg-navy-600'
           : 'border border-ink-300 text-ink-900 hover:bg-paper'}"
>
  {label}
</a>
