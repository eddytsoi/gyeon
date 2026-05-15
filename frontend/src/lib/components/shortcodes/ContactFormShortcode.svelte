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

  // Field state is keyed by the field name. Checkbox values are a Set so the
  // submit serialiser can join them with commas before posting.
  let values = $state<Record<string, string>>({});
  let checkboxValues = $state<Record<string, Set<string>>>({});
  let fieldErrors = $state<Record<string, string>>({});
  let formError = $state('');
  let submitted = $state(false);
  let submitting = $state(false);

  // Initialise once we have a form spec: pre-populate defaults so SSR and
  // hydrated state agree.
  $effect(() => {
    if (!form) return;
    const next: Record<string, string> = {};
    const checks: Record<string, Set<string>> = {};
    for (const f of form.fields) {
      if (f.type === 'submit') continue;
      if (f.type === 'checkbox') {
        const set = new Set<string>();
        if (f.default) set.add(f.default);
        checks[f.name] = set;
      } else {
        next[f.name] = f.default ?? '';
      }
    }
    values = next;
    checkboxValues = checks;
  });

  // The markup is the layout template; `fields` drives state + validation.
  // Look up a field by tag name during render.
  const segments = $derived(form ? splitFormMarkup(form.markup) : []);
  const fieldsByName = $derived<Record<string, FormField>>(
    form ? Object.fromEntries(form.fields.map((f) => [f.name, f])) : {}
  );
  const submitField = $derived(form?.fields.find((f) => f.type === 'submit'));

  function inputId(f: FormField): string {
    return f.id || f.name;
  }

  function toggleCheckbox(name: string, value: string, checked: boolean) {
    const set = new Set(checkboxValues[name] ?? []);
    if (checked) set.add(value);
    else set.delete(value);
    checkboxValues = { ...checkboxValues, [name]: set };
  }

  // reCAPTCHA config lives in the public_settings layout payload; we read
  // it reactively so toggling the site setting at runtime takes effect on
  // the next page load.
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

  async function onSubmit(e: SubmitEvent) {
    e.preventDefault();
    if (!form || submitting) return;
    submitting = true;
    formError = '';
    fieldErrors = {};

    // Serialise: checkbox sets → comma-joined string.
    const data: Record<string, string> = { ...values };
    for (const [name, set] of Object.entries(checkboxValues)) {
      data[name] = [...set].join(',');
    }

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
    // Result is FormSubmitError here — narrow explicitly so TS sees the
    // optional `fields` / `error` members.
    const err = result as { error?: string; fields?: Record<string, string> };
    if (err.fields) {
      fieldErrors = err.fields;
    }
    formError = err.error || form.error_message;
  }
</script>

{#snippet fieldInput(f: FormField)}
  {#if f.type === 'hidden'}
    <input type="hidden" name={f.name} value={values[f.name] ?? f.default ?? ''} />
  {:else if f.type === 'textarea'}
    <textarea
      id={inputId(f)}
      name={f.name}
      required={f.required}
      placeholder={f.placeholder ?? ''}
      maxlength={f.maxlength}
      minlength={f.minlength}
      rows={5}
      bind:value={values[f.name]}
      class="block w-full rounded-xl border border-gray-200 bg-white px-3.5 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
    ></textarea>
  {:else if f.type === 'select'}
    <select
      id={inputId(f)}
      name={f.name}
      required={f.required}
      bind:value={values[f.name]}
      class="block w-full rounded-xl border border-gray-200 bg-white px-3.5 py-2.5 text-sm text-gray-900 shadow-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
    >
      <option value="">— select —</option>
      {#each f.options ?? [] as opt (opt.value)}
        <option value={opt.value}>{opt.label}</option>
      {/each}
    </select>
  {:else if f.type === 'radio'}
    <div class="space-y-1.5" role="radiogroup">
      {#each f.options ?? [] as opt (opt.value)}
        <label class="inline-flex items-center gap-2 text-sm text-gray-700">
          <input
            type="radio"
            name={f.name}
            value={opt.value}
            checked={values[f.name] === opt.value}
            onchange={() => (values = { ...values, [f.name]: opt.value })}
            required={f.required}
          />
          <span>{opt.label}</span>
        </label>
      {/each}
    </div>
  {:else if f.type === 'checkbox'}
    <div class="space-y-1.5">
      {#each f.options ?? [] as opt (opt.value)}
        <label class="inline-flex items-center gap-2 text-sm text-gray-700">
          <input
            type="checkbox"
            name={f.name}
            value={opt.value}
            checked={checkboxValues[f.name]?.has(opt.value)}
            onchange={(e) =>
              toggleCheckbox(f.name, opt.value, (e.currentTarget as HTMLInputElement).checked)}
          />
          <span>{opt.label}</span>
        </label>
      {/each}
    </div>
  {:else}
    <input
      id={inputId(f)}
      name={f.name}
      type={f.type}
      required={f.required}
      placeholder={f.placeholder ?? ''}
      maxlength={f.maxlength}
      minlength={f.minlength}
      min={f.min ?? undefined}
      max={f.max ?? undefined}
      bind:value={values[f.name]}
      class="block w-full rounded-xl border border-gray-200 bg-white px-3.5 py-2.5 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
    />
  {/if}
{/snippet}

{#if !form}
  <div class="my-4 rounded-xl border border-dashed border-gray-300 bg-gray-50 px-4 py-3 text-sm text-gray-500 {attrs.class ?? ''}">
    Contact form <span class="font-mono">[contact-form id="{slug || '...'}"]</span> not found.
  </div>
{:else if submitted}
  <div class="my-6 rounded-2xl border border-emerald-100 bg-emerald-50 px-5 py-4 text-sm text-emerald-800 {attrs.class ?? ''}">
    {form.success_message}
  </div>
{:else}
  <form class="my-6 {attrs.class ?? ''}" onsubmit={onSubmit} novalidate>
    {#each segments as seg, i (i)}
      {#if seg.type === 'html'}
        {@html seg.html}
      {:else if seg.type === 'submit'}
        <button
          type="submit"
          disabled={submitting}
          class="inline-flex items-center justify-center rounded-xl bg-gray-900 px-5 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {submitting ? 'Sending…' : (submitField?.label || 'Send')}
        </button>
      {:else}
        {@const f = fieldsByName[seg.name]}
        {#if f}
          {@render fieldInput(f)}
          {#if fieldErrors[f.name]}
            <p class="text-xs text-red-500">{fieldErrors[f.name]}</p>
          {/if}
        {/if}
      {/if}
    {/each}

    {#if formError}
      <div class="mt-4 rounded-xl border border-red-100 bg-red-50 px-4 py-3 text-sm text-red-700">
        {formError}
      </div>
    {/if}
  </form>
{/if}
