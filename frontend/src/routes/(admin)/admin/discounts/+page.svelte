<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData } from './$types';
  import type { Campaign, Coupon } from '$lib/api/admin';
  import { showResult } from '$lib/stores/notifications.svelte';
  import { spotlight } from '$lib/actions/spotlight';
  import NewButton from '$lib/components/admin/NewButton.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();

  type Tab = 'campaigns' | 'coupons';
  let tab = $state<Tab>('campaigns');
  const tabLabel = $derived(tab === 'campaigns' ? m.admin_discounts_tab_campaigns() : m.admin_discounts_tab_coupons());

  let deleteCampaign = $state<Campaign | null>(null);
  let deleteCoupon = $state<Coupon | null>(null);

  function fmtDiscount(t: 'percentage' | 'fixed', v: number) {
    return t === 'percentage' ? `${v}%` : `HK$${v.toFixed(2)}`;
  }

  function fmtDate(s?: string) {
    if (!s) return '—';
    return new Date(s).toLocaleDateString();
  }

  function fmtRange(starts?: string, ends?: string) {
    if (!starts && !ends) return m.admin_discounts_range_always();
    return `${fmtDate(starts)} → ${fmtDate(ends)}`;
  }

  function targetLabel(c: Campaign) {
    switch (c.target_type) {
      case 'all': return m.admin_discounts_target_all();
      case 'category': return m.admin_discounts_target_category();
      case 'product': return m.admin_discounts_target_product();
    }
  }
</script>

