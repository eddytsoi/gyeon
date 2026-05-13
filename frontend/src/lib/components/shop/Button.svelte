<script lang="ts">
  let { href, label, style = 'primary' }: {
    href: string;
    label: string;
    style?: 'primary' | 'secondary';
  } = $props();

  const isExternal = $derived(/^https?:\/\//i.test(href));
</script>

<a
  {href}
  target={isExternal ? '_blank' : undefined}
  rel={isExternal ? 'noopener noreferrer' : undefined}
  class="inline-flex items-center justify-center px-6 py-3 rounded-xl font-display text-sm font-semibold
         uppercase tracking-[0.12em] transition-colors
         {style === 'primary'
           ? 'bg-navy-500 text-white hover:bg-navy-600'
           : 'border border-ink-300 text-ink-900 hover:bg-paper'}"
>
  {label}
</a>
