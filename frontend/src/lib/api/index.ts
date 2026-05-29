import type { Address, BundleItem, Cart, CartItem, Category, CheckoutResult, CmsPage, CmsPost, Customer, CustomerInfoInput, NavMenu, Order, OrderNotice, PaymentConfig, Product, ProductImage, PromoBundle, SavedPaymentMethod, ShippingAddressInput, Variant } from '$lib/types';

const base = () =>
  typeof window === 'undefined'
    ? (process.env.API_BASE ?? 'http://localhost:8080/api/v1')
    : '/api/v1';

// SSR fetches go intra-Docker to the backend; 8s is a generous ceiling that
// still keeps `+page.server.ts` Promise.all fan-outs under SvelteKit's request
// budget. Browser-side: 15s is past mobile-user patience but covers the
// occasional cold-DB or restart-window blip without showing a 500.
// Without this, the .catch(() => null) idiom used throughout load functions
// is useless: a hung fetch never rejects, so the page hangs indefinitely.
const DEFAULT_TIMEOUT_MS = typeof window === 'undefined' ? 8000 : 15000;

function withTimeout(init?: RequestInit): RequestInit {
  const t = AbortSignal.timeout(DEFAULT_TIMEOUT_MS);
  return {
    ...init,
    signal: init?.signal ? AbortSignal.any([init.signal, t]) : t
  };
}

// Silent client-side retry for transient mobile failures (one dropped 5G
// packet, brief Cloudflare edge blip, etc) that would otherwise surface as
// "Internal Error" to the user. SSR is excluded because the load function's
// .catch() already handles fallback; non-GET methods are excluded because
// replaying POST/PUT/DELETE could double-create / double-charge — the same
// rationale as nginx's `proxy_next_upstream` policy. 5xx is the only
// retryable status: 4xx is a real client error, 2xx/3xx are success.
async function fetchWithRetry(url: string, init?: RequestInit): Promise<Response> {
  const isServer = typeof window === 'undefined';
  const method = (init?.method ?? 'GET').toUpperCase();
  const idempotent = method === 'GET' || method === 'HEAD';
  const maxAttempts = isServer || !idempotent ? 1 : 3;

  let lastResponse: Response | undefined;
  let lastError: unknown;
  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    if (attempt > 1) {
      await new Promise((r) => setTimeout(r, 200 * (attempt - 1)));
    }
    try {
      const res = await fetch(url, withTimeout(init));
      if (res.status < 500) return res;
      lastResponse = res;
    } catch (e) {
      // Caller cancelled (e.g. SvelteKit aborted SPA navigation) — respect it.
      if ((init?.signal as AbortSignal | undefined)?.aborted) throw e;
      lastError = e;
    }
  }
  if (lastResponse) return lastResponse;
  throw lastError;
}

