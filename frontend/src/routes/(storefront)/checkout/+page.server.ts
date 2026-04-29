import { getMyProfile, getMyAddresses, getPaymentConfig, getPublicSettings, getMySavedCards } from '$lib/api';
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
  const [paymentConfig, settings] = await Promise.all([
    getPaymentConfig().catch(() => ({ publishable_key: '', mode: 'test' as const })),
    getPublicSettings().catch(() => [])
  ]);
  const shippingCountries = parseShippingCountries(
    settings.find((s) => s.key === 'shipping_countries')?.value
  );
  const saveCardsEnabled = settings.find((s) => s.key === 'stripe_save_cards')?.value === 'true';
  const shipanyEnabled = settings.find((s) => s.key === 'shipany_enabled')?.value === 'true';

  if (!token) {
    return {
      token: null, customer: null, addresses: [], savedCards: [],
      saveCardsEnabled, paymentConfig, shippingCountries, shipanyEnabled
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
      saveCardsEnabled, paymentConfig, shippingCountries, shipanyEnabled
    };
  } catch {
    return {
      token: null, customer: null, addresses: [], savedCards: [],
      saveCardsEnabled, paymentConfig, shippingCountries, shipanyEnabled
    };
  }
};
