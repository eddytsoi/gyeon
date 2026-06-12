<script lang="ts">
  import { enhance } from '$app/forms';
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/state';
  import {
    adminCombineMutationsIntoOrder,
    adminDeleteStockMutation,
    adminDuplicateStockMutation,
    adminExecuteStockMutation,
    StockMutationInsufficientStockError,
    type StockMutationImportResult,
    type StockMutationSummary,
    type StockMutationType
  } from '$lib/api/admin';
  import { notify } from '$lib/stores/notifications.svelte';
  import CustomerPicker, { type CustomerSelection } from '$lib/components/admin/CustomerPicker.svelte';
  import NewButton from '$lib/components/admin/NewButton.svelte';
  import Pagination from '$lib/components/admin/Pagination.svelte';
  import SearchInput from '$lib/components/admin/SearchInput.svelte';
  import * as m from '$lib/paraglide/messages';
  import type { PageData } from './$types';

  let { data }: { data: PageData } = $props();

  // ── Combine executed out-mutations into one order ─────────────────────────
  // Only already-executed, not-yet-consumed out-mutations are eligible. The
  // selected set is rolled into a single accounting-only order (no re-deduction
  // of stock); the backend locks each source mutation to that order.
  let selectedIds = $state<string[]>([]);
  let combineOpen = $state(false);
  let combineCustomer = $state<CustomerSelection>({ kind: 'none' });
  let combineAddressId = $state('');
  let combineStatus = $state<'pending' | 'processing'>('pending');
  let combineCoupon = $state('');
  let combineSubmitting = $state(false);

  function isCombinable(row: StockMutationSummary): boolean {
    return row.type === 'out' && row.status === 'executed' && !row.consumed_by_order_id;
  }

  const combinableRows = $derived(data.list.items.filter(isCombinable));
  const allCombinableSelected = $derived(
    combinableRows.length > 0 && combinableRows.every((r) => selectedIds.includes(r.id))
  );

  function toggleSelect(id: string) {
    selectedIds = selectedIds.includes(id)
      ? selectedIds.filter((x) => x !== id)
      : [...selectedIds, id];
  }

  function toggleSelectAll() {
    selectedIds = allCombinableSelected ? [] : combinableRows.map((r) => r.id);
  }

  function openCombine() {
    // Drop any ids that scrolled out of view / changed page so the dialog only
    // acts on still-visible, still-eligible rows.
    selectedIds = selectedIds.filter((id) => combinableRows.some((r) => r.id === id));
    if (selectedIds.length === 0) return;
    combineCustomer = { kind: 'none' };
    combineAddressId = '';
    combineStatus = 'pending';
    combineCoupon = '';
    combineOpen = true;
  }

  // Preselect the customer's default address whenever an existing customer is
  // picked (combine bills a known customer/installer, so a saved address is the
  // common case).
  $effect(() => {
    if (combineCustomer.kind === 'existing' && combineCustomer.addresses.length > 0) {
      const def =
        combineCustomer.addresses.find((a) => a.is_default) ?? combineCustomer.addresses[0];
      if (!combineAddressId || !combineCustomer.addresses.some((a) => a.id === combineAddressId)) {
        combineAddressId = def.id;
      }
    }
  });

  const combineReady = $derived(
    combineCustomer.kind === 'existing' &&
      combineCustomer.addresses.length > 0 &&
      combineAddressId !== ''
  );

  async function submitCombine() {
    if (!data.token || !combineReady || combineCustomer.kind !== 'existing') return;
    combineSubmitting = true;
    try {
      const order = await adminCombineMutationsIntoOrder(data.token, {
        mutation_ids: selectedIds,
        customer_id: combineCustomer.customer.id,
        shipping_address_id: combineAddressId,
        initial_status: combineStatus,
        coupon_code: combineCoupon.trim() ? combineCoupon.trim() : null
      });
      notify.success(m.admin_stock_mutations_combine_success({ id: order.order_number }));
      combineOpen = false;
      selectedIds = [];
      await goto(`/admin/orders/${order.id}`);
    } catch (e) {
      notify.error(
        m.admin_stock_mutations_combine_failure(),
        e instanceof Error ? e.message : m.admin_stock_mutations_unknown_error()
      );
      // A 409/422 usually means a row was consumed by a concurrent op — refresh
      // so its checkbox flips to the disabled "已綜合" state.
      await invalidateAll();
    } finally {
      combineSubmitting = false;
    }
  }

  let deleteTarget = $state<StockMutationSummary | null>(null);
  let deleting = $state(false);
  let executeTarget = $state<StockMutationSummary | null>(null);
  let executingId = $state<string | null>(null);
  let duplicatingId = $state<string | null>(null);

  // CSV import — one hidden file input shared by both Import buttons. The
  // active "direction" is captured when the user clicks the button; on file
  // pick we submit the matching form (?/importIn or ?/importOut).
  let importDirection = $state<StockMutationType | null>(null);
  let importing = $state(false);
  let importErrors = $state<StockMutationImportResult['errors']>(undefined);
  let fileInputEl = $state<HTMLInputElement | null>(null);
  let importInFormEl = $state<HTMLFormElement | null>(null);
  let importOutFormEl = $state<HTMLFormElement | null>(null);

  function openImportPicker(direction: StockMutationType) {
    importDirection = direction;
    importErrors = undefined;
    fileInputEl?.click();
  }

  function onFilePicked() {
    if (!fileInputEl?.files?.length) return;
    const form = importDirection === 'in' ? importInFormEl : importOutFormEl;
    if (!form) return;
    // Attach the picked file to the hidden file input INSIDE the target form
    // (the visible file input is shared and lives outside both forms).
    const targetInput = form.querySelector('input[name="file"]') as HTMLInputElement | null;
    if (targetInput) {
      targetInput.files = fileInputEl.files;
    }
    form.requestSubmit();
  }

  async function handleImportResult(
    result: { type: string; data?: unknown }
  ) {
    if (result.type !== 'success' && result.type !== 'failure') return;
    const r = result.data as
      | { importResult?: StockMutationImportResult; importError?: string }
      | undefined;
    if (r?.importResult) {
      const { mutation, imported, skipped, errors } = r.importResult;
      importErrors = errors;
      if (mutation && imported > 0) {
        if (skipped > 0) {
          notify.error(
            m.admin_stock_mutations_import_partial({
              ok: String(imported),
              skip: String(skipped)
            })
          );
        } else {
          notify.success(m.admin_stock_mutations_import_success({ n: String(imported) }));
        }
        await goto(`/admin/stock-mutations/${mutation.id}`);
      } else {
        notify.error(
          m.admin_stock_mutations_import_zero_rows({ skip: String(skipped) })
        );
      }
    } else {
      notify.error(
        m.admin_stock_mutations_import_failure(),
        r?.importError ?? ''
      );
    }
  }

  function pushParams(mutate: (p: URLSearchParams) => void) {
    const url = new URL(page.url);
    mutate(url.searchParams);
    url.searchParams.delete('page');
    goto(url.pathname + url.search, { replaceState: true, keepFocus: true, noScroll: true });
  }

  function onSearch(q: string) {
    pushParams(p => { q ? p.set('q', q) : p.delete('q'); });
  }
  function setStatus(s: '' | 'draft' | 'executed') {
    pushParams(p => { s ? p.set('status', s) : p.delete('status'); });
  }
  function setType(t: '' | 'in' | 'out') {
    pushParams(p => { t ? p.set('type', t) : p.delete('type'); });
  }
  function setDate(key: 'from' | 'to', value: string) {
    pushParams(p => { value ? p.set(key, value) : p.delete(key); });
  }
  function setCreator(id: string) {
    pushParams(p => { id ? p.set('created_by', id) : p.delete('created_by'); });
  }
  function clearAll() {
    pushParams(p => {
      p.delete('q'); p.delete('status'); p.delete('type');
      p.delete('from'); p.delete('to'); p.delete('created_by');
    });
  }

  // ── Date-range quick presets ──────────────────────────────────────────────
  // Presets write into the same from/to params as the manual date inputs, so
  // the inputs visibly mirror the picked range. Weeks start Monday. Dates are
  // computed in the browser's local timezone (HKT for our users); ymd avoids
  // toISOString() which would shift across the UTC day boundary.
  type DatePreset = 'all' | 'this_week' | 'this_month' | 'last_week' | 'last_month';

  function ymd(d: Date): string {
    const y = d.getFullYear();
    const mo = String(d.getMonth() + 1).padStart(2, '0');
    const da = String(d.getDate()).padStart(2, '0');
    return `${y}-${mo}-${da}`;
  }

  function presetRange(p: Exclude<DatePreset, 'all'>): { from: string; to: string } {
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const dow = (today.getDay() + 6) % 7; // 0=Mon … 6=Sun
    if (p === 'this_week' || p === 'last_week') {
      const mon = new Date(today);
      mon.setDate(today.getDate() - dow - (p === 'last_week' ? 7 : 0));
      const sun = new Date(mon);
      sun.setDate(mon.getDate() + 6);
      return { from: ymd(mon), to: ymd(sun) };
    }
    const monthOffset = p === 'last_month' ? -1 : 0;
    const first = new Date(today.getFullYear(), today.getMonth() + monthOffset, 1);
    const last = new Date(today.getFullYear(), today.getMonth() + monthOffset + 1, 0);
    return { from: ymd(first), to: ymd(last) };
  }

  function applyPreset(p: DatePreset) {
    const r = p === 'all' ? { from: '', to: '' } : presetRange(p);
    pushParams(sp => {
      r.from ? sp.set('from', r.from) : sp.delete('from');
      r.to ? sp.set('to', r.to) : sp.delete('to');
    });
  }

  // Which preset (if any) the current from/to range corresponds to — drives the
  // active-button highlight. null means a custom (manually-tweaked) range.
  const activePreset = $derived.by((): DatePreset | null => {
    if (!data.from && !data.to) return 'all';
    for (const p of ['this_week', 'this_month', 'last_week', 'last_month'] as const) {
      const r = presetRange(p);
      if (r.from === data.from && r.to === data.to) return p;
    }
    return null;
  });

  async function confirmDelete() {
    if (!deleteTarget || !data.token) return;
    const t = deleteTarget;
    deleting = true;
    try {
      await adminDeleteStockMutation(data.token, t.id);
      notify.success(m.admin_stock_mutations_deleted_success({ id: t.mutation_number }));
      deleteTarget = null;
      await invalidateAll();
    } catch (e) {
      notify.error(
        m.admin_stock_mutations_delete_failure({ id: t.mutation_number }),
        e instanceof Error ? e.message : m.admin_stock_mutations_unknown_error()
      );
    } finally {
      deleting = false;
    }
  }

  async function confirmExecute() {
    if (!executeTarget || !data.token) return;
    const row = executeTarget;
    executingId = row.id;
    try {
      await adminExecuteStockMutation(data.token, row.id);
      notify.success(m.admin_stock_mutations_executed_success({ id: row.mutation_number }));
      executeTarget = null;
      await invalidateAll();
    } catch (e) {
      if (e instanceof StockMutationInsufficientStockError) {
        const lines = e.conflicts
          .map(c => `• ${c.product_name ?? c.variant_id} (${c.variant_sku ?? '—'}): ${m.admin_stock_mutations_conflict_line({ requested: String(c.requested), available: String(c.available) })}`)
          .join('\n');
        notify.error(m.admin_stock_mutations_insufficient_stock_title({ id: row.mutation_number }), lines);
        executeTarget = null;
      } else {
        notify.error(
          m.admin_stock_mutations_execute_failure({ id: row.mutation_number }),
          e instanceof Error ? e.message : m.admin_stock_mutations_unknown_error()
        );
      }
    } finally {
      executingId = null;
    }
  }

  async function duplicate(row: StockMutationSummary) {
    if (!data.token) return;
    duplicatingId = row.id;
    try {
      const created = await adminDuplicateStockMutation(data.token, row.id);
      notify.success(m.admin_stock_mutations_duplicated_success({ id: created.mutation_number }));
      goto(`/admin/stock-mutations/${created.id}`);
    } catch (e) {
      notify.error(
        m.admin_stock_mutations_duplicate_failure(),
        e instanceof Error ? e.message : m.admin_stock_mutations_unknown_error()
      );
    } finally {
      duplicatingId = null;
    }
  }

  function fmtDateTime(iso: string | undefined | null) {
    if (!iso) return '—';
    try {
      return new Date(iso).toLocaleString();
    } catch {
      return iso;
    }
  }

  const hasFilters = $derived(
    !!data.q || !!data.status || !!data.type || !!data.from || !!data.to || !!data.createdBy
  );