// ApiError preserves the HTTP status and the backend's `{error: string}` body
// so callers can branch on status (e.g. cart-add surfacing the 403 from
// ErrCannotPurchase to a toast rather than failing silently).
export class ApiError extends Error {
  readonly status: number;
  readonly path: string;
  readonly serverMessage: string | null;
  constructor(status: number, path: string, serverMessage: string | null) {
    super(serverMessage ? `API ${status}: ${serverMessage}` : `API ${status}: ${path}`);
    this.name = 'ApiError';
    this.status = status;
    this.path = path;
    this.serverMessage = serverMessage;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetchWithRetry(`${base()}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers }
  });
  if (!res.ok) {
    let serverMessage: string | null = null;
    try {
      const body = await res.json();
      if (body && typeof body.error === 'string') serverMessage = body.error;
    } catch {
      // body was not JSON — leave serverMessage null
    }
    throw new ApiError(res.status, path, serverMessage);
  }
  return res.json() as Promise<T>;
}

// Bearer-header helper. Shared between the public storefront helpers (where
// the token is optional) and the customer-account helpers further down (where
// it's required). Hoisted up here so the optional-token public helpers below
// don't reference it before declaration.
function authed(token: string): RequestInit {
  return { headers: { Authorization: `Bearer ${token}` } };
}

// Public storefront reads. The optional token lets server-side load functions
// forward the visitor's customer_token cookie so the backend applies their
// actual storefront role (installer vs customer) when filtering by
// per-role category rules. Anonymous requests omit the token and are treated
// as "customer" — the historical default.
//
// Browser-side calls have no easy access to the httpOnly customer_token
// cookie, so client-side load-more / filter changes fall through anonymously.
// SSR-fetched data covers the first paint and is the main thing the visitor
// notices.
export const getCategories = (token?: string | null) =>
  request<Category[]>('/categories', token ? authed(token) : undefined);
export const getCategoryBySlug = (slug: string, token?: string | null) =>
  request<Category>(`/categories/by-slug/${slug}`, token ? authed(token) : undefined);

export interface ProductListFilters {
  limit?: number;
  offset?: number;
  search?: string;
  category?: string;
  minPrice?: number;
  maxPrice?: number;
  sort?: 'new' | 'price_asc' | 'price_desc' | 'name';
}

const buildProductQuery = (filters: ProductListFilters): URLSearchParams => {
  const qs = new URLSearchParams({
    limit: String(filters.limit ?? 20),
    offset: String(filters.offset ?? 0)
  });
  if (filters.search) qs.set('q', filters.search);
  if (filters.category) qs.set('category', filters.category);
  if (filters.minPrice != null) qs.set('min_price', String(filters.minPrice));
  if (filters.maxPrice != null) qs.set('max_price', String(filters.maxPrice));
  if (filters.sort) qs.set('sort', filters.sort);
  return qs;
};

// Token param: forwards customer_token from SSR loads so the backend filters
// products / categories by the visitor's actual storefront role (installer
// vs customer) per the role-rules matrix. Omit → backend treats request as
// anonymous "customer". Variants, images and bundle-item metadata aren't
// role-filtered so those helpers keep the simpler signature.
export const getProducts = (limit = 20, offset = 0, search = '', token?: string | null) =>
  request<Product[]>(
    `/products?${buildProductQuery({ limit, offset, search }).toString()}`,
    token ? authed(token) : undefined
  );

export const getProductsFiltered = (
  filters: ProductListFilters,
  init?: RequestInit,
  token?: string | null
) =>
  request<Product[]>(`/products?${buildProductQuery(filters).toString()}`, {
    ...init,
    ...(token ? { headers: { Authorization: `Bearer ${token}`, ...init?.headers } } : {})
  });

// Variant of getProductsFiltered that also surfaces the X-Total-Count header
// so the storefront can render an accurate "共 X 件商品" alongside infinite
// scroll. Total reflects the WHERE-filtered set BEFORE limit/offset.
export const getProductsListPage = async (
  filters: ProductListFilters,
  init?: RequestInit,
  token?: string | null
): Promise<{ items: Product[]; total: number }> => {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetchWithRetry(`${base()}/products?${buildProductQuery(filters).toString()}`, {
    ...init,
    headers: { ...headers, ...init?.headers }
  });
  if (!res.ok) throw new Error(`API ${res.status}: /products`);
  const items = (await res.json()) as Product[];
  const totalHeader = res.headers.get('X-Total-Count');
  const total = totalHeader != null ? Number(totalHeader) : items.length;
  return { items, total: Number.isFinite(total) ? total : items.length };
};

export const getProductsByCategorySlug = (
  categorySlug: string,
  limit = 20,
  offset = 0,
  search = '',
  token?: string | null
) =>
  request<Product[]>(
    `/products?${buildProductQuery({ limit, offset, search, category: categorySlug }).toString()}`,
    token ? authed(token) : undefined
  );
export const getProductByID = (id: string, token?: string | null) =>
  request<Product>(`/products/${id}`, token ? authed(token) : undefined);
// Public single-product lookup by slug. Per-role can_view still 404s the PDP
// for that role (matches storefront listing visibility); the "is_listed"
// (private-link) dimension is intentionally bypassed so unlisted PDPs stay
// reachable via direct URL.
export const getProductBySlug = (slug: string, token?: string | null) =>
  request<Product>(
    `/products/by-slug/${encodeURIComponent(slug)}`,
    token ? authed(token) : undefined
  );
export const getProductVariants = (id: string) => request<Variant[]>(`/products/${id}/variants`);
export const getVariantByID = (id: string) => request<Variant>(`/products/variants/${id}`);
export const getProductImages = (id: string) => request<ProductImage[]>(`/products/${id}/images`);
export const getProductBundleItems = (id: string) => request<BundleItem[]>(`/products/${id}/bundle-items`);
export const getProductPromoBundles = (id: string, token?: string | null) =>
  request<PromoBundle[]>(
    `/products/${id}/promo-bundles`,
    token ? authed(token) : undefined
  );
export const getFrequentlyBoughtTogether = (id: string, limit = 4, token?: string | null) =>
  request<Product[]>(
    `/products/${id}/frequently-bought-together?limit=${limit}`,
    token ? authed(token) : undefined
  );

export const getOrCreateCart = (sessionToken: string, customerID?: string) =>
  request<Cart>('/cart', {
    method: 'POST',
    body: JSON.stringify({ session_token: sessionToken, customer_id: customerID ?? null })
  });
export const getCart = (id: string) => request<Cart>(`/cart/${id}`);
export const addToCart = (cartID: string, variantID: string, quantity: number) =>
  request<CartItem>(`/cart/${cartID}/items`, {
    method: 'POST',
    body: JSON.stringify({ variant_id: variantID, quantity })
  });
export const updateCartItem = (cartID: string, itemID: string, quantity: number) =>
  request(`/cart/${cartID}/items/${itemID}`, {
    method: 'PUT',
    body: JSON.stringify({ quantity })
  });
export const removeCartItem = (cartID: string, itemID: string) =>
  fetch(`${base()}/cart/${cartID}/items/${itemID}`, { method: 'DELETE' });

export const checkout = (
  cartID: string,
  options: {
    customerID?: string;
    customerInfo?: CustomerInfoInput;
    shippingAddressID?: string;
    shippingAddress?: ShippingAddressInput;
    saveAddress?: boolean;
    shippingFee?: number;
    couponCode?: string;
    notes?: string;
    saveCard?: boolean;
    savedPaymentMethodId?: string;
  } = {}
) =>
  request<CheckoutResult>('/orders/checkout', {
    method: 'POST',
    body: JSON.stringify({
      cart_id: cartID,
      customer_id: options.customerID ?? null,
      customer_info: options.customerInfo ?? null,
      shipping_address_id: options.shippingAddressID ?? null,
      shipping_address: options.shippingAddress ?? null,
      save_address: options.saveAddress ?? false,
      shipping_fee: options.shippingFee ?? 0,
      coupon_code: options.couponCode ?? null,
      notes: options.notes ?? null,
      save_card: options.saveCard ?? false,
      saved_payment_method_id: options.savedPaymentMethodId ?? null
    })
  });

export const validateCoupon = (
  code: string,
  subtotal: number,
  customerRole?: string,
  isGuest?: boolean
) =>
  request<{ valid: boolean; discount_type?: string; discount_value?: number; discount_amount?: number; message?: string; message_code?: string }>(
    '/pricing/validate-coupon',
    {
      method: 'POST',
      body: JSON.stringify({
        code,
        subtotal,
        customer_role: customerRole ?? undefined,
        is_guest: isGuest
      })
    }
  );

export interface QuoteAppliedCampaign {
  id: string;
  name: string;
  description?: string | null;
  amount: number;
}

export interface QuoteAppliedCoupon {
  id: string;
  code: string;
  description?: string | null;
  amount: number;
}

export interface QuoteResult {
  subtotal: number;
  applied_campaigns: QuoteAppliedCampaign[];
  applied_coupon?: QuoteAppliedCoupon | null;
  total_discount: number;
  tax_amount: number;
  tax_inclusive: boolean;
  shipping_free: boolean;
  total: number;
  coupon_error?: string;
  coupon_error_code?: string;
}

export const quoteOrder = (
  cartID: string,
  options: { couponCode?: string; customerID?: string } = {}
) =>
  request<QuoteResult>('/orders/quote', {
    method: 'POST',
    body: JSON.stringify({
      cart_id: cartID,
      coupon_code: options.couponCode ?? null,
      customer_id: options.customerID ?? null
    })
  });

export const getPaymentConfig = () => request<PaymentConfig>('/payments/config');

// ── ShipAny logistics ───────────────────────────────────────────

export type ShipanyAddress = {
  name?: string;
  phone?: string;
  line1: string;
  line2?: string;
  district?: string;
  city?: string;
  postal_code?: string;
  country: string; // ISO 3166-1 alpha-2
};

export type ShipanyRateOption = {
  quot_uid?: string;
  carrier: string;
  carrier_name: string;
  service: string;
  service_name: string;
  fee_hkd: number;
  eta_days?: string;
  requires_pickup_point: boolean;
};

export type ShipanyPickupPoint = {
  id: string;
  name: string;
  address: string;
  district?: string;
  carrier?: string;
};

export const getShipanyQuote = (cartID: string, shippingAddress: ShipanyAddress) =>
  request<ShipanyRateOption[]>('/shipany/quote', {
    method: 'POST',
    body: JSON.stringify({ cart_id: cartID, shipping_address: shippingAddress })
  });

export const listShipanyPickupPoints = (carrier: string, district?: string) => {
  const q = new URLSearchParams({ carrier });
  if (district) q.set('district', district);
  return request<ShipanyPickupPoint[]>(`/shipany/pickup-points?${q.toString()}`);
};

// Default courier + service resolved from admin settings, with display labels
// looked up via the ShipAny couriers feed. Used by the storefront checkout
// to render a read-only logistics panel — the customer no longer picks.
export type ShippingDefault = {
  configured: boolean;
  courier_uid?: string;
  courier_name?: string;
  service_uid?: string;
  service_name?: string;
};

export const getShippingDefault = () =>
  request<ShippingDefault>('/shipany/shipping-default');

export type PublicSetting = { key: string; value: string; description?: string; updated_at: string };
export const getPublicSettings = () => request<PublicSetting[]>('/settings/');

export const setupPassword = (token: string, password: string) =>
  fetch(`${base()}/customers/setup-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token, password })
  });

