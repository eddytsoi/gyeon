<script lang="ts">
  import { page } from '$app/state';
  import type { FormField, ShortcodeAttrs, ShortcodeRefs } from '$lib/shortcodes/types';
  import { EMPTY_REFS } from '$lib/shortcodes/types';
  import { splitFormMarkup } from '$lib/shortcodes/formMarkup';
  import { getRecaptchaToken } from '$lib/recaptcha';
  import { submitForm } from '$lib/api';

  let {
    attrs,
    refs = EMPTY_REFS
  }: { attrs: ShortcodeAttrs; refs?: ShortcodeRefs } = $props();

  const slug = $derived((attrs.id ?? '').trim());
  const form = $derived(slug ? refs.forms[slug] : undefined);

  let fieldErrors = $state<Record<string, string>>({});
  let formError = $state('');
  let submitted = $state(false);
  let submitting = $state(false);
  let formEl = $state<HTMLFormElement | null>(null);

  const segments = $derived(form ? splitFormMarkup(form.markup) : []);
  const fieldsByName = $derived<Record<string, FormField>>(
    form ? Object.fromEntries(form.fields.map((f) => [f.name, f])) : {}
  );
  const submitField = $derived(form?.fields.find((f) => f.type === 'submit'));
  const submitLabel = $derived(submitField?.label || 'Send');

  // Render the entire form body as one HTML string. Building a single string
  // is required because adjacent {@html} blocks each get parsed in isolation
  // — when chunks split a `<table>`, the browser's HTML parser auto-closes
  // each fragment and foster-parents the substituted inputs out of the
  // table. Concatenating first means the browser parses the whole table
  // shape coherently.
  const formHtml = $derived(form ? buildFormHtml(segments, fieldsByName, submitLabel) : '');

  // reCAPTCHA config from the public_settings layout payload.
  const recaptchaSiteKey = $derived(
    (page.data as { publicSettings?: { key: string; value: string }[] } | undefined)?.publicSettings?.find(
      (s) => s.key === 'recaptcha_site_key'
    )?.value ?? ''
  );
  const recaptchaEnabled = $derived(
    (page.data as { publicSettings?: { key: string; value: string }[] } | undefined)?.publicSettings?.find(
      (s) => s.key === 'recaptcha_enabled'
    )?.value === 'true'
  );

  // Reflect submit-button state into the DOM (the button itself lives inside
  // the {@html} block so Svelte can't bind directly).
  $effect(() => {
    if (!formEl) return;
    const btn = formEl.querySelector<HTMLButtonElement>('button[data-cf-submit]');
    if (!btn) return;
    btn.disabled = submitting;
    btn.textContent = submitting ? 'Sending…' : submitLabel;
  });

  // Reflect per-field errors into the inline <p data-cf-error="..."> slots.
  $effect(() => {
    if (!formEl) return;
    formEl.querySelectorAll<HTMLElement>('[data-cf-error]').forEach((el) => {
      const name = el.getAttribute('data-cf-error') ?? '';
      const msg = fieldErrors[name];
      el.textContent = msg ?? '';
      el.classList.toggle('hidden', !msg);
    });
  });

  async function onSubmit(e: SubmitEvent) {
    e.preventDefault();
    if (!form || submitting) return;
    submitting = true;
    formError = '';
    fieldErrors = {};

    // Read native form state. FormData groups same-name entries (checkbox
    // groups, multi-selects) into multiple values, which we join with commas
    // to match the API's flat-map contract.
    const fd = new FormData(e.target as HTMLFormElement);
    const grouped: Record<string, string[]> = {};
    for (const [k, v] of fd.entries()) {
      (grouped[k] ??= []).push(typeof v === 'string' ? v : '');
    }
    const data: Record<string, string> = {};
    for (const [k, vs] of Object.entries(grouped)) data[k] = vs.join(',');

    let token = '';
    if (recaptchaEnabled && recaptchaSiteKey) {
      token = await getRecaptchaToken(recaptchaSiteKey, form.recaptcha_action || 'contact_form');
    }

    const result = await submitForm(form.slug, data, token);
    submitting = false;

    if ('ok' in result && result.ok) {
      submitted = true;
      return;
    }
    const err = result as { error?: string; fields?: Record<string, string> };
    if (err.fields) fieldErrors = err.fields;
    formError = err.error || form.error_message;
  }

  function buildFormHtml(
    segs: ReturnType<typeof splitFormMarkup>,
    byName: Record<string, FormField>,
    submitLbl: string
  ): string {
    const parts: string[] = [];
    for (const seg of segs) {
      if (seg.type === 'html') {
        parts.push(seg.html);
      } else if (seg.type === 'submit') {
        parts.push(renderSubmit(submitLbl));
      } else {
        const f = byName[seg.name];
        if (!f) continue;
        parts.push(renderField(f));
        if (f.type !== 'hidden' && f.type !== 'submit') {
          parts.push(`<p class="text-xs text-red-500 hidden" data-cf-error="${esc(f.name)}"></p>`);
        }
      }
    }
    return parts.join('');
  }

  function renderField(f: FormField): string {
    const id = f.id || f.name;
    const inputCls =
      'block w-full rounded-xl border border-gray-200 bg-white px-3.5 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900';
    const selectCls =
      'block w-full rounded-xl border border-gray-200 bg-white px-3.5 py-2.5 text-sm text-gray-900 shadow-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900';
    const opts = f.options ?? [];
    const def = f.default ?? '';

    if (f.type === 'hidden') {
      return `<input type="hidden" name="${esc(f.name)}" value="${esc(def)}">`;
    }
    if (f.type === 'textarea') {
      return (
        `<textarea id="${esc(id)}" name="${esc(f.name)}"${attr(' required', f.required)}` +
        attr(' placeholder', f.placeholder) +
        attr(' maxlength', f.maxlength) +
        attr(' minlength', f.minlength) +
        ` rows="5" class="${inputCls}">${esc(def)}</textarea>`
      );
    }
    if (f.type === 'select') {
      const optsHtml = opts
        .map(
          (o) =>
            `<option value="${esc(o.value)}"${o.value === def ? ' selected' : ''}>${esc(o.label)}</option>`
        )
        .join('');
      return (
        `<select id="${esc(id)}" name="${esc(f.name)}"${attr(' required', f.required)} class="${selectCls}">` +
        `<option value="">— select —</option>${optsHtml}</select>`
      );
    }
    if (f.type === 'radio') {
      const items = opts
        .map(
          (o) =>
            `<label class="inline-flex items-center gap-2 text-sm text-gray-700"><input type="radio" name="${esc(f.name)}" value="${esc(o.value)}"${o.value === def ? ' checked' : ''}${attr(' required', f.required)}><span>${esc(o.label)}</span></label>`
        )
        .join('');
      return `<div class="space-y-1.5" role="radiogroup">${items}</div>`;
    }
    if (f.type === 'checkbox') {
      const items = opts
        .map(
          (o) =>
            `<label class="inline-flex items-center gap-2 text-sm text-gray-700"><input type="checkbox" name="${esc(f.name)}" value="${esc(o.value)}"${o.value === def ? ' checked' : ''}><span>${esc(o.label)}</span></label>`
        )
        .join('');
      return `<div class="space-y-1.5">${items}</div>`;
    }
    // text / email / tel / date
    return (
      `<input id="${esc(id)}" name="${esc(f.name)}" type="${esc(f.type)}"` +
      attr(' required', f.required) +
      attr(' placeholder', f.placeholder) +
      attr(' value', def) +
      attr(' maxlength', f.maxlength) +
      attr(' minlength', f.minlength) +
      attr(' min', f.min) +
      attr(' max', f.max) +
      ` class="${inputCls}">`
    );
  }

  function renderSubmit(label: string): string {
    return (
      `<button type="submit" data-cf-submit ` +
      `class="inline-flex items-center justify-center rounded-xl bg-gray-900 px-5 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50">` +
      `${esc(label)}</button>`
    );
  }

  function esc(s: string | number | undefined | null): string {
    if (s === undefined || s === null || s === '') return '';
    return String(s).replace(/[&<>"']/g, (c) => {
      switch (c) {
        case '&': return '&amp;';
        case '<': return '&lt;';
        case '>': return '&gt;';
        case '"': return '&quot;';
        default: return '&#39;';
      }
    });
  }

  // Emit ` key="value"` only when value is truthy/non-empty.
  function attr(prefix: string, value: string | number | boolean | undefined | null): string {
    if (value === undefined || value === null || value === false || value === '' || value === 0) return '';
    if (value === true) return prefix;
    return `${prefix}="${esc(value)}"`;
  }
</script>

{#if !form}
  <div class="my-4 rounded-xl border border-dashed border-gray-300 bg-gray-50 px-4 py-3 text-sm text-gray-500 {attrs.class ?? ''}">
    Contact form <span class="font-mono">[contact-form id="{slug || '...'}"]</span> not found.
  </div>
{:else if submitted}
  <div class="my-6 rounded-2xl border border-emerald-100 bg-emerald-50 px-5 py-4 text-sm text-emerald-800 {attrs.class ?? ''}">
    {form.success_message}
  </div>
{:else}
  <form bind:this={formEl} class="my-6 {attrs.class ?? ''}" onsubmit={onSubmit} novalidate>
    {@html formHtml}
    {#if formError}
      <div class="mt-4 rounded-xl border border-red-100 bg-red-50 px-4 py-3 text-sm text-red-700">
        {formError}
      </div>
    {/if}
  </form>
{/if}
