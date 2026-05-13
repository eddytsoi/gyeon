<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import { showResult } from '$lib/stores/notifications.svelte';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  const c = data.campaign;
  const isNew = !c;
  let saving = $state(false);

  let name = $state(c?.name ?? '');
  let description = $state(c?.description ?? '');
  let discountType = $state<'percentage' | 'fixed'>(c?.discount_type ?? 'percentage');
  let discountValue = $state<number>(c?.discount_value ?? 10);
  let targetType = $state<'all' | 'category' | 'product'>(c?.target_type ?? 'all');
  let targetID = $state(c?.target_id ?? '');
  let minOrder = $state<string>(c?.min_order_amount != null ? String(c.min_order_amount) : '');
  let isActive = $state<boolean>(c?.is_active ?? true);

  function toLocalInput(s?: string): string {
    if (!s) return '';
    const d = new Date(s);
    if (Number.isNaN(d.getTime())) return '';
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
  }

  let startsAt = $state(toLocalInput(c?.starts_at));
  let endsAt = $state(toLocalInput(c?.ends_at));
</script>

<svelte:head>
  <title>{isNew ? m.admin_discounts_campaign_new_title() : m.admin_discounts_campaign_edit_title({ name: c?.name ?? '' })}</title>
</svelte:head>

<div class="max-w-3xl mx-auto space-y-6">
  <div class="flex items-center gap-4">
    <a href="/admin/discounts"
       class="p-2 rounded-xl text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
      <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5"/>
      </svg>
    </a>
    <h2 class="text-xl font-bold text-gray-900">{isNew ? m.admin_discounts_campaign_new_heading() : m.admin_discounts_campaign_edit_heading()}</h2>
  </div>

  <form method="POST" action="?/save" class="space-y-6"
        use:enhance={() => {
          if (saving) return;
          saving = true;
          const cName = name;
          return async ({ result, update }) => {
            showResult(result,
              isNew ? m.admin_discounts_campaign_create_success({ name: cName }) : m.admin_discounts_campaign_save_success({ name: cName }),
              isNew ? m.admin_discounts_campaign_create_failure({ name: cName }) : m.admin_discounts_campaign_save_failure({ name: cName }));
            await update();
            saving = false;
          };
        }}>
    <!-- Basics -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5 space-y-5">
      <div>
        <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_discounts_label_name()}</label>
        <input type="text" name="name" bind:value={name} required
               class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                      focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
      </div>
      <div>
        <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
          {m.admin_discounts_label_description()} <span class="normal-case font-normal text-gray-400">{m.common_optional()}</span>
        </label>
        <textarea name="description" bind:value={description} rows="2"
                  class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm resize-none
                         focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent"></textarea>
      </div>
    </div>

    <!-- Discount -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5 space-y-5">
      <h3 class="text-sm font-semibold text-gray-700">{m.admin_discounts_section_discount()}</h3>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_discounts_label_type()}</label>
          <select name="discount_type" bind:value={discountType}
                  class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                         focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent">
            <option value="percentage">{m.admin_discounts_type_percentage()}</option>
            <option value="fixed">{m.admin_discounts_type_fixed()}</option>
          </select>
        </div>
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            {discountType === 'percentage' ? m.admin_discounts_label_value_percent() : m.admin_discounts_label_value_amount()}
          </label>
          <input type="number" name="discount_value" bind:value={discountValue} min="0" step="0.01" required
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm font-mono
                        focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
        </div>
      </div>
      <div>
        <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
          {m.admin_discounts_label_min_order()} <span class="normal-case font-normal text-gray-400">{m.common_optional()}</span>
        </label>
        <input type="number" name="min_order_amount" bind:value={minOrder} min="0" step="0.01"
               class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm font-mono
                      focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
      </div>
    </div>

    <!-- Target -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5 space-y-5">
      <h3 class="text-sm font-semibold text-gray-700">{m.admin_discounts_section_target()}</h3>
      <div>
        <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_discounts_label_target_type()}</label>
        <select name="target_type" bind:value={targetType}
                class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                       focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent">
          <option value="all">{m.admin_discounts_target_all()}</option>
          <option value="category">{m.admin_discounts_target_category()}</option>
          <option value="product">{m.admin_discounts_target_product()}</option>
        </select>
      </div>
      {#if targetType === 'category'}
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_discounts_label_pick_category()}</label>
          <select name="target_id" bind:value={targetID} required
                  class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                         focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent">
            <option value="">—</option>
            {#each data.categories as cat}
              <option value={cat.id}>{cat.name}</option>
            {/each}
          </select>
        </div>
      {:else if targetType === 'product'}
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">{m.admin_discounts_label_pick_product()}</label>
          <select name="target_id" bind:value={targetID} required
                  class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                         focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent">
            <option value="">—</option>
            {#each data.products as p}
              <option value={p.id}>PRD-{p.number} · {p.name}</option>
            {/each}
          </select>
        </div>
      {:else}
        <input type="hidden" name="target_id" value="" />
      {/if}
    </div>

    <!-- Schedule -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5 space-y-5">
      <h3 class="text-sm font-semibold text-gray-700">{m.admin_discounts_section_schedule()}</h3>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            {m.admin_discounts_label_starts_at()} <span class="normal-case font-normal text-gray-400">{m.common_optional()}</span>
          </label>
          <input type="datetime-local" name="starts_at" bind:value={startsAt}
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
        </div>
        <div>
          <label class="block text-xs font-semibold text-gray-500 uppercase tracking-wide mb-1.5">
            {m.admin_discounts_label_ends_at()} <span class="normal-case font-normal text-gray-400">{m.common_optional()}</span>
          </label>
          <input type="datetime-local" name="ends_at" bind:value={endsAt}
                 class="w-full px-3.5 py-2.5 rounded-xl border border-gray-200 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
        </div>
      </div>
    </div>

    <!-- Active + submit -->
    <div class="bg-white rounded-2xl border border-gray-100 px-6 py-5 flex flex-col sm:flex-row sm:items-center gap-4">
      <label class="flex items-center gap-3 cursor-pointer select-none">
        <div class="relative">
          <input type="checkbox" class="sr-only peer" bind:checked={isActive} />
          <input type="hidden" name="is_active" value={isActive ? 'true' : 'false'} />
          <div class="w-10 h-6 bg-gray-200 peer-checked:bg-gray-900 rounded-full transition-colors"></div>
          <div class="absolute top-1 left-1 w-4 h-4 bg-white rounded-full shadow
                      transition-transform peer-checked:translate-x-4"></div>
        </div>
        <span class="text-sm font-medium text-gray-700">
          {isActive ? m.admin_discounts_status_active() : m.admin_discounts_status_inactive()}
        </span>
      </label>
      <div class="sm:ml-auto flex gap-3">
        <a href="/admin/discounts"
           class="px-5 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                  text-gray-700 hover:bg-gray-50 transition-colors">
          {m.common_cancel()}
        </a>
        <SaveButton loading={saving}
                class="inline-flex items-center justify-center gap-1.5 px-5 py-2.5 rounded-xl bg-gray-900
                       text-white text-sm font-medium hover:bg-gray-700 transition-colors disabled:opacity-50">
          {isNew ? m.admin_discounts_create_button() : m.common_save_changes()}
        </SaveButton>
      </div>
    </div>
  </form>
</div>