export const requestPasswordReset = (email: string) =>
  fetch(`${base()}/customers/forgot-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email })
  });

// CMS public API
export const getBlogPosts = (limit = 20, offset = 0) =>
  request<CmsPost[]>(`/cms/posts?limit=${limit}&offset=${offset}`);

export const getBlogPostsByCategorySlug = (categorySlug: string, limit = 20, offset = 0) => {
  const qs = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
    category: categorySlug
  });
  return request<CmsPost[]>(`/cms/posts?${qs.toString()}`);
};

export const getBlogPostBySlug = (slug: string) =>
  request<CmsPost>(`/cms/posts/by-slug/${slug}`);

export const getBlogCategoryBySlug = (slug: string) =>
  request<{
    id: string;
    slug: string;
    name: string;
    desktop_banner_url?: string;
    mobile_banner_url?: string;
    sort_order: number;
  }>(`/cms/post-categories/by-slug/${slug}`);

export const getCmsPageBySlug = (slug: string) =>
  request<CmsPage>(`/cms/pages/by-slug/${slug}`);

export const getCmsPageByID = (id: string) =>
  request<CmsPage>(`/cms/pages/by-id/${id}`);

// Resolve a batch of media_files.original_name values to their canonical
// /uploads/... URLs. Backs the [photo-grid] shortcode's server-side
// resolve step. Unknown names are simply absent from the returned map;
// an empty input skips the network call entirely.
export const lookupMediaByNames = async (
  names: string[]
): Promise<Record<string, string>> => {
  if (names.length === 0) return {};
  const qs = new URLSearchParams({ names: names.join(',') });
  return request<Record<string, string>>(`/media/by-names?${qs.toString()}`);
};

// Forms (CF7-style). getPublicForm fetches the public form spec (no admin
// fields like mail templates); submitForm posts the user's data + grecaptcha
// token to the backend.
import type { PublicForm } from '$lib/shortcodes/types';
export const getPublicForm = (slug: string) =>
  request<PublicForm>(`/forms/${slug}`);

export type FormSubmitResult = { ok: true; message: string };
export type FormSubmitError = { error: string; fields?: Record<string, string>; code?: string };

export const submitForm = async (
  slug: string,
  data: Record<string, string>,
  recaptchaToken: string
): Promise<FormSubmitResult | FormSubmitError> => {
  const res = await fetch(`${base()}/forms/${slug}/submit`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ data, recaptcha_token: recaptchaToken })
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) return body as FormSubmitError;
  return body as FormSubmitResult;
};

// submitFormMultipart is the upload-aware sibling of submitForm. The browser
// sets the multipart boundary automatically; we use XHR (not fetch) so the
// caller can render a real upload progress bar — fetch() doesn't expose
// upload progress without ReadableStream hacks. `onProgress` is called with
// a 0–1 value whenever a progress event fires.
export const submitFormMultipart = (
  slug: string,
  formData: FormData,
  onProgress?: (fraction: number) => void
): Promise<FormSubmitResult | FormSubmitError> =>
  new Promise((resolve) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${base()}/forms/${slug}/submit`);
    if (onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) onProgress(e.loaded / e.total);
      };
    }
    xhr.onload = () => {
      let body: unknown = {};
      try {
        body = JSON.parse(xhr.responseText || '{}');
      } catch {
        body = {};
      }
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(body as FormSubmitResult);
      } else {
        resolve(body as FormSubmitError);
      }
    };
    xhr.onerror = () => resolve({ error: 'Network error' });
    xhr.send(formData);
  });

