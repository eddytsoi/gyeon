/*
 * Role-aware free-shipping threshold resolver.
 *
 * Picks between the default (guest + role=customer) and installer (施工店)
 * settings pairs. No fallback — when the resolved role's enabled flag is
 * false or amount is 0, that role always pays shipping.
 */
export interface SiteSetting {
  key: string;
  value: string;
}

export interface FreeShippingThreshold {
  enabled: boolean;
  threshold: number;
}

export function resolveFreeShippingThreshold(
  settings: ReadonlyArray<SiteSetting>,
  role: string | null | undefined
): FreeShippingThreshold {
  const isInstaller = role === 'installer';
  const enabledKey = isInstaller
    ? 'free_shipping_threshold_installer_enabled'
    : 'free_shipping_threshold_enabled';
  const amountKey = isInstaller
    ? 'free_shipping_threshold_installer_hkd'
    : 'free_shipping_threshold_hkd';

  const enabled = settings.find((s) => s.key === enabledKey)?.value === 'true';
  const raw = settings.find((s) => s.key === amountKey)?.value;
  const n = raw ? Number(raw) : 0;
  const threshold = Number.isFinite(n) && n > 0 ? n : 0;

  return { enabled, threshold };
}
