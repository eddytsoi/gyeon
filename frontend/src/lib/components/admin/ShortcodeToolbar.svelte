<script lang="ts">
  import * as m from '$lib/paraglide/messages';

  // Two-way binding to the textarea's value and a reference to the element
  // itself so we can insert at the cursor and re-focus afterwards.
  let { value = $bindable(''), textarea }: {
    value: string;
    textarea: HTMLTextAreaElement | null;
  } = $props();

  const templates: { name: string; label: () => string; snippet: string }[] = [
    { name: 'product', label: m.admin_shortcode_insert_product, snippet: '[product id="PRD-1"]' },
    { name: 'products', label: m.admin_shortcode_insert_products, snippet: '[products ids="PRD-1,PRD-2,PRD-3"]' },
    { name: 'products-cat', label: m.admin_shortcode_insert_products_categories, snippet: '[products categories="category-slug-1,category-slug-2"]' },
    { name: 'button', label: m.admin_shortcode_insert_button, snippet: '[button href="/shop" label="Shop now" style="primary"]' },
    { name: 'note', label: m.admin_shortcode_insert_note, snippet: '[note type="info"]Your message here.[/note]' }
  ];

  function insert(snippet: string) {
    if (!textarea) {
      value = (value ?? '') + (value && !value.endsWith('\n') ? '\n' : '') + snippet;
      return;
    }
    const start = textarea.selectionStart ?? value.length;
    const end = textarea.selectionEnd ?? value.length;
    const before = value.slice(0, start);
    const after = value.slice(end);
    const needsLeadingNewline = before.length > 0 && !before.endsWith('\n');
    const needsTrailingNewline = after.length > 0 && !after.startsWith('\n');
    const insertion =
      (needsLeadingNewline ? '\n' : '') + snippet + (needsTrailingNewline ? '\n' : '');
    value = before + insertion + after;
    // Restore focus and place the caret right after the inserted snippet.
    queueMicrotask(() => {
      if (!textarea) return;
      const caret = before.length + insertion.length;
      textarea.focus();
      textarea.setSelectionRange(caret, caret);
    });
  }
</script>

<div class="flex flex-wrap items-center gap-1.5 mb-1.5">
  <span class="text-[11px] uppercase tracking-wide text-gray-400 mr-1">
    {m.admin_shortcode_insert_label()}
  </span>
  {#each templates as t (t.name)}
    <button
      type="button"
      onclick={() => insert(t.snippet)}
      title={t.snippet}
      class="px-2.5 py-1 rounded-lg border border-gray-200 text-xs font-medium text-gray-700
             bg-white hover:bg-gray-50 hover:border-gray-300 transition-colors"
    >
      {t.label()}
    </button>
  {/each}
</div>