// Token is optional — anonymous storefront visitors don't have one,
// and the backend treats them as "customer". When a logged-in customer
// hits this, forwarding the token lets the backend filter nav items
// per their actual role (customer / installer).
export const getNavMenu = (handle: string, token?: string | null) =>
  request<NavMenu>(`/cms/nav/by-handle/${handle}`, token ? authed(token) : undefined);

// Public read of an order, authorized via a Stripe payment_intent returned
// from the checkout redirect. The backend confirms PI matches the order
// before returning a redacted summary (no PII). Used by /checkout/success.
export const getOrderByPaymentIntent = (id: string, paymentIntent: string) =>
  request<Order>(`/orders/${id}?payment_intent=${encodeURIComponent(paymentIntent)}`);

export type OrderPaymentInfo = {
  order: Order;
  client_secret: string;
  publishable_key: string;
  mode: string;
  currency: string;
};

export const getOrderPaymentInfo = (id: string, clientSecret: string) =>
  request<OrderPaymentInfo>(`/orders/${id}/payment-info?cs=${encodeURIComponent(clientSecret)}`);

export type CartPendingOrder = {
  order_id: string;
  order_number: string;
  total: number;
  client_secret: string;
};

// Returns the cart's outstanding unpaid order so cart/checkout can show a
// "resume payment" banner, or null when there is none (backend replies 204).
// Best-effort: any error resolves to null so the banner simply doesn't show.
export const getCartPendingOrder = async (cartID: string): Promise<CartPendingOrder | null> => {
  try {
    const res = await fetch(`${base()}/orders/by-cart/${cartID}/pending`);
    if (res.status === 204 || !res.ok) return null;
    return (await res.json()) as CartPendingOrder;
  } catch {
    return null;
  }
};

