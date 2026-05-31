/** Storefront prices render as whole HK$ (no decimals), matching the live
 *  WooCommerce site's 0-decimal display. Admin surfaces keep full precision. */
export const formatHKD = (n: number): string => `HK$${Math.round(n)}`;

/** Rounded amount only (no prefix) — for i18n message params that supply
 *  their own currency text. */
export const roundAmount = (n: number): number => Math.round(n);
