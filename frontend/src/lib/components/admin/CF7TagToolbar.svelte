<script lang="ts">
  import * as m from '$lib/paraglide/messages';

  // Two-way binding to the textarea's value and a reference to the element
  // itself so we can insert at the cursor and re-focus afterwards.
  let { value = $bindable(''), textarea }: {
    value: string;
    textarea: HTMLTextAreaElement | null;
  } = $props();

  const tags: { name: string; label: () => string; snippet: string }[] = [
    { name: 'text',     label: m.admin_forms_tag_text,     snippet: '[text* your-name placeholder "Your name"]' },
    { name: 'email',    label: m.admin_forms_tag_email,    snippet: '[email* your-email placeholder "you@example.com"]' },
    { name: 'tel',      label: m.admin_forms_tag_tel,      snippet: '[tel your-phone placeholder "+1 555 0100"]' },
    { name: 'textarea', label: m.admin_forms_tag_textarea, snippet: '[textarea* your-message placeholder "Your message"]' },
    { name: 'select',   label: m.admin_forms_tag_select,   snippet: '[select your-choice "Option 1" "Option 2|value-2"]' },
    { name: 'checkbox', label: m.admin_forms_tag_checkbox, snippet: '[checkbox your-options "Yes|yes" "No|no"]' },
    { name: 'radio',    label: m.admin_forms_tag_radio,    snippet: '[radio your-choice "Option A|a" "Option B|b"]' },
    { name: 'date',     label: m.admin_forms_tag_date,     snippet: '[date your-date]' },
    { name: 'submit',   label: m.admin_forms_tag_submit,   snippet: '[submit "Send message"]' }
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
    {m.admin_forms_insert_tag_label()}
  </span>
  {#each tags as t (t.name)}
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
