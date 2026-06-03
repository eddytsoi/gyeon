<script lang="ts">
  // WooCommerce-style bank-transfer (銀行轉賬 / BACS) notice. Shown to installer /
  // installer_v2 customers — the only payment method available to them — on the
  // checkout payment section, the order-confirmation page, and the account order
  // detail page. The static labels are i18n messages; the account details come
  // from admin-editable site settings (resolved upstream into `details`).
  import * as m from '$lib/paraglide/messages';
  import type { BankTransferDetails } from '$lib/bankTransfer';

  let {
    details,
    variant = 'radio'
  }: { details: BankTransferDetails; variant?: 'radio' | 'plain' } = $props();

  // Inject the WhatsApp number into the i18n instruction as an inline link so
  // the wording — and the number's position within it — stays correct across
  // locales. Rendered via {@html}, mirroring the checkout_success_body pattern.
  const whatsappLink = $derived(
    details.whatsappUrl
      ? `<a href="${details.whatsappUrl}" target="_blank" rel="noopener" class="font-medium text-gray-900 underline">${details.whatsappDisplay}</a>`
      : `<span class="font-medium text-gray-900">${details.whatsappDisplay}</span>`
  );
</script>

{#if variant === 'radio'}
  <div class="flex items-center gap-3 rounded-xl border border-gray-200 px-4 py-3.5">
    <span
      class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border-2 border-gray-900"
      aria-hidden="true"
    >
      <span class="h-2.5 w-2.5 rounded-full bg-gray-900"></span>
    </span>
    <span class="font-semibold text-gray-900">{m.bank_transfer_radio_label()}</span>
  </div>
{/if}

<div
  class="rounded-xl border border-gray-100 bg-gray-50 px-4 py-4 text-sm leading-relaxed text-gray-700 {variant ===
  'radio'
    ? 'mt-3'
    : ''}"
>
  <p>{m.bank_transfer_intro()}</p>

  <p class="mt-3 font-medium text-gray-900">{m.bank_transfer_details_heading()}</p>
  <p class="mt-0.5">{m.bank_transfer_field_name()}：{details.accountName}</p>
  <p>{m.bank_transfer_field_bank()}：{details.bankName}</p>
  <p class="tabular-nums">{m.bank_transfer_field_account()}：{details.accountNumber}</p>

  <p class="mt-3">{@html m.bank_transfer_whatsapp_instruction({ number: whatsappLink })}</p>
  {#if details.whatsappUrl}
    <p class="mt-0.5">
      <a
        href={details.whatsappUrl}
        target="_blank"
        rel="noopener"
        class="break-all text-gray-500 underline hover:text-gray-900">{details.whatsappUrl}</a
      >
    </p>
  {/if}
</div>
