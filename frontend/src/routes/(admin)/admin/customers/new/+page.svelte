<script lang="ts">
  import { enhance } from '$app/forms';
  import type { ActionData } from './$types';
  import SaveButton from '$lib/components/admin/SaveButton.svelte';
  import * as m from '$lib/paraglide/messages';
  import type { CustomerRole } from '$lib/types';

  let { form }: { form: ActionData } = $props();

  let saving = $state(false);
  // Field state survives an enhance re-render after a failed submit, so the
  // admin never loses what they typed. role/checkbox keep their own drafts.
  let roleDraft = $state<CustomerRole>('customer');
  let sendSetupEmail = $state(true);

  const errorMessage = $derived(
    form?.error === 'email_taken'
      ? m.admin_customer_new_email_taken()
      : form?.error === 'invalid_email'
        ? m.admin_customer_new_invalid_email()
        : form?.error === 'missing_first_name'
          ? m.admin_customer_new_missing_first_name()
          : form?.error
            ? m.admin_customer_new_error()
            : null
  );

  const inputClass =
    'w-full text-sm border border-gray-200 rounded-lg px-3 py-2 bg-white focus:outline-none focus:ring-2 focus:ring-gray-900/10';
  const labelClass =
    'block text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1';
</script>

<svelte:head><title>{m.admin_customer_new()}</title></svelte:head>

<div class="max-w-2xl">
  <div class="flex items-center gap-3 mb-6">
    <a href="/admin/customers" class="text-sm text-gray-400 hover:text-gray-700 transition-colors">{m.admin_customer_back()}</a>
    <span class="text-gray-200">/</span>
    <h1 class="text-xl font-bold text-gray-900">{m.admin_customer_new()}</h1>
  </div>

  {#if errorMessage}
    <div class="mb-4 px-4 py-3 bg-red-50 border border-red-100 rounded-xl text-sm text-red-600">
      {errorMessage}
    </div>
  {/if}

  <form
    method="POST"
    use:enhance={() => {
      saving = true;
      return async ({ update }) => {
        await update();
        saving = false;
      };
    }}
    class="bg-white rounded-2xl border border-gray-100 p-6"
  >
    <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
      <div>
        <label for="first_name" class={labelClass}>{m.admin_customer_new_first_name()}</label>
        <input id="first_name" name="first_name" type="text" required autocomplete="off" class={inputClass} />
      </div>
      <div>
        <label for="last_name" class={labelClass}>{m.admin_customer_new_last_name()}</label>
        <input id="last_name" name="last_name" type="text" autocomplete="off" class={inputClass} />
      </div>
      <div>
        <label for="email" class={labelClass}>{m.admin_customer_label_email()}</label>
        <input id="email" name="email" type="email" required autocomplete="off" class={inputClass} />
      </div>
      <div>
        <label for="phone" class={labelClass}>{m.admin_customer_label_phone()}</label>
        <input id="phone" name="phone" type="tel" autocomplete="off" class={inputClass} />
      </div>
      <div>
        <label for="role" class={labelClass}>{m.admin_customer_label_role()}</label>
        <select id="role" name="role" bind:value={roleDraft} class={inputClass}>
          <option value="customer">{m.admin_role_customer()}</option>
          <option value="installer">{m.admin_role_installer()}</option>
          <option value="installer_v2">{m.admin_role_installer_v2()}</option>
        </select>
      </div>
    </div>

    <label class="mt-5 pt-5 border-t border-gray-100 flex items-start gap-3 cursor-pointer">
      <input
        type="checkbox"
        name="send_setup_email"
        bind:checked={sendSetupEmail}
        class="mt-0.5 h-4 w-4 rounded border-gray-300 text-gray-900 focus:ring-gray-900/20"
      />
      <span>
        <span class="block text-sm font-medium text-gray-900">{m.admin_customer_new_send_setup_email()}</span>
        <span class="block text-xs text-gray-400 mt-0.5">{m.admin_customer_new_send_setup_email_hint()}</span>
      </span>
    </label>

    <div class="mt-6 flex items-center gap-3">
      <SaveButton
        loading={saving}
        class="inline-flex items-center justify-center gap-1.5 px-5 py-2 text-sm font-semibold text-white bg-gray-900 rounded-lg hover:bg-gray-700 disabled:opacity-50"
      >
        {m.admin_customer_new_submit()}
      </SaveButton>
      <a
        href="/admin/customers"
        class="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-200 rounded-lg hover:bg-gray-50 transition-colors"
      >
        {m.admin_customer_reset_modal_cancel()}
      </a>
    </div>
  </form>
</div>