</script>

<svelte:head><title>{m.admin_stock_mutations_title()}</title></svelte:head>

<div class="space-y-4">
  <div class="flex items-center justify-between gap-2 flex-wrap">
    <h1 class="text-2xl font-semibold text-gray-900">{m.admin_stock_mutations_heading()}</h1>
    <div class="flex items-center gap-2 flex-wrap">
      <button
        type="button"
        onclick={() => openImportPicker('in')}
        disabled={importing}
        class="inline-flex items-center gap-1.5 px-3 py-2 text-sm rounded-lg border border-emerald-200 text-emerald-700 hover:bg-emerald-50 disabled:opacity-50"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
        </svg>
        {m.admin_stock_mutations_import_in()}
      </button>
      <button
        type="button"
        onclick={() => openImportPicker('out')}
        disabled={importing}
        class="inline-flex items-center gap-1.5 px-3 py-2 text-sm rounded-lg border border-red-200 text-red-700 hover:bg-red-50 disabled:opacity-50"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
        </svg>
        {m.admin_stock_mutations_import_out()}
      </button>
      <a
        href="/admin/stock-mutations/inventory.csv"
        target="_blank"
        rel="noopener"
        class="inline-flex items-center gap-1.5 px-3 py-2 text-sm rounded-lg border border-gray-200 text-gray-700 hover:bg-gray-50"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
        </svg>
        {m.admin_stock_mutations_export_inventory()}
      </a>
      <NewButton label={m.admin_stock_mutations_new()} href="/admin/stock-mutations/new" />
    </div>
  </div>

  <!-- Shared hidden file input + two hidden forms for importIn/importOut.
       Clicking either Import button populates importDirection then triggers
       this picker; on change we copy files to the right form and submit. -->
  <input
    type="file"
    accept=".csv,text/csv"
    class="hidden"
    bind:this={fileInputEl}
    onchange={onFilePicked}
  />
  <form
    bind:this={importInFormEl}
    method="POST"
    action="?/importIn"
    enctype="multipart/form-data"
    class="hidden"
    use:enhance={() => {
      importing = true;
      return async ({ result, update }) => {
        importing = false;
        if (fileInputEl) fileInputEl.value = '';
        await handleImportResult(result);
        await update();
      };
    }}
  >
    <input type="file" name="file" />
  </form>
  <form
    bind:this={importOutFormEl}
    method="POST"
    action="?/importOut"
    enctype="multipart/form-data"
    class="hidden"
    use:enhance={() => {
      importing = true;
      return async ({ result, update }) => {
        importing = false;
        if (fileInputEl) fileInputEl.value = '';
        await handleImportResult(result);
        await update();
      };
    }}
  >
    <input type="file" name="file" />
  </form>

  {#if importErrors && importErrors.length > 0}
    <div class="bg-amber-50 border border-amber-200 rounded-xl p-3 text-xs text-amber-800 space-y-1">
      <div class="font-medium">{m.admin_stock_mutations_import_errors_heading({ n: String(importErrors.length) })}</div>
      <ul class="list-disc pl-5 space-y-0.5">
        {#each importErrors.slice(0, 20) as e}
          <li>Row {e.row}: {e.message}</li>
        {/each}
        {#if importErrors.length > 20}
          <li>… and {importErrors.length - 20} more</li>
        {/if}
      </ul>
    </div>
  {/if}

  <!-- Filters -->
  <div class="bg-white border border-gray-200 rounded-xl p-4 space-y-3">
    <div class="flex flex-wrap gap-3 items-center">
      <div class="flex-1 min-w-[240px]">
        <SearchInput value={data.q} placeholder={m.admin_stock_mutations_search_placeholder()} onChange={onSearch} />
      </div>

      <div class="inline-flex rounded-lg border border-gray-200 overflow-hidden text-xs">
        <button class="px-3 py-1.5 {data.status === '' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => setStatus('')}>{m.admin_stock_mutations_filter_all()}</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {data.status === 'draft' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => setStatus('draft')}>{m.admin_stock_mutations_status_draft()}</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {data.status === 'executed' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => setStatus('executed')}>{m.admin_stock_mutations_status_executed()}</button>
      </div>

      <div class="inline-flex rounded-lg border border-gray-200 overflow-hidden text-xs">
        <button class="px-3 py-1.5 {data.type === '' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => setType('')}>{m.admin_stock_mutations_filter_all_types()}</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {data.type === 'in' ? 'bg-emerald-600 text-white' : 'bg-white text-emerald-700 hover:bg-gray-50'}" onclick={() => setType('in')}>{m.admin_stock_mutations_type_in()}</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {data.type === 'out' ? 'bg-red-600 text-white' : 'bg-white text-red-700 hover:bg-gray-50'}" onclick={() => setType('out')}>{m.admin_stock_mutations_type_out()}</button>
      </div>

      <select value={data.createdBy}
              onchange={(e) => setCreator((e.currentTarget as HTMLSelectElement).value)}
              class="text-xs border border-gray-200 rounded-lg px-2 py-1.5 bg-white max-w-[200px]"
              aria-label={m.admin_stock_mutations_filter_creator()}>
        <option value="">{m.admin_stock_mutations_creator_all()}</option>
        {#each data.creators as c (c.id)}
          <option value={c.id}>{c.email}</option>
        {/each}
      </select>

      <div class="inline-flex rounded-lg border border-gray-200 overflow-hidden text-xs">
        <button class="px-3 py-1.5 {activePreset === 'all' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => applyPreset('all')}>{m.admin_stock_mutations_date_all()}</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {activePreset === 'this_week' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => applyPreset('this_week')}>{m.admin_stock_mutations_date_this_week()}</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {activePreset === 'this_month' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => applyPreset('this_month')}>{m.admin_stock_mutations_date_this_month()}</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {activePreset === 'last_week' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => applyPreset('last_week')}>{m.admin_stock_mutations_date_last_week()}</button>
        <button class="px-3 py-1.5 border-l border-gray-200 {activePreset === 'last_month' ? 'bg-gray-900 text-white' : 'bg-white text-gray-700 hover:bg-gray-50'}" onclick={() => applyPreset('last_month')}>{m.admin_stock_mutations_date_last_month()}</button>
      </div>

      <input type="date" value={data.from} onchange={(e) => setDate('from', (e.currentTarget as HTMLInputElement).value)}
             class="text-xs border border-gray-200 rounded-lg px-2 py-1.5" aria-label="from date" />
      <span class="text-gray-400 text-xs">→</span>
      <input type="date" value={data.to} onchange={(e) => setDate('to', (e.currentTarget as HTMLInputElement).value)}
             class="text-xs border border-gray-200 rounded-lg px-2 py-1.5" aria-label="to date" />

      {#if hasFilters}
        <button onclick={clearAll} class="text-xs text-gray-500 underline hover:text-gray-700">{m.admin_stock_mutations_filter_clear()}</button>
      {/if}
    </div>
  </div>

  <!-- Table -->
  <div class="bg-white border border-gray-200 rounded-xl overflow-hidden">
    <div class="overflow-x-auto">
      <table class="w-full text-sm">
        <thead class="bg-gray-50 text-left text-xs uppercase tracking-wide text-gray-500">
          <tr>
            <th class="px-3 py-2 w-10">
              <input
                type="checkbox"
                class="rounded border-gray-300"
                checked={allCombinableSelected}
                disabled={combinableRows.length === 0}
                onchange={toggleSelectAll}
                aria-label={m.admin_stock_mutations_combine_select_aria()}
              />
            </th>
            <th class="px-4 py-2 font-medium">{m.admin_stock_mutations_col_number()}</th>
            <th class="px-4 py-2 font-medium">{m.admin_stock_mutations_col_type()}</th>
            <th class="px-4 py-2 font-medium">{m.admin_stock_mutations_col_status()}</th>
            <th class="px-4 py-2 font-medium text-right">{m.admin_stock_mutations_col_items()}</th>
            <th class="px-4 py-2 font-medium text-right">{m.admin_stock_mutations_col_total_qty()}</th>
            <th class="px-4 py-2 font-medium">{m.admin_stock_mutations_col_created()}</th>
            <th class="px-4 py-2 font-medium">{m.admin_stock_mutations_col_executed()}</th>
            <th class="px-4 py-2 font-medium text-right">{m.admin_stock_mutations_col_actions()}</th>
          </tr>
        </thead>
        <tbody>
          {#if data.list.items.length === 0}
            <tr>
              <td colspan="9" class="px-4 py-10 text-center text-sm text-gray-400">
                {hasFilters ? m.admin_stock_mutations_empty_with_filters() : m.admin_stock_mutations_empty_no_filters()}
              </td>
            </tr>
          {:else}
            {#each data.list.items as row (row.id)}
              <tr class="border-t border-gray-100 hover:bg-gray-50">
                <td class="px-3 py-2">
                  {#if row.consumed_by_order_id}
                    <a href="/admin/orders/{row.consumed_by_order_id}"
                       class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium bg-violet-50 text-violet-700 hover:bg-violet-100"
                       title={m.admin_stock_mutations_combine_consumed_title()}>
                      {m.admin_stock_mutations_combine_consumed_badge()}
                    </a>
                  {:else if isCombinable(row)}
                    <input
                      type="checkbox"
                      class="rounded border-gray-300"
                      checked={selectedIds.includes(row.id)}
                      onchange={() => toggleSelect(row.id)}
                      aria-label={m.admin_stock_mutations_combine_select_aria()}
                    />
                  {/if}
                </td>
                <td class="px-4 py-2 font-mono text-sm">
                  <a href="/admin/stock-mutations/{row.id}" class="text-gray-900 hover:underline">{row.mutation_number}</a>
                </td>
                <td class="px-4 py-2">
                  {#if row.type === 'in'}
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-emerald-50 text-emerald-700">{m.admin_stock_mutations_type_in()}</span>
                  {:else}
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-red-50 text-red-700">{m.admin_stock_mutations_type_out()}</span>
                  {/if}
                </td>
                <td class="px-4 py-2">
                  {#if row.status === 'draft'}
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-amber-50 text-amber-700">{m.admin_stock_mutations_status_draft()}</span>
                  {:else}
                    <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-50 text-blue-700">{m.admin_stock_mutations_status_executed()}</span>
                  {/if}
                </td>
                <td class="px-4 py-2 text-right tabular-nums">{row.item_count}</td>
                <td class="px-4 py-2 text-right tabular-nums">{row.total_quantity}</td>
                <td class="px-4 py-2 text-xs text-gray-500">
                  <div>{fmtDateTime(row.created_at)}</div>
                  {#if row.created_by_email}<div class="text-gray-400">{row.created_by_email}</div>{/if}
                </td>
                <td class="px-4 py-2 text-xs text-gray-500">
                  {#if row.executed_at}
                    <div>{fmtDateTime(row.executed_at)}</div>
                    {#if row.executed_by_email}<div class="text-gray-400">{row.executed_by_email}</div>{/if}
                  {:else}
                    <span class="text-gray-300">—</span>
                  {/if}
                </td>
                <td class="px-4 py-2 text-right">
                  <div class="flex items-center justify-end gap-1">
                    <a href="/admin/stock-mutations/{row.id}"
                       class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
                       title={row.status === 'draft' ? m.admin_stock_mutations_action_edit() : m.admin_stock_mutations_action_view()}>
                      {#if row.status === 'draft'}
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                          <path stroke-linecap="round" stroke-linejoin="round"
                            d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Z"/>
                        </svg>
                      {:else}
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                          <path stroke-linecap="round" stroke-linejoin="round"
                            d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.964-7.178Z"/>
                          <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/>
                        </svg>
                      {/if}
                    </a>
                    {#if row.status === 'draft'}
                      <button class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                              disabled={executingId === row.id}
                              onclick={() => (executeTarget = row)}
                              title={m.admin_stock_mutations_action_execute()}>
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                          <path stroke-linecap="round" stroke-linejoin="round"
                            d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 0 1 0 1.972l-11.54 6.347a1.125 1.125 0 0 1-1.667-.986V5.653Z"/>
                        </svg>
                      </button>
                      <button class="p-1.5 rounded-lg text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors"
                              onclick={() => (deleteTarget = row)}
                              title={m.admin_stock_mutations_action_delete()}>
                        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                          <path stroke-linecap="round" stroke-linejoin="round"
                            d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"/>
                        </svg>
                      </button>
                    {/if}
                    <button class="p-1.5 rounded-lg text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                            disabled={duplicatingId === row.id}
                            onclick={() => duplicate(row)}
                            title={m.admin_stock_mutations_action_duplicate()}>
                      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                        <path stroke-linecap="round" stroke-linejoin="round"
                          d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 0 1-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 0 1 1.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 0 0-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 0 1-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 0 0-3.375-3.375h-1.5a1.125 1.125 0 0 1-1.125-1.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H9.75"/>
                      </svg>
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>

  <Pagination total={data.list.total} pageSize={data.pageSize} currentPage={data.page} />
</div>

<!-- Sticky action bar — appears once at least one out-mutation is selected. -->
{#if selectedIds.length > 0}
  <div class="fixed bottom-4 left-1/2 -translate-x-1/2 z-30 flex items-center gap-3 bg-gray-900 text-white rounded-full shadow-xl pl-5 pr-2 py-2">
    <span class="text-sm">{selectedIds.length}</span>
    <button
      type="button"
      onclick={openCombine}
      class="inline-flex items-center gap-1.5 px-4 py-1.5 text-sm font-medium rounded-full bg-white text-gray-900 hover:bg-gray-100"
    >
      {m.admin_stock_mutations_combine_btn({ n: String(selectedIds.length) })}
    </button>
    <button
      type="button"
      onclick={() => (selectedIds = [])}
      class="p-1.5 rounded-full text-gray-300 hover:text-white hover:bg-white/10"
      aria-label={m.admin_stock_mutations_cancel()}
    >
      <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
        <path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
      </svg>
    </button>
  </div>
{/if}

{#if combineOpen}
  <div class="fixed inset-0 z-40 bg-black/40 flex items-center justify-center p-4" role="dialog" aria-modal="true">
    <div class="bg-white rounded-xl shadow-xl w-full max-w-lg p-5 space-y-4 max-h-[90vh] overflow-y-auto">
      <h2 class="text-lg font-semibold">{m.admin_stock_mutations_combine_modal_title()}</h2>
      <p class="text-sm text-gray-600">{m.admin_stock_mutations_combine_modal_desc({ n: String(selectedIds.length) })}</p>

      <div class="space-y-1.5">
        <label class="block text-sm font-medium text-gray-700" for="combine-customer">{m.admin_stock_mutations_combine_customer_label()}</label>
        {#if data.token}
          <CustomerPicker token={data.token} bind:value={combineCustomer} />
        {/if}
      </div>

      {#if combineCustomer.kind === 'existing'}
        {#if combineCustomer.addresses.length > 0}
          <div class="space-y-1.5">
            <label class="block text-sm font-medium text-gray-700" for="combine-address">{m.admin_stock_mutations_combine_address_label()}</label>
            <select id="combine-address" bind:value={combineAddressId}
                    class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 bg-white">
              {#each combineCustomer.addresses as a (a.id)}
                <option value={a.id}>{a.first_name} {a.last_name} · {a.line1}{a.is_default ? ' ·★' : ''}</option>
              {/each}
            </select>
          </div>
        {:else}
          <p class="text-sm text-amber-700 bg-amber-50 border border-amber-200 rounded-lg p-2.5">{m.admin_stock_mutations_combine_no_address()}</p>
        {/if}
      {/if}

      <div class="grid grid-cols-2 gap-3">
        <div class="space-y-1.5">
          <label class="block text-sm font-medium text-gray-700" for="combine-status">{m.admin_stock_mutations_combine_status_label()}</label>
          <select id="combine-status" bind:value={combineStatus}
                  class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2 bg-white">
            <option value="pending">{m.admin_stock_mutations_combine_status_pending()}</option>
            <option value="processing">{m.admin_stock_mutations_combine_status_processing()}</option>
          </select>
        </div>
        <div class="space-y-1.5">
          <label class="block text-sm font-medium text-gray-700" for="combine-coupon">{m.admin_stock_mutations_combine_coupon_label()}</label>
          <input id="combine-coupon" bind:value={combineCoupon} type="text"
                 class="w-full text-sm border border-gray-200 rounded-lg px-3 py-2" />
        </div>
      </div>

      <div class="flex justify-end gap-2 pt-1">
        <button class="px-3 py-1.5 text-sm rounded-lg border border-gray-200 hover:bg-gray-50"
                onclick={() => (combineOpen = false)} disabled={combineSubmitting}>{m.admin_stock_mutations_cancel()}</button>
        <button class="px-3 py-1.5 text-sm rounded-lg bg-gray-900 text-white hover:bg-gray-800 disabled:opacity-50"
                onclick={submitCombine} disabled={!combineReady || combineSubmitting}>
          {combineSubmitting ? '…' : m.admin_stock_mutations_combine_confirm()}
        </button>
      </div>
    </div>
  </div>
{/if}

{#if deleteTarget}
  <div class="fixed inset-0 z-40 bg-black/40 flex items-center justify-center p-4" role="dialog" aria-modal="true">
    <div class="bg-white rounded-xl shadow-xl w-full max-w-md p-5 space-y-4">
      <h2 class="text-lg font-semibold">{m.admin_stock_mutations_delete_modal_title({ id: deleteTarget.mutation_number })}</h2>
      <p class="text-sm text-gray-600">{m.admin_stock_mutations_delete_modal_body()}</p>
      <div class="flex justify-end gap-2">
        <button class="px-3 py-1.5 text-sm rounded-lg border border-gray-200 hover:bg-gray-50"
                onclick={() => (deleteTarget = null)} disabled={deleting}>{m.admin_stock_mutations_cancel()}</button>
        <button class="px-3 py-1.5 text-sm rounded-lg bg-red-600 text-white hover:bg-red-700 disabled:opacity-50"
                onclick={confirmDelete} disabled={deleting}>{deleting ? '…' : m.admin_stock_mutations_confirm_delete_btn()}</button>
      </div>
    </div>
  </div>
{/if}

{#if executeTarget}
  {@const busy = executingId === executeTarget.id}
  <div class="fixed inset-0 z-40 bg-black/40 flex items-center justify-center p-4" role="dialog" aria-modal="true">
    <div class="bg-white rounded-xl shadow-xl w-full max-w-md p-5 space-y-4">
      <h2 class="text-lg font-semibold">{m.admin_stock_mutations_execute_modal_title({ id: executeTarget.mutation_number })}</h2>
      <p class="text-sm text-gray-600">{m.admin_stock_mutations_execute_modal_body()}</p>
      <div class="flex justify-end gap-2">
        <button class="px-3 py-1.5 text-sm rounded-lg border border-gray-200 hover:bg-gray-50"
                onclick={() => (executeTarget = null)} disabled={busy}>{m.admin_stock_mutations_cancel()}</button>
        <button class="px-3 py-1.5 text-sm rounded-lg bg-emerald-600 text-white hover:bg-emerald-700 disabled:opacity-50"
                onclick={confirmExecute} disabled={busy}>{busy ? '…' : m.admin_stock_mutations_confirm_execute_btn()}</button>
      </div>
    </div>
  </div>
{/if}
