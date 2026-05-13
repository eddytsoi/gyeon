<script lang="ts">
  import { renderMarkdown } from '$lib/markdown';
  import { parseShortcodes } from '$lib/shortcodes/parser';
  import { shortcodeRegistry } from '$lib/shortcodes/registry';
  import { EMPTY_REFS, type ShortcodeRefs } from '$lib/shortcodes/types';

  let { content, refs = EMPTY_REFS, placeholderMode = false }: {
    content: string | undefined | null;
    refs?: ShortcodeRefs;
    // When true, shortcodes that depend on server-resolved refs (product /
    // products) render as a visible chip showing their source — used in
    // admin preview where the live data isn't fetched.
    placeholderMode?: boolean;
  } = $props();

  const chunks = $derived(parseShortcodes(content));

  function needsRefs(name: string): boolean {
    return name === 'product' || name === 'products';
  }
</script>

{#each chunks as chunk, i (i)}
  {#if chunk.type === 'md'}
    {@html renderMarkdown(chunk.text)}
  {:else if placeholderMode && needsRefs(chunk.name)}
    <code class="inline-block my-1 px-2 py-1 rounded bg-gray-100 text-gray-600 text-xs font-mono break-all">{chunk.raw}</code>
  {:else}
    {@const Cmp = shortcodeRegistry[chunk.name]}
    {#if Cmp}
      <Cmp attrs={chunk.attrs} body={chunk.body} {refs} />
    {/if}
  {/if}
{/each}
