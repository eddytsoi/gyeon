import { getMyProfile, getMyAddresses, getPaymentConfig, getPublicSettings, getMySavedCards, getShippingDefault, getCmsPageBySlug } from '$lib/api';
import { scanShortcodeRefs } from '$lib/shortcodes/scan';
import { resolveShortcodeRefs } from '$lib/shortcodes/resolve';
import type { PageServerLoad } from './$types';

function parseShippingCountries(raw: string | undefined): string[] {
  if (!raw) return ['HK'];
  try {
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed)) {
      const codes = parsed.filter((v): v is string => typeof v === 'string');
      return codes.length > 0 ? codes : ['HK'];
    }
  } catch {
    /* fall through */
  }
  return ['HK'];
}

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('customer_token') ?? null;
  const [paymentConfig, settings, shippingDefault, termsPage] = await Promise.all([
    getPaymentConfig().catch(() => ({ publishable_key: '', mode: 'test' as const })),
    getPublicSettings().catch(() => []),
    getShippingDefault().catch(() => ({ configured: false })),
    getCmsPageBySlug('terms-and-conditions').catch(() => null)
  ]);
  const shippingCountries = parseShippingCountries(
    settings.find((s) => s.key === 'shipping_countries')?.value
  );
  const saveCardsEnabled = settings.find((s) => s.key === 'stripe_save_cards')?.value === 'true';
  const shipanyEnabled = settings.find((s) => s.key === 'shipany_enabled')?.value === 'true';
  const termsRefs = termsPage
    ? await resolveShortcodeRefs(scanShortcodeRefs(termsPage.content)).catch(() => null)
    : null;

  if (!token) {
    return {
      token: null, customer: null, addresses: [], savedCards: [],
      saveCardsEnabled, paymentConfig, shippingCountries, shipanyEnabled, shippingDefault,
      termsPage, termsRefs
    };
  }

  try {
    const [customer, addresses, savedCards] = await Promise.all([
      getMyProfile(token),
      getMyAddresses(token),
      saveCardsEnabled ? getMySavedCards(token).catch(() => []) : Promise.resolve([])
    ]);
    return {
      token, customer, addresses, savedCards,
      saveCardsEnabled, paymentConfig, shippingCountries, shipanyEnabled, shippingDefault,
      termsPage, termsRefs
    };
  } catch {
    return {
      token: null, customer: null, addresses: [], savedCards: [],
      saveCardsEnabled, paymentConfig, shippingCountries, shipanyEnabled, shippingDefault,
      termsPage, termsRefs
    };
  }
};
