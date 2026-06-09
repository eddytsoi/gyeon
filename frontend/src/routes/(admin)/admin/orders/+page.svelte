<script lang="ts">
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { invalidateAll } from '$app/navigation';
  import { SvelteSet } from 'svelte/reactivity';
  import {
    adminDeleteOrder,
    adminCreateReceiptBatch,
    adminGetReceiptBatch,
    adminBatchWaybills,
    type ReceiptBatchError,
    type WaybillBatchSkip
  } from '$lib/api/admin';
  import { notify } from '$lib/stores/notifications.svelte';
  import { getLocale } from '$lib/i18n';
  import type { Order } from '$lib/types';
  import type { PageData } from './$types';
  import { spotlight } from '$lib/actions/spotlight';
  import Pagination from '$lib/components/admin/Pagination.svelte';
  import SearchInput from '$lib/components/admin/SearchInput.svelte';
  import NewButton from '$lib/components/admin/NewButton.svelte';
  import Spinner from '$lib/components/admin/Spinner.svelte';
  import AdminModal from '$lib/components/admin/AdminModal.svelte';
  import * as m from '$lib/paraglide/messages';
  import { orderStatusLabel } from '$lib/orderStatus';

  let { data }: { data: PageData } = $props();

  let deleteTarget = $state<Order | null>(null);
  let deleting = $state(false);

  // ── Batch actions ──────────────────────────────────────────────────────────
  let selectedIds = new SvelteSet<string>();
  let batchAction = $state<'download_receipt' | 'download_waybill'>('download_receipt');
  let running = $state(false);
  let batchErrors = $state<(ReceiptBatchError | WaybillBatchSkip)[] | null>(null);

  const allSelected = $derived(
    data.orders.length > 0 && data.orders.every((o) => selectedIds.has(o.id))
  );
  const someSelected = $derived(selectedIds.size > 0 && !allSelected);

  function toggleRow(id: string) {
    if (selectedIds.has(id)) selectedIds.delete(id);
    else selectedIds.add(id);
  }

  function toggleAll() {
    if (allSelected) {
      for (const o of data.orders) selectedIds.delete(o.id);
    } else {
      for (const o of data.orders) selectedIds.add(o.id);
    }
  }

  function reasonLabel(reason: string): string {
    switch (reason) {
      case 'not_receiptable': return m.admin_orders_batch_error_not_receiptable();
      case 'generation_failed': return m.admin_orders_batch_error_generation_failed();
      case 'not_found': return m.admin_orders_batch_error_not_found();
      case 'not_processing': return m.admin_orders_batch_error_not_processing();
      case 'no_waybill': return m.admin_orders_batch_error_no_waybill();
      case 'download_failed': return m.admin_orders_batch_error_download_failed();
      default: return reason;
    }
  }

  function triggerDownload(batchId: string) {
    const a = document.createElement('a');
    a.href = `/admin/order-receipts/batch/${batchId}/download`;
    a.rel = 'noopener';
    // download attr both hints a save and makes SvelteKit's client router skip
    // the click (it's a server endpoint, not a page route). The actual filename
    // comes from the proxy's Content-Disposition header.
    a.setAttribute('download', '');
    document.body.appendChild(a);
    a.click();
    a.remove();
  }

  const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

  async function runBatch() {
    if (batchAction === 'download_waybill') return runWaybillBatch();
    if (running || selectedIds.size === 0 || !data.token) return;
    running = true;
    batchErrors = null;
    try {
      const { batch_id } = await adminCreateReceiptBatch(data.token, [...selectedIds], getLocale());
      // Poll until the worker finishes. ~3 min safety cap (200 cold renders).
      const deadline = Date.now() + 3 * 60 * 1000;
      let status = await adminGetReceiptBatch(data.token, batch_id);
      while (status.status !== 'succeeded' && status.status !== 'failed') {
        if (Date.now() > deadline) {
          notify.error(m.admin_orders_batch_timeout());
          return;
        }
        await sleep(1200);
        status = await adminGetReceiptBatch(data.token, batch_id);
      }
      if (status.status === 'failed') {
        notify.error(m.admin_orders_batch_failed());
        return;
      }
      if (status.succeeded_count > 0 && status.zip_ready) {
        triggerDownload(batch_id);
        selectedIds.clear();
        if (status.errors.length > 0) {
          batchErrors = status.errors;
          notify.warning(
            m.admin_orders_batch_done_title(),
            m.admin_orders_batch_summary({ ok: status.succeeded_count, skip: status.errors.length })
          );
        } else {
          notify.success(m.admin_orders_batch_all_ok({ ok: status.succeeded_count }));
        }
      } else {
        // Nothing produced a receipt — show the skip details only.
        batchErrors = status.errors;
        notify.error(m.admin_orders_batch_none_title(), m.admin_orders_batch_none_body());
      }
    } catch (e) {
      notify.error(
        m.admin_orders_batch_failed(),
        e instanceof Error ? e.message : undefined
      );
    } finally {
      running = false;
    }
  }

  function todayStamp(): string {
    const d = new Date();
    const mm = String(d.getMonth() + 1).padStart(2, '0');
    const dd = String(d.getDate()).padStart(2, '0');
    return `${d.getFullYear()}${mm}${dd}`;
  }

  async function runWaybillBatch() {
    if (running || selectedIds.size === 0 || !data.token) return;
    running = true;
    batchErrors = null;
    try {
      const { pdf, report } = await adminBatchWaybills(data.token, [...selectedIds]);
      if (pdf) {
        const url = URL.createObjectURL(pdf);
        const a = document.createElement('a');
        a.href = url;
        a.download = `SF-Waybills-${todayStamp()}.pdf`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(url);
        selectedIds.clear();
        if (report.errors.length > 0) {
          batchErrors = report.errors;
          notify.warning(
            m.admin_orders_batch_waybills_done_title(),
            m.admin_orders_batch_summary({ ok: report.succeeded_count, skip: report.errors.length })
          );
        } else {
          notify.success(m.admin_orders_batch_waybills_all_ok({ ok: report.succeeded_count }));
        }
      } else {
        // No selected order had a waybill — show the skip details only.
        batchErrors = report.errors;
        notify.error(
          m.admin_orders_batch_waybills_none_title(),
          m.admin_orders_batch_waybills_none_body()
        );
      }
    } catch (e) {
      notify.error(
        m.admin_orders_batch_failed(),
        e instanceof Error ? e.message : undefined
      );
    } finally {
      running = false;
    }
  }

  const STATUSES = [
    'pending', 'paid', 'processing', 'shipped', 'delivered', 'cancelled', 'refunded'
  ] as const;

  const NEEDS_ACTION = ['pending', 'paid', 'processing'] as const;

  const ROLES = ['customer', 'installer', 'installer_v2'] as const;

  const statusColour: Record<string, string> = {
    pending:    'bg-amber-50 text-amber-700',
    paid:       'bg-blue-50 text-blue-700',
    processing: 'bg-indigo-50 text-indigo-700',
    shipped:    'bg-violet-50 text-violet-700',
    delivered:  'bg-green-50 text-green-700',
    cancelled:  'bg-gray-100 text-gray-500',
    refunded:   'bg-red-50 text-red-700',
  };

  const roleColour: Record<string, string> = {
    customer:     'bg-gray-50 text-gray-700',
    installer:    'bg-orange-50 text-orange-700',
    installer_v2: 'bg-sky-50 text-sky-700',
  };

  function roleLabel(role: string): string {
    if (role === 'installer') return m.admin_orders_filter_role_installer();
    if (role === 'installer_v2') return m.admin_orders_filter_role_installer_v2();
    if (role === 'customer') return m.admin_orders_filter_role_customer();
    return role;
  }

  // Selected status chips — derived from URL so back/forward stays in sync.
  const selectedStatuses = $derived(new Set(data.statuses));
  const selectedRoles = $derived(new Set(data.roles));
  const hasFilters = $derived(
    !!data.q || data.statuses.length > 0 || !!data.from || !!data.to || data.unread ||
    data.roles.length > 0 || data.hasNotes
  );
  // "Needs action" highlights only when the selected set is exactly the
  // shortcut's three statuses — partial overlap is treated as a manual
  // selection, not the quick filter.
  const needsActionActive = $derived(
    selectedStatuses.size === NEEDS_ACTION.length &&
    NEEDS_ACTION.every(s => selectedStatuses.has(s))
  );

  function pushParams(mutate: (p: URLSearchParams) => void) {
    const url = new URL(page.url);
    mutate(url.searchParams);
    url.searchParams.delete('page'); // any filter change resets pagination
    goto(url.pathname + url.search, { replaceState: true, keepFocus: true, noScroll: true });
  }

  function onSearch(q: string) {
    pushParams(p => { q ? p.set('q', q) : p.delete('q'); });
  }

  function toggleStatus(s: string) {
    pushParams(p => {
      const current = new Set((p.get('status') ?? '').split(',').filter(Boolean));
      if (current.has(s)) current.delete(s);
      else current.add(s);
      if (current.size === 0) p.delete('status');
      else p.set('status', [...current].join(','));
    });
  }

  function clearStatuses() {
    pushParams(p => p.delete('status'));
  }

  function onDateChange(key: 'from' | 'to', value: string) {
    pushParams(p => { value ? p.set(key, value) : p.delete(key); });
  }

  function toggleNeedsAction() {
    pushParams(p => {
      if (needsActionActive) p.delete('status');
      else p.set('status', NEEDS_ACTION.join(','));
    });
  }

  function toggleUnread() {
    pushParams(p => {
      if (data.unread) p.delete('unread');
      else p.set('unread', '1');
    });
  }

  function toggleRole(role: string) {
    pushParams(p => {
      const current = new Set((p.get('role') ?? '').split(',').filter(Boolean));
      if (current.has(role)) current.delete(role);
      else current.add(role);
      if (current.size === 0) p.delete('role');
      else p.set('role', [...current].join(','));
    });
  }

  function toggleHasNotes() {
    pushParams(p => {
      if (data.hasNotes) p.delete('has_notes');
      else p.set('has_notes', '1');
    });
  }

  function clearAll() {
    pushParams(p => {
      p.delete('q');
      p.delete('status');
      p.delete('from');
      p.delete('to');
      p.delete('unread');
      p.delete('role');
      p.delete('has_notes');
    });
  }

  async function confirmDelete() {
    if (!deleteTarget || !data.token) return;
    const target = deleteTarget;
    const shortId = target.order_number || `ORD-${target.number}`;
    deleting = true;
    try {
      await adminDeleteOrder(data.token, target.id);
      notify.success(m.admin_orders_deleted_success({ id: shortId }));
      deleteTarget = null;
      await invalidateAll();
    } catch (e) {
      notify.error(
        m.admin_orders_delete_failure({ id: shortId }),
        e instanceof Error ? e.message : m.admin_orders_delete_failure_default()
      );
    } finally {
      deleting = false;
    }
  }
