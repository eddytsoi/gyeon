/*
 * Bank-transfer (銀行轉賬 / BACS) helpers.
 *
 * Bank transfer is the only payment method offered to installer / installer_v2
 * customers; regular customers + guests always pay by Stripe. The account
 * details are admin-editable site settings; the static labels live in the
 * storefront i18n messages and are rendered by BankTransferNotice.svelte.
 */
import type { SiteSetting } from './shippingThreshold';

export interface BankTransferDetails {
  enabled: boolean;
  accountName: string;
  bankName: string;
  accountNumber: string;
  whatsappDisplay: string;
  whatsappUrl: string;
}

/** True when the customer's role is restricted to bank transfer. */
export function isBankTransferRole(role: string | null | undefined): boolean {
  return role === 'installer' || role === 'installer_v2';
}

/** Pull the admin-configured bank-transfer account details out of the public
 * settings array. Missing keys resolve to empty strings. */
export function resolveBankTransfer(
  settings: ReadonlyArray<SiteSetting>
): BankTransferDetails {
  const get = (key: string) => settings.find((s) => s.key === key)?.value ?? '';
  return {
    enabled: get('bank_transfer_enabled') !== 'false',
    accountName: get('bank_transfer_account_name'),
    bankName: get('bank_transfer_bank_name'),
    accountNumber: get('bank_transfer_account_number'),
    whatsappDisplay: get('bank_transfer_whatsapp_display'),
    whatsappUrl: get('bank_transfer_whatsapp_url')
  };
}