export type OrderSetupTokenResult = {
  token?: string;
  url?: string;
  already_set: boolean;
};

export const createOrderSetupToken = (orderID: string, paymentIntent: string) =>
  request<OrderSetupTokenResult>(`/orders/${orderID}/setup-token`, {
    method: 'POST',
    body: JSON.stringify({ payment_intent: paymentIntent })
  });

// --- Customer auth & account ---
// (authed() defined near the top so the optional-token public helpers can
// call it without referencing-before-declaration concerns.)

export const registerCustomer = (
  email: string,
  password: string,
  firstName: string,
  lastName: string,
  phone?: string
) =>
  request<{ customer: Customer; token: string; expires_in: number }>('/customers/register', {
    method: 'POST',
    body: JSON.stringify({ email, password, first_name: firstName, last_name: lastName, phone })
  });

export const loginCustomer = (email: string, password: string) =>
  request<{ customer: Customer; token: string; expires_in: number }>('/customers/login', {
    method: 'POST',
    body: JSON.stringify({ email, password })
  });

export const getMyProfile = (token: string) =>
  request<Customer>('/customers/me', authed(token));

export const updateMyProfile = (
  token: string,
  data: { first_name: string; last_name: string; phone?: string }
) =>
  request<Customer>('/customers/me', {
    method: 'PUT',
    body: JSON.stringify(data),
    ...authed(token)
  });