<svelte:head><title>{m.admin_discounts_title()}</title></svelte:head>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex items-center justify-between">
    <div>
      <h2 class="text-xl font-bold text-gray-900">{tabLabel}</h2>
      <p class="text-sm text-gray-500 mt-0.5">{m.admin_discounts_subtitle()}</p>
    </div>
    {#if tab === 'campaigns'}
      <NewButton label={m.admin_discounts_new_campaign()} href="/admin/discounts/campaigns/new" />
    {:else}
      <NewButton label={m.admin_discounts_new_coupon()} href="/admin/discounts/coupons/new" />
    {/if}
  </div>

  <!-- Tabs -->
  <div class="flex gap-1 border-b border-gray-100">
    <button onclick={() => tab = 'campaigns'}
            class="px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors
                   {tab === 'campaigns' ? 'border-gray-900 text-gray-900' : 'border-transparent text-gray-400 hover:text-gray-700'}">
      {m.admin_discounts_tab_campaigns()} <span class="ml-1.5 text-xs text-gray-400">({data.campaigns.length})</span>
    </button>
    <button onclick={() => tab = 'coupons'}
            class="px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition-colors
                   {tab === 'coupons' ? 'border-gray-900 text-gray-900' : 'border-transparent text-gray-400 hover:text-gray-700'}">
      {m.admin_discounts_tab_coupons()} <span class="ml-1.5 text-xs text-gray-400">({data.coupons.length})</span>
    </button>
  </div>

  {#if tab === 'campaigns'}
    <!-- Campaigns table -->
    <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden"
         use:spotlight={{ selector: '.js-row' }}>
      {#if data.campaigns.length === 0}
        <div class="flex flex-col items-center justify-center py-20 text-center">
          <p class="text-sm font-medium text-gray-400">{m.admin_discounts_empty_campaigns()}</p>
          <a href="/admin/discounts/campaigns/new" class="mt-3 text-sm text-gray-900 underline underline-offset-2">
            {m.admin_discounts_create_first_campaign()}
          </a>
        </div>
      {:else}
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-gray-50">
              <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_discounts_col_name()}</th>
              <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_discounts_col_discount()}</th>
              <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_discounts_col_target()}</th>
              <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_discounts_col_period()}</th>
              <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_discounts_col_status()}</th>
              <th class="px-6 py-3.5"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-50">
            {#each data.campaigns as c}
              <tr class="js-row transition-colors">
                <td class="px-6 py-4">
                  <p class="font-medium text-gray-900">{c.name}</p>
                  {#if c.description}
                    <p class="text-xs text-gray-400 mt-0.5 truncate max-w-xs">{c.description}</p>
                  {/if}
                </td>
                <td class="px-6 py-4 text-gray-700 font-mono">{fmtDiscount(c.discount_type, c.discount_value)}</td>
                <td class="px-6 py-4 text-gray-500 text-xs">{targetLabel(c)}</td>
                <td class="px-6 py-4 text-gray-400 text-xs">{fmtRange(c.starts_at, c.ends_at)}</td>
                <td class="px-6 py-4">
                  <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium
                               {c.is_active ? 'bg-emerald-50 text-emerald-700' : 'bg-gray-100 text-gray-500'}">
                    {c.is_active ? m.admin_discounts_status_active() : m.admin_discounts_status_inactive()}
                  </span>
                </td>
                <td class="px-6 py-4">
                  <div class="flex items-center justify-end gap-2">
                    <a href="/admin/discounts/campaigns/{c.id}"
                       class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z"/>
                      </svg>
                    </a>
                    <button onclick={() => deleteCampaign = c}
                            class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166M18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                      </svg>
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  {:else}
    <!-- Coupons table -->
    <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden"
         use:spotlight={{ selector: '.js-row' }}>
      {#if data.coupons.length === 0}
        <div class="flex flex-col items-center justify-center py-20 text-center">
          <p class="text-sm font-medium text-gray-400">{m.admin_discounts_empty_coupons()}</p>
          <a href="/admin/discounts/coupons/new" class="mt-3 text-sm text-gray-900 underline underline-offset-2">
            {m.admin_discounts_create_first_coupon()}
          </a>
        </div>
      {:else}
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-gray-50">
              <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_discounts_col_code()}</th>
              <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_discounts_col_discount()}</th>
              <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_discounts_col_usage()}</th>
              <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_discounts_col_period()}</th>
              <th class="text-left px-6 py-3.5 text-xs font-semibold text-gray-400 uppercase tracking-wide">{m.admin_discounts_col_status()}</th>
              <th class="px-6 py-3.5"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-50">
            {#each data.coupons as c}
              <tr class="js-row transition-colors">
                <td class="px-6 py-4">
                  <p class="font-mono font-semibold text-gray-900">{c.code}</p>
                  {#if c.description}
                    <p class="text-xs text-gray-400 mt-0.5 truncate max-w-xs">{c.description}</p>
                  {/if}
                </td>
                <td class="px-6 py-4 text-gray-700 font-mono">{fmtDiscount(c.discount_type, c.discount_value)}</td>
                <td class="px-6 py-4 text-gray-500 text-xs">
                  {c.used_count}{c.max_uses != null ? ` / ${c.max_uses}` : ''}
                </td>
                <td class="px-6 py-4 text-gray-400 text-xs">{fmtRange(c.starts_at, c.ends_at)}</td>
                <td class="px-6 py-4">
                  <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium
                               {c.is_active ? 'bg-emerald-50 text-emerald-700' : 'bg-gray-100 text-gray-500'}">
                    {c.is_active ? m.admin_discounts_status_active() : m.admin_discounts_status_inactive()}
                  </span>
                </td>
                <td class="px-6 py-4">
                  <div class="flex items-center justify-end gap-2">
                    <a href="/admin/discounts/coupons/{c.id}"
                       class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z"/>
                      </svg>
                    </a>
                    <button onclick={() => deleteCoupon = c}
                            class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors">
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166M18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                      </svg>
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  {/if}
</div>

<!-- Delete campaign modal -->
{#if deleteCampaign}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => deleteCampaign = null} role="button" tabindex="-1"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="text-base font-bold text-gray-900 mb-1">{m.admin_discounts_delete_campaign_title()}</h3>
      <p class="text-sm text-gray-500 mb-5">
        {m.admin_discounts_delete_body_pre()}<span class="font-medium text-gray-700">{deleteCampaign.name}</span>{m.admin_discounts_delete_body_post()}
      </p>
      <div class="flex gap-3">
        <button onclick={() => deleteCampaign = null}
                class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                       text-gray-700 hover:bg-gray-50 transition-colors">
          {m.common_cancel()}
        </button>
        <form method="POST" action="?/deleteCampaign" class="flex-1"
              use:enhance={() => {
                const name = deleteCampaign?.name ?? '';
                return async ({ result, update }) => {
                  showResult(result, m.admin_discounts_deleted_campaign_success({ name }), m.admin_discounts_deleted_campaign_failure({ name }));
                  await update();
                  deleteCampaign = null;
                };
              }}>
          <input type="hidden" name="id" value={deleteCampaign.id} />
          <button type="submit"
                  class="w-full px-4 py-2.5 rounded-xl bg-red-500 text-white text-sm font-medium
                         hover:bg-red-600 transition-colors">
            {m.common_delete()}
          </button>
        </form>
      </div>
    </div>
  </div>
{/if}

<!-- Delete coupon modal -->
{#if deleteCoupon}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => deleteCoupon = null} role="button" tabindex="-1"></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="text-base font-bold text-gray-900 mb-1">{m.admin_discounts_delete_coupon_title()}</h3>
      <p class="text-sm text-gray-500 mb-5">
        {m.admin_discounts_delete_body_pre()}<span class="font-mono font-medium text-gray-700">{deleteCoupon.code}</span>{m.admin_discounts_delete_body_post()}
      </p>
      <div class="flex gap-3">
        <button onclick={() => deleteCoupon = null}
                class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                       text-gray-700 hover:bg-gray-50 transition-colors">
          {m.common_cancel()}
        </button>
        <form method="POST" action="?/deleteCoupon" class="flex-1"
              use:enhance={() => {
                const code = deleteCoupon?.code ?? '';
                return async ({ result, update }) => {
                  showResult(result, m.admin_discounts_deleted_coupon_success({ code }), m.admin_discounts_deleted_coupon_failure({ code }));
                  await update();
                  deleteCoupon = null;
                };
              }}>
          <input type="hidden" name="id" value={deleteCoupon.id} />
          <button type="submit"
                  class="w-full px-4 py-2.5 rounded-xl bg-red-500 text-white text-sm font-medium
                         hover:bg-red-600 transition-colors">
            {m.common_delete()}
          </button>
        </form>
      </div>
    </div>
  </div>
{/if}
