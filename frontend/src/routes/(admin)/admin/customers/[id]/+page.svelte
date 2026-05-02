<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData, ActionData } from './$types';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';

  let { data, form }: { data: PageData; form: ActionData } = $props();

  let confirmOpen = $state(false);
  let sending = $state(false);
  let successMessage = $state<string | null>(null);

  function openConfirm() {
    successMessage = null;
    confirmOpen = true;
  }

  function closeConfirm() {
    if (sending) return;
    confirmOpen = false;
  }

  const statusColour: Record<string, string> = {
    pending:    'bg-amber-50 text-amber-700',
    paid:       'bg-blue-50 text-blue-700',
    processing: 'bg-indigo-50 text-indigo-700',
    shipped:    'bg-violet-50 text-violet-700',
    delivered:  'bg-green-50 text-green-700',
    cancelled:  'bg-gray-100 text-gray-500',
    refunded:   'bg-red-50 text-red-700',
  };
</script>

<svelte:head>
  <title>{data.customer ? `${data.customer.first_name} ${data.customer.last_name}` : 'Customer'} — Gyeon Admin</title>
</svelte:head>

<div class="max-w-4xl">
  <div class="flex items-center gap-3 mb-6">
    <a href="/admin/customers" class="text-sm text-gray-400 hover:text-gray-700 transition-colors">← Customers</a>
    <span class="text-gray-200">/</span>
    <h1 class="text-xl font-bold text-gray-900">
      {data.customer ? `${data.customer.first_name} ${data.customer.last_name}` : 'Customer Not Found'}
    </h1>
  </div>

  {#if !data.customer}
    <div class="bg-white rounded-2xl border border-gray-100 p-8 text-center text-gray-400">
      Customer not found.
    </div>
  {:else}
    {#if form?.resetSent || successMessage}
      <div class="mb-4 px-4 py-3 bg-green-50 border border-green-100 rounded-xl text-sm text-green-700 flex items-center justify-between">
        <span>{successMessage ?? `已寄出 reset password email 至 ${data.customer.email}`}</span>
        <button class="text-green-700/60 hover:text-green-700 text-lg leading-none" onclick={() => { successMessage = null; }} aria-label="Dismiss">×</button>
      </div>
    {/if}
    {#if form?.resetError}
      <div class="mb-4 px-4 py-3 bg-red-50 border border-red-100 rounded-xl text-sm text-red-600">
        寄送失敗：{form.resetError}
      </div>
    {/if}

    <!-- Profile Card -->
    <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-6">
      <div class="flex items-center justify-between mb-4">
        <h2 class="font-semibold text-gray-900">Profile</h2>
        <button
          type="button"
          onclick={openConfirm}
          class="px-3 py-1.5 text-xs font-medium text-gray-700 bg-white border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors"
        >
          Reset password
        </button>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1">Name</p>
          <p class="text-sm text-gray-900">{data.customer.first_name} {data.customer.last_name}</p>
        </div>
        <div>
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1">Email</p>
          <p class="text-sm text-gray-900">{data.customer.email}</p>
        </div>
        {#if data.customer.phone}
          <div>
            <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1">Phone</p>
            <p class="text-sm text-gray-900">{data.customer.phone}</p>
          </div>
        {/if}
        <div>
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1">Status</p>
          <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                       {data.customer.is_active ? 'bg-green-50 text-green-700' : 'bg-gray-100 text-gray-500'}">
            {data.customer.is_active ? 'Active' : 'Inactive'}
          </span>
        </div>
        <div>
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1">Member Since</p>
          <p class="text-sm text-gray-900">{new Date(data.customer.created_at).toLocaleDateString('en-HK')}</p>
        </div>
      </div>
    </div>

    <!-- Order History -->
    <div class="bg-white rounded-2xl border border-gray-100 overflow-hidden">
      <div class="px-6 py-4 border-b border-gray-100">
        <h2 class="font-semibold text-gray-900">Order History</h2>
      </div>
      {#if data.orders.length === 0}
        <div class="px-6 py-8 text-center text-gray-400 text-sm">No orders yet.</div>
      {:else}
        <table class="w-full text-sm">
          <thead class="bg-gray-50 border-b border-gray-100">
            <tr>
              <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">Order ID</th>
              <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">Status</th>
              <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide">Total</th>
              <th class="text-left px-5 py-3 text-xs font-semibold text-gray-400 uppercase tracking-wide hidden sm:table-cell">Date</th>
              <th class="px-5 py-3"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-50">
            {#each data.orders as order}
              <tr class="hover:bg-gray-50 transition-colors">
                <td class="px-5 py-3 font-mono text-xs text-gray-500">{order.id.slice(0, 8)}…</td>
                <td class="px-5 py-3">
                  <span class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium
                               {statusColour[order.status] ?? 'bg-gray-100 text-gray-600'}">
                    {order.status}
                  </span>
                </td>
                <td class="px-5 py-3 font-medium text-gray-900">
                  HK${(order.total ?? 0).toLocaleString('en-HK')}
                </td>
                <td class="px-5 py-3 text-gray-400 text-xs hidden sm:table-cell">
                  {new Date(order.created_at).toLocaleDateString('en-HK')}
                </td>
                <td class="px-5 py-3 text-right">
                  <a href="/admin/orders/{order.id}"
                     class="text-xs font-medium text-gray-600 hover:text-gray-900 transition-colors">
                    View
                  </a>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>

    {#if confirmOpen}
      <div
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4"
        onclick={closeConfirm}
        onkeydown={(e) => { if (e.key === 'Escape') closeConfirm(); }}
        role="presentation"
      >
        <div
          class="bg-white rounded-2xl shadow-xl max-w-sm w-full p-6"
          onclick={(e) => e.stopPropagation()}
          role="dialog"
          aria-modal="true"
          aria-labelledby="reset-pw-title"
          tabindex="-1"
        >
          <h3 id="reset-pw-title" class="font-semibold text-gray-900 mb-2">Reset password</h3>
          <p class="text-sm text-gray-600 mb-5">
            確定寄出 reset password email 俾
            <span class="font-medium text-gray-900">{data.customer.email}</span>
            ？連結將於 24 小時後失效。
          </p>
          <form
            method="POST"
            action="?/sendResetPassword"
            use:enhance={() => {
              if (sending) return;
              sending = true;
              return async ({ update }) => {
                await update();
                sending = false;
                confirmOpen = false;
              };
            }}
            class="flex justify-end gap-2"
          >
            <button
              type="button"
              onclick={closeConfirm}
              disabled={sending}
              class="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-200 rounded-lg hover:bg-gray-50 disabled:opacity-50"
            >
              Cancel
            </button>
            <SaveButton
              loading={sending}
              class="inline-flex items-center justify-center gap-1.5 px-4 py-2 text-sm font-semibold text-white bg-gray-900 rounded-lg hover:bg-gray-700 disabled:opacity-50"
            >
              Send
            </SaveButton>
          </form>
        </div>
      </div>
    {/if}
  {/if}
</div>
