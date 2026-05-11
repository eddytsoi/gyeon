<script lang="ts">
  /*
   * Bilingual section head — English eyebrow + Chinese H2 + 12px navy underline.
   * The underline is the visual signature added on top of the gyeon.hk vocabulary.
   * Pass `align="center"` for hero-style centred heads.
   */
  import Eyebrow from './Eyebrow.svelte';

  let {
    eyebrow,
    title,
    align = 'left',
    tone = 'light',
    class: extra = ''
  }: {
    eyebrow?: string;
    title: string;
    align?: 'left' | 'center';
    tone?: 'light' | 'dark';
    class?: string;
  } = $props();

  const alignClass = $derived(align === 'center' ? 'text-center' : 'text-left');
  const underlineWrap = $derived(align === 'center' ? 'mx-auto' : '');
  const titleColor = $derived(tone === 'dark' ? 'text-white' : 'text-ink-900');
</script>

<header class="mb-8 md:mb-12 {alignClass} {extra}">
  {#if eyebrow}
    <Eyebrow tone={tone === 'dark' ? 'invert' : 'navy'} class="mb-2">{eyebrow}</Eyebrow>
  {/if}
  <h2 class="font-display text-3xl md:text-4xl font-bold tracking-tight {titleColor}">
    {title}
  </h2>
  <div class="mt-4 h-px w-12 bg-navy-500 {underlineWrap}"></div>
</header>