</script>

<svelte:head><title>{m.admin_orders_title()}</title></svelte:head>

<div class="flex items-center justify-between mb-6 gap-3">
  <div class="flex items-baseline gap-3 min-w-0">
    <h1 class="text-2xl font-bold text-gray-900">{m.admin_orders_heading()}</h1>
    <span class="text-sm text-gray-400">
      {#if hasFilters}
        {data.total === 1 ? m.admin_orders_count_match_one({ count: data.total }) : m.admin_orders_count_match_many({ count: data.total })}
      {:else}
        {m.admin_orders_count_total({ count: data.total })}
      {/if}
    </span>
  </div>
  <NewButton href="/admin/orders/new" label={m.admin_orders_new_button()} />
</div>

<!-- Filters -->
<div class="mb-4 space-y-3">
  <div class="flex flex-wrap items-center gap-3">
    <SearchInput value={data.q} placeholder={m.admin_orders_search_placeholder()} onChange={onSearch} />

    <div class="flex items-center gap-2">
      <label class="text-xs text-gray-500" for="orders-from">{m.admin_orders_filter_from()}</label>
      <input id="orders-from" type="date" value={data.from} max={data.to || undefined}
             oninput={(e) => onDateChange('from', (e.currentTarget as HTMLInputElement).value)}
             class="text-sm px-2.5 py-2 rounded-xl border border-gray-200 bg-white
                    focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-gray-900" />
      <label class="text-xs text-gray-500" for="orders-to">{m.admin_orders_filter_to()}</label>
      <input id="orders-to" type="date" value={data.to} min={data.from || undefined}
             oninput={(e) => onDateChange('to', (e.currentTarget as HTMLInputElement).value)}
             class="text-sm px-2.5 py-2 rounded-xl border border-gray-200 bg-white
                    focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-gray-900" />
    </div>

    {#if hasFilters}
      <button type="button" onclick={clearAll}
              class="text-xs text-gray-500 hover:text-gray-900 underline-offset-2 hover:underline">
        {m.admin_orders_filter_clear()}
      </button>
    {/if}
  </div>

  <div class="flex flex-wrap items-center gap-2">
    <button type="button" onclick={toggleNeedsAction}
            class="px-3 py-1 rounded-full text-xs font-medium border transition-colors
                   {needsActionActive
                     ? 'bg-amber-500 text-white border-amber-500'
                     : 'bg-white text-gray-700 border-gray-200 hover:border-amber-400'}">
      {m.admin_orders_filter_needs_action()}
    </button>
    <button type="button" onclick={toggleUnread}
            class="px-3 py-1 rounded-full text-xs font-medium border transition-colors inline-flex items-center gap-1.5
                   {data.unread
                     ? 'bg-green-500 text-white border-green-500'
                     : 'bg-white text-gray-700 border-gray-200 hover:border-green-400'}">
      <span class="inline-block w-1.5 h-1.5 rounded-full {data.unread ? 'bg-white' : 'bg-green-500'}"></span>
      {m.admin_orders_filter_unread()}
    </button>
    <span class="h-4 w-px bg-gray-200 mx-1" aria-hidden="true"></span>
    <button type="button" onclick={clearStatuses}
            class="px-3 py-1 rounded-full text-xs font-medium border transition-colors
                   {selectedStatuses.size === 0
                     ? 'bg-gray-900 text-white border-gray-900'
                     : 'bg-white text-gray-600 border-gray-200 hover:border-gray-400'}">
      {m.admin_orders_filter_status_all()}
    </button>
    {#each STATUSES as s}
      {@const active = selectedStatuses.has(s)}
      <button type="button" onclick={() => toggleStatus(s)}
              class="px-3 py-1 rounded-full text-xs font-medium border transition-colors
                     {active
                       ? `${statusColour[s]} border-current`
                       : 'bg-white text-gray-600 border-gray-200 hover:border-gray-400'}">
        {orderStatusLabel(s)}
      </button>
    {/each}
  </div>

  <!-- Row 2: role / shipping / carrier / has-notes -->
  <div class="flex flex-wrap items-center gap-2">
    {#each ROLES as role}
      {@const active = selectedRoles.has(role)}
      <button type="button" onclick={() => toggleRole(role)}
              class="px-3 py-1 rounded-full text-xs font-medium border transition-colors
                     {active
                       ? `${roleColour[role]} border-current`
                       : 'bg-white text-gray-600 border-gray-200 hover:border-gray-400'}">
        {roleLabel(role)}
      </button>
    {/each}
    <span class="h-4 w-px bg-gray-200 mx-1" aria-hidden="true"></span>
    <button type="button" onclick={toggleHasNotes}
            class="px-3 py-1 rounded-full text-xs font-medium border transition-colors inline-flex items-center gap-1.5
                   {data.hasNotes
                     ? 'bg-yellow-100 text-yellow-800 border-yellow-300'
                     : 'bg-white text-gray-700 border-gray-200 hover:border-yellow-400'}">
      <span aria-hidden="true">💬</span>
      {m.admin_orders_filter_has_notes()}
    </button>
  </div>
</div>

{#if selectedIds.size > 0}
  <div class="mb-4 flex flex-wrap items-center gap-3 rounded-2xl border border-gray-200 bg-gray-50 px-4 py-3">
    <span class="text-sm font-medium text-gray-700">
      {m.admin_orders_batch_selected({ count: selectedIds.size })}
    </span>
    <select bind:value={batchAction} disabled={running}
            aria-label={m.admin_orders_batch_action_download_receipt()}
            class="text-sm px-3 py-2 rounded-xl border border-gray-200 bg-white
                   focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-gray-900
                   disabled:opacity-50">
      <option value="download_receipt">{m.admin_orders_batch_action_download_receipt()}</option>
      <option value="download_waybill">{m.admin_orders_batch_action_download_waybill()}</option>
    </select>
    <button type="button" onclick={runBatch} disabled={running}
            class="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-gray-900 text-white
                   text-sm font-medium hover:bg-gray-800 transition-colors disabled:opacity-50">
      {#if running}<Spinner />{/if}
      {running ? m.admin_orders_batch_running() : m.admin_orders_batch_execute()}
    </button>
    <button type="button" onclick={() => selectedIds.clear()} disabled={running}
            class="text-xs text-gray-500 hover:text-gray-900 underline-offset-2 hover:underline disabled:opacity-50">
      {m.admin_orders_batch_clear()}
    </button>
  </div>
{/if}

<div class="bg-white rounded-2xl border border-gray-100 overflow-hidden"
     use:spotlight={{ selector: '.js-row' }}>
  <table class="w-full text-sm">
    <thead class="bg-gray-50 border-b border-gray-100">
      <tr>
        <th class="w-10 px-5 py-3">
          <input type="checkbox" checked={allSelected} indeterminate={someSelected}
                 onchange={toggleAll} aria-label={m.admin_orders_select_all_aria()}
                 class="h-4 w-4 rounded border-gray-300 text-gray-900 focus:ring-gray-900 cursor-pointer" />
        </th>
        <th class="text-left px-5 py-3 font-medium text-gray-500">{m.admin_orders_col_id()}</th>
        <th class="text-left px-5 py-3 font-medium text-gray-500">{m.admin_orders_col_status()}</th>
        <th class="text-left px-5 py-3 font-medium text-gray-500 hidden md:table-cell">{m.admin_orders_col_customer()}</th>
        <th class="text-left px-5 py-3 font-medium text-gray-500 hidden lg:table-cell">{m.admin_orders_col_phone()}</th>
        <th class="text-right px-5 py-3 font-medium text-gray-500 hidden lg:table-cell">{m.admin_orders_col_items()}</th>
        <th class="text-left px-5 py-3 font-medium text-gray-500 hidden sm:table-cell">{m.admin_orders_col_date()}</th>
        <th class="text-right px-5 py-3 font-medium text-gray-500">{m.admin_orders_col_total()}</th>
        <th class="px-5 py-3"></th>
      </tr>
    </thead>
    <tbody class="divide-y divide-gray-50">
      {#each data.orders as order}
        <tr class="js-row transition-colors" class:bg-gray-50={selectedIds.has(order.id)}>
          <td class="px-5 py-3">
            <input type="checkbox" checked={selectedIds.has(order.id)}
                   onchange={() => toggleRow(order.id)}
                   aria-label={m.admin_orders_select_row_aria({ id: order.order_number || `ORD-${order.number}` })}
                   class="h-4 w-4 rounded border-gray-300 text-gray-900 focus:ring-gray-900 cursor-pointer" />
          </td>
          <td class="px-5 py-3 font-mono text-xs text-gray-700">
            <span class="inline-flex items-center gap-2">
              <a href="/admin/orders/{order.id}"
                 class="text-gray-700 hover:text-indigo-700 hover:underline underline-offset-2">
                {order.order_number || `ORD-${order.number}`}
              </a>
              {#if (data.unreadCounts?.[order.id] ?? 0) > 0}
                <span title={m.admin_orders_unread_aria()}
                      class="inline-flex items-center justify-center min-w-[18px] h-[18px] px-1.5
                             rounded-full bg-green-500 text-white text-[10px] font-bold leading-none">
                  {data.unreadCounts[order.id]}
                </span>
              {/if}
              {#if order.notes}
                <span title={m.admin_orders_has_notes_indicator_tooltip()} aria-hidden="true"
                      class="text-yellow-600 text-[11px]">💬</span>
              {/if}
            </span>
          </td>
          <td class="px-5 py-3">
            <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                         {statusColour[order.status] ?? 'bg-gray-100 text-gray-500'}">
              {orderStatusLabel(order.status)}
            </span>
          </td>
          <td class="px-5 py-3 text-gray-700 hidden md:table-cell">
            <span class="inline-flex items-center gap-1.5">
              {order.customer_name || '—'}
              {#if order.customer_role && order.customer_role !== 'customer'}
                <span class="inline-flex items-center px-1.5 py-0.5 rounded-full text-[10px] font-medium {roleColour[order.customer_role] ?? roleColour.installer}">
                  {roleLabel(order.customer_role)}
                </span>
              {/if}
            </span>
          </td>
          <td class="px-5 py-3 text-gray-500 hidden lg:table-cell font-mono text-xs">
            {order.customer_phone || '—'}
          </td>
          <td class="px-5 py-3 text-right text-gray-700 hidden lg:table-cell tabular-nums">
            {order.items_count ?? '—'}
          </td>
          <td class="px-5 py-3 text-gray-500 hidden sm:table-cell">
            {new Date(order.created_at).toLocaleString('sv-SE', { timeZone: 'Asia/Hong_Kong', hour12: false, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}
          </td>
          <td class="px-5 py-3 text-right font-medium text-gray-900">
            <span class="inline-flex items-center gap-1.5 justify-end">
              HK${order.total.toFixed(2)}
              {#if order.refunded_at}
                <span title={m.admin_orders_refund_tooltip({ amount: (order.refund_amount ?? 0).toFixed(2) })}
                      class="text-red-500 text-[11px]" aria-hidden="true">↩</span>
              {/if}
            </span>
          </td>
          <td class="px-5 py-3">
            <div class="flex items-center justify-end gap-1">
              <a href="/admin/orders/{order.id}"
                 title={m.admin_orders_action_details()}
                 aria-label={m.admin_orders_aria_details()}
                 class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.964-7.178Z"/>
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/>
                </svg>
              </a>
              <button onclick={() => deleteTarget = order}
                      disabled={order.status !== 'cancelled' && order.status !== 'refunded'}
                      title={order.status === 'cancelled' || order.status === 'refunded' ? m.admin_orders_action_delete() : m.admin_orders_delete_disabled_hint()}
                      aria-label={m.admin_orders_aria_delete()}
                      class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors disabled:opacity-30 disabled:cursor-not-allowed disabled:hover:text-gray-400 disabled:hover:bg-transparent">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round"
                    d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                </svg>
              </button>
            </div>
          </td>
        </tr>
      {:else}
        <tr>
          <td colspan="9" class="px-5 py-10 text-center text-gray-400">
            {#if hasFilters}
              <p class="font-medium text-gray-700 mb-1">{m.admin_orders_empty_no_match()}</p>
              <p class="text-xs">{m.admin_orders_empty_no_match_hint()}</p>
            {:else}
              {m.admin_orders_empty()}
            {/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<Pagination total={data.total} pageSize={data.pageSize} currentPage={data.page} />

<!-- Delete confirmation modal -->
{#if deleteTarget}
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="absolute inset-0 bg-black/40 backdrop-blur-sm"
         onclick={() => { if (!deleting) deleteTarget = null; }}
         role="button" tabindex="-1" aria-label={m.admin_orders_aria_close()}></div>
    <div class="relative bg-white rounded-2xl shadow-2xl p-6 w-full max-w-sm">
      <h3 class="text-base font-bold text-gray-900 mb-1">{m.admin_orders_delete_title()}</h3>
      <p class="text-sm text-gray-500 mb-5">
        {m.admin_orders_delete_body_pre()}<span class="font-mono font-medium text-gray-700">{deleteTarget.order_number || `ORD-${deleteTarget.number}`}</span>{m.admin_orders_delete_body_post()}
      </p>
      <div class="flex gap-3">
        <button onclick={() => deleteTarget = null} disabled={deleting}
                class="flex-1 px-4 py-2.5 rounded-xl border border-gray-200 text-sm font-medium
                       text-gray-700 hover:bg-gray-50 transition-colors disabled:opacity-50">
          {m.common_cancel()}
        </button>
        <button onclick={confirmDelete} disabled={deleting}
                class="flex-1 px-4 py-2.5 rounded-xl bg-red-500 text-white text-sm font-medium
                       hover:bg-red-600 transition-colors disabled:opacity-50">
          {deleting ? m.admin_orders_deleting() : m.admin_orders_delete_button()}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Batch receipt skipped/failed orders -->
<AdminModal open={batchErrors !== null} onClose={() => (batchErrors = null)} size="md">
  <h3 class="text-base font-bold text-gray-900 mb-3">{m.admin_orders_batch_errors_title()}</h3>
  <ul class="mb-5 max-h-72 overflow-y-auto divide-y divide-gray-100 text-sm">
    {#each batchErrors ?? [] as err}
      <li class="flex items-center justify-between gap-3 py-2">
        <span class="font-mono text-xs text-gray-700">{err.order_number || err.order_id}</span>
        <span class="text-gray-500">{reasonLabel(err.reason)}</span>
      </li>
    {/each}
  </ul>
  <div class="flex justify-end">
    <button type="button" onclick={() => (batchErrors = null)}
            class="px-4 py-2.5 rounded-xl bg-gray-900 text-white text-sm font-medium
                   hover:bg-gray-800 transition-colors">
      {m.admin_orders_batch_errors_done()}
    </button>
  </div>
</AdminModal>
