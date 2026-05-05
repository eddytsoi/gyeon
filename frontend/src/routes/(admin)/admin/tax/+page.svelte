<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import { showResult } from '$lib/stores/notifications.svelte';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  function val(key: string): string {
    return data.settings.find((s) => s.key === key)?.value ?? '';
  }

  // Stored as decimal (0.05); presented as percent (5.0).
  const initialRatePct = (() => {
    const raw = val('tax_rate');
    if (!raw) return 0;
    const n = Number(raw);
    return Number.isFinite(n) ? n * 100 : 0;
  })();

  let enabled = $state(val('tax_enabled') === 'true');
  let inclusive = $state(val('tax_inclusive') === 'true');
  let ratePct = $state<number>(initialRatePct);
  let label = $state(val('tax_label') || 'Sales Tax');
  let saving = $state(false);

  // Live preview: subtotal HK$100
  const PREVIEW_SUBTOTAL = 100;
  const previewTax = $derived(() => {
    if (!enabled) return 0;
    const r = (Number(ratePct) || 0) / 100;
    if (inclusive) {
      // tax already embedded — back-calculate
      return PREVIEW_SUBTOTAL - PREVIEW_SUBTOTAL / (1 + r);
    }
    return PREVIEW_SUBTOTAL * r;
  });
  const previewTotal = $derived(() =>
    inclusive ? PREVIEW_SUBTOTAL : PREVIEW_SUBTOTAL + previewTax()
  );
  const previewSubtotal = $derived(() =>
    inclusive ? PREVIEW_SUBTOTAL - previewTax() : PREVIEW_SUBTOTAL
  );

  function fmt(n: number): string {
    return `HK$${n.toFixed(2)}`;
  }
</script>

<div class="max-w-2xl mx-auto space-y-6">
  <div class="flex items-center gap-4">
    <h2 class="text-xl font-bold text-gray-900">{m.admin_tax_heading()}</h2>
  </div>
  <p class="text-sm text-gray-500 -mt-3">{m.admin_tax_subtitle()}</p>

  <form method="POST" action="?/save" class="space-y-6"
        use:enhance={() => {
          if (saving) return;
          saving = true;
          return async ({ result, update }) => {
            showResult(result, m.admin_tax_save_success(), m.admin_tax_save_failure());
            await update();
            saving = false;
          };
        }}>
    <!-- Enable toggle -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
      <label class="flex items-center justify-between cursor-pointer select-none">
        <div>
          <p class="text-sm font-semibold text-gray-900">{m.admin_tax_enable_label()}</p>
          <p class="text-xs text-gray-400 mt-0.5">{m.admin_tax_enable_hint()}</p>
        </div>
        <div class="relative">
          <input type="checkbox" class="sr-only peer" bind:checked={enabled} />
          <input type="hidden" name="tax_enabled" value={enabled ? 'true' : 'false'} />
          <div class="w-10 h-6 bg-gray-200 peer-checked:bg-gray-900 rounded-full transition-colors"></div>
          <div class="absolute top-1 left-1 w-4 h-4 bg-white rounded-full shadow
                      transition-transform peer-checked:translate-x-4"></div>
        </div>
      </label>
    </div>

    <!-- Rate + Label -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5 space-y-5"
         class:opacity-50={!enabled}>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label for="tax_rate_pct" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_tax_rate_label()}</label>
          <div class="relative">
            <input id="tax_rate_pct" name="tax_rate_pct" type="number" step="0.01" min="0" max="100"
                   bind:value={ratePct} disabled={!enabled}
                   class="w-full px-3.5 py-2.5 pr-9 rounded-xl border border-gray-200 text-sm font-mono
                          focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent
                          disabled:bg-gray-50" />
            <span class="absolute inset-y-0 right-3 flex items-center text-sm text-gray-400 pointer-events-none">%</span>
          </div>
          <p class="mt-1.5 text-xs text-gray-400">{m.admin_tax_rate_hint()}</p>
        </div>
        <div>
          <label for="tax_label_input" class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_tax_label_label()}</label>
          <input id="tax_label_input" name="tax_label" type="text" bind:value={label} disabled={!enabled}
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent
                        disabled:bg-gray-50" />
          <p class="mt-1.5 text-xs text-gray-400">{m.admin_tax_label_hint()}</p>
        </div>
      </div>

      <!-- Pricing model -->
      <div>
        <p class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-2">{m.admin_tax_pricing_model_label()}</p>
        <input type="hidden" name="tax_inclusive" value={inclusive ? 'true' : 'false'} />
        <div class="space-y-2">
          <label class="flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-colors
                        {!inclusive ? 'border-gray-900 bg-gray-50' : 'border-gray-200 hover:bg-gray-50'}">
            <input type="radio" class="mt-0.5" checked={!inclusive} onclick={() => inclusive = false} disabled={!enabled} />
            <div>
              <p class="text-sm font-medium text-gray-900">{m.admin_tax_pricing_exclusive_title()}</p>
              <p class="text-xs text-gray-500 mt-0.5">{m.admin_tax_pricing_exclusive_hint()}</p>
            </div>
          </label>
          <label class="flex items-start gap-3 p-3 rounded-xl border cursor-pointer transition-colors
                        {inclusive ? 'border-gray-900 bg-gray-50' : 'border-gray-200 hover:bg-gray-50'}">
            <input type="radio" class="mt-0.5" checked={inclusive} onclick={() => inclusive = true} disabled={!enabled} />
            <div>
              <p class="text-sm font-medium text-gray-900">{m.admin_tax_pricing_inclusive_title()}</p>
              <p class="text-xs text-gray-500 mt-0.5">{m.admin_tax_pricing_inclusive_hint()}</p>
            </div>
          </label>
        </div>
      </div>
    </div>

    <!-- Preview -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5">
      <p class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">{m.admin_tax_preview_heading()}</p>
      <div class="space-y-1.5 font-mono text-sm">
        <div class="flex justify-between text-gray-600">
          <span>{m.admin_tax_preview_subtotal()}</span>
          <span>{fmt(previewSubtotal())}</span>
        </div>
        <div class="flex justify-between text-gray-600">
          <span>{label || 'Tax'} ({ratePct.toFixed(2)}%)</span>
          <span>{fmt(previewTax())}</span>
        </div>
        <div class="flex justify-between pt-1.5 border-t border-gray-100 text-gray-900 font-semibold">
          <span>{m.admin_tax_preview_total()}</span>
          <span>{fmt(previewTotal())}</span>
        </div>
      </div>
    </div>

    <!-- Submit -->
    <div class="flex justify-end gap-3">
      <SaveButton loading={saving}
              class="inline-flex items-center justify-center gap-1.5 px-5 py-2.5 rounded-xl bg-gray-900
                     text-white text-sm font-medium hover:bg-gray-700 transition-colors disabled:opacity-50">
        {m.common_save_changes()}
      </SaveButton>
    </div>
  </form>
</div>
