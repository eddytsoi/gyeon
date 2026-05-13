<script lang="ts">
  import { renderMarkdown } from '$lib/markdown';
  import type { ShortcodeAttrs } from '$lib/shortcodes/types';

  let { attrs, body }: { attrs: ShortcodeAttrs; body: string } = $props();

  const type = $derived(
    attrs.type === 'warn' || attrs.type === 'success' ? attrs.type : 'info'
  );

  const tone = $derived(
    type === 'warn'
      ? 'border-amber-300 bg-amber-50 text-amber-900'
      : type === 'success'
        ? 'border-emerald-300 bg-emerald-50 text-emerald-900'
        : 'border-sky-300 bg-sky-50 text-sky-900'
  );
</script>

<div class="my-6 rounded-xl border-l-4 px-5 py-4 prose prose-sm max-w-none {tone}">
  {@html renderMarkdown(body)}
</div>