export const getMyAddresses = (token: string) =>
  request<Address[]>('/customers/me/addresses', authed(token));

export const createMyAddress = (token: string, data: object) =>
  request<Address>('/customers/me/addresses', {
    method: 'POST',
    body: JSON.stringify(data),
    ...authed(token)
  });

export const updateMyAddress = (token: string, id: string, data: object) =>
  request<Address>(`/customers/me/addresses/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
    ...authed(token)
  });

export const deleteMyAddress = (token: string, id: string) =>
  fetch(`${base()}/customers/me/addresses/${id}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` }
  });

export const getMyOrders = (token: string, limit = 20, offset = 0) =>
  request<Order[]>(`/customers/me/orders?limit=${limit}&offset=${offset}`, authed(token));

export const lookupMyOrder = (token: string, n: string) =>
  request<{ id: string }>(`/customers/me/orders/lookup/${n}`, authed(token));

// Authenticated read of a single order owned by the current customer. The
// backend 404s if the order belongs to someone else.
export const getMyOrderByID = (token: string, id: string) =>
  request<Order>(`/customers/me/orders/${id}`, authed(token));

// Owner-authenticated payment info for a still-payable pending order, so the
// account page can build a /pay/{id}?cs=… link without the shopper holding a
// magic-link cs. 404s if the order isn't owned by the caller or isn't payable.
export const getMyOrderPaymentInfo = (token: string, id: string) =>
  request<OrderPaymentInfo>(`/customers/me/orders/${id}/payment-info`, authed(token));

// --- Order notices (customer) ---

export const getMyOrderNotices = (token: string, orderID: string) =>
  request<OrderNotice[]>(`/order-notices/${orderID}`, authed(token));

export const createMyOrderNotice = (token: string, orderID: string, body: string) =>
  request<OrderNotice>(`/order-notices/${orderID}`, {
    method: 'POST',
    body: JSON.stringify({ body }),
    ...authed(token)
  });

export const markMyOrderNoticesRead = (token: string, orderID: string) =>
  fetch(`${base()}/order-notices/${orderID}/read`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` }
  });

export const getMyOrderNoticeUnreadCounts = (token: string) =>
  request<Record<string, number>>(`/order-notices/unread-counts`, authed(token));

// --- Saved payment methods ---

export const getMySavedCards = (token: string) =>
  request<SavedPaymentMethod[]>('/payments/saved-cards', authed(token));

export const deleteMySavedCard = (token: string, id: string) =>
  fetch(`${base()}/payments/saved-cards/${id}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` }
  });

export const setDefaultCard = (token: string, id: string) =>
  fetch(`${base()}/payments/saved-cards/${id}/default`, {
    method: 'PUT',
    headers: { Authorization: `Bearer ${token}` }
  });

// --- Loyalty (P3 #24) ---

export interface LoyaltyLedgerRow {
  id: string;
  delta: number;
  balance_after: number;
  reason: string;
  order_id?: string;
  actor_email?: string;
  note?: string;
  created_at: string;
}

export const getMyLoyaltyBalance = (token: string) =>
  request<{ points: number }>('/loyalty/', authed(token));

export const getMyLoyaltyLedger = (token: string) =>
  request<LoyaltyLedgerRow[]>('/loyalty/ledger', authed(token));
