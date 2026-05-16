import type { BundleItem, Category, Order, OrderNotice, Product, Variant, ProductImage } from '$lib/types';

const base = () =>
  typeof window === 'undefined'
    ? (process.env.API_BASE ?? 'http://localhost:8080/api/v1')
    : '/api/v1';

async function request<T>(path: string, token: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${base()}${path}`, {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
      ...init?.headers
    },
    ...init
  });
  if (!res.ok) throw new Error(`API ${res.status}: ${path}`);
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export interface AdminStats {
  total_products: number;
  total_orders: number;
  total_revenue: number;
  pending_orders: number;
}

export const adminLogin = async (email: string, password: string): Promise<string> => {
  const res = await fetch(`${base()}/admin/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  });
  if (!res.ok) throw new Error('Invalid credentials');
  const data = await res.json();
  return data.token;
};

export const getStats = (token: string) =>
  request<AdminStats>('/admin/stats', token);

// Products — admin list hits a dedicated endpoint that returns all statuses.
// Detail reads (GET) use the public /products/* (open). Mutations use
// /admin/products/* so they pass through the admin auth + audit middleware.
//
// Response wraps the rows in `{items, total}` so the admin UI can render
// pagination math without a second roundtrip. Each row carries
// `variant_count` so the list page never has to fan out one /variants
// request per product.
export interface AdminProductRow extends Product {
  variant_count: number;
}
export interface PagedResponse<T> {
  items: T[];
  total: number;
}
export const adminGetProducts = (token: string, limit = 50, offset = 0, q = '', categorySlug = '', kind = '') => {
  const qs = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (q) qs.set('q', q);
  if (categorySlug) qs.set('category', categorySlug);
  if (kind) qs.set('kind', kind);
  return request<PagedResponse<AdminProductRow>>(`/admin/inventory/?${qs.toString()}`, token);
};

export const adminCreateProduct = (token: string, body: Partial<Product>) =>
  request<Product>('/admin/products', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateProduct = (token: string, id: string, body: Partial<Product> & { status: string }) =>
  request<Product>(`/admin/products/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeleteProduct = (token: string, id: string) =>
  request(`/admin/products/${id}`, token, { method: 'DELETE' });

export const adminGetProduct = (token: string, id: string) =>
  request<Product>(`/products/${id}`, token);

export const adminGetVariants = (token: string, productID: string) =>
  request<Variant[]>(`/products/${productID}/variants`, token);

export const adminCreateVariant = (token: string, productID: string, body: Partial<Variant>) =>
  request<Variant>(`/admin/products/${productID}/variants`, token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateVariant = (token: string, productID: string, variantID: string, body: Partial<Variant> & { is_active: boolean }) =>
  request<Variant>(`/admin/products/${productID}/variants/${variantID}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeleteVariant = (token: string, productID: string, variantID: string) =>
  request(`/admin/products/${productID}/variants/${variantID}`, token, { method: 'DELETE' });

export const adminAdjustStock = (token: string, productID: string, variantID: string, delta: number) =>
  request<Variant>(`/admin/products/${productID}/variants/${variantID}/stock`, token, { method: 'POST', body: JSON.stringify({ delta }) });

export const adminReorderVariants = (token: string, productID: string, ids: string[]) =>
  request<void>(`/admin/products/${productID}/variants/reorder`, token, {
    method: 'PATCH',
    body: JSON.stringify({ ids })
  });

export interface VariantHistoryRow {
  id: string;
  variant_id: string;
  delta: number;
  before_qty: number;
  after_qty: number;
  reason: string;
  actor_user_id?: string;
  actor_email?: string;
  order_id?: string;
  note?: string;
  created_at: string;
}

export const adminGetVariantStockHistory = (token: string, productID: string, variantID: string, limit = 50) =>
  request<VariantHistoryRow[]>(`/admin/products/${productID}/variants/${variantID}/history?limit=${limit}`, token);

export const adminGetImages = (token: string, productID: string) =>
  request<ProductImage[]>(`/products/${productID}/images`, token);

export const adminAddImage = (token: string, productID: string, body: Partial<ProductImage>) =>
  request<ProductImage>(`/admin/products/${productID}/images`, token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateImage = (token: string, productID: string, imageID: string, body: Partial<ProductImage>) =>
  request<ProductImage>(`/admin/products/${productID}/images/${imageID}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeleteImage = (token: string, productID: string, imageID: string) =>
  request(`/admin/products/${productID}/images/${imageID}`, token, { method: 'DELETE' });

// Bundle items
export const adminGetBundleItems = (token: string, productID: string) =>
  request<BundleItem[]>(`/products/${productID}/bundle-items`, token);

export const adminSetBundleItems = (
  token: string,
  productID: string,
  items: Array<{ component_variant_id: string; quantity: number; sort_order: number; display_name_override?: string }>
) =>
  request<BundleItem[]>(`/admin/products/${productID}/bundle-items`, token, {
    method: 'PUT',
    body: JSON.stringify({ items })
  });

// Categories — admin opts out of the storefront-facing hidden filter so it
// can still see / assign / pick from hidden categories.
export const adminGetCategories = (token: string) =>
  request<Category[]>('/categories?include_hidden=true', token);

export const adminCreateCategory = (token: string, body: Partial<Category>) =>
  request<Category>('/categories', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateCategory = (token: string, id: string, body: Partial<Category> & { is_active: boolean }) =>
  request<Category>(`/categories/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeleteCategory = (token: string, id: string) =>
  request<void>(`/categories/${id}`, token, { method: 'DELETE' });

export const adminReorderCategories = (token: string, ids: string[]) =>
  request<void>(`/categories/reorder`, token, {
    method: 'PATCH',
    body: JSON.stringify({ ids })
  });

// Orders
export const adminGetOrders = (token: string, limit = 50, offset = 0) =>
  request<PagedResponse<Order>>(`/admin/orders?limit=${limit}&offset=${offset}`, token);

export const adminGetOrder = (token: string, id: string) =>
  request<Order>(`/admin/orders/${id}`, token);

export const adminUpdateOrderStatus = (token: string, id: string, status: string, note?: string) =>
  request<Order>(`/admin/orders/${id}/status`, token, {
    method: 'POST',
    body: JSON.stringify({ status, note })
  });

export const adminDeleteOrder = (token: string, id: string) =>
  request(`/admin/orders/${id}`, token, { method: 'DELETE' });

export const adminIssueRefund = (token: string, id: string, amountCents: number, reason: string) =>
  request<Order>(`/admin/orders/${id}/refund`, token, {
    method: 'POST',
    body: JSON.stringify({ amount_cents: amountCents, reason })
  });

// Order notices (admin)
export const adminListOrderNotices = (token: string, orderID: string) =>
  request<OrderNotice[]>(`/admin/order-notices/${orderID}`, token);

export const adminCreateOrderNotice = (token: string, orderID: string, role: 'system' | 'admin', body: string) =>
  request<OrderNotice>(`/admin/order-notices/${orderID}`, token, {
    method: 'POST',
    body: JSON.stringify({ role, body })
  });

export const adminMarkOrderNoticesRead = (token: string, orderID: string) =>
  request<void>(`/admin/order-notices/${orderID}/read`, token, { method: 'POST' });

export const adminGetOrderNoticeUnreadCounts = (token: string) =>
  request<Record<string, number>>(`/admin/order-notices/unread-counts`, token);

// ── CMS ──────────────────────────────────────────────────────────────────────

export interface CmsPage {
  id: string;
  number: number;
  slug: string;
  title: string;
  content: string;
  meta_title?: string;
  meta_desc?: string;
  is_published: boolean;
  show_title: boolean;
  content_padded: boolean;
  created_at: string;
  updated_at: string;
}

export interface CmsPost {
  id: string;
  number: number;
  category_id?: string;
  category_ids?: string[];
  slug: string;
  title: string;
  excerpt?: string;
  content: string;
  cover_image_url?: string;
  is_published: boolean;
  published_at?: string;
  created_at: string;
  updated_at: string;
}

// Pages
export const adminGetPages = (token: string, limit = 50, offset = 0, q = '') => {
  const qs = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (q) qs.set('q', q);
  return request<PagedResponse<CmsPage>>(`/admin/cms/pages?${qs.toString()}`, token);
};

export const adminGetPage = (token: string, id: string) =>
  request<CmsPage>(`/admin/cms/pages/${id}`, token);

export const adminCreatePage = (token: string, body: Partial<CmsPage>) =>
  request<CmsPage>('/admin/cms/pages', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdatePage = (token: string, id: string, body: Partial<CmsPage> & { is_published: boolean }) =>
  request<CmsPage>(`/admin/cms/pages/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeletePage = (token: string, id: string) =>
  request(`/admin/cms/pages/${id}`, token, { method: 'DELETE' });

// Posts — admin list returns `{items, total}` for pagination.
export const adminGetPosts = (token: string, limit = 50, offset = 0, q = '', categorySlug = '') => {
  const qs = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (q) qs.set('q', q);
  if (categorySlug) qs.set('category', categorySlug);
  return request<PagedResponse<CmsPost>>(`/admin/cms/posts?${qs.toString()}`, token);
};

export const adminGetPost = (token: string, id: string) =>
  request<CmsPost>(`/admin/cms/posts/${id}`, token);

export const adminCreatePost = (token: string, body: Partial<CmsPost>) =>
  request<CmsPost>('/admin/cms/posts', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdatePost = (token: string, id: string, body: Partial<CmsPost> & { is_published: boolean }) =>
  request<CmsPost>(`/admin/cms/posts/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

// Admin-only ID lookup: resolves a sequential number to a UUID for any
// of the supported entities. Returns 404 if no row matches.
export const adminLookup = (token: string, entity: 'products' | 'orders' | 'pages' | 'posts', n: string) =>
  request<{ id: string }>(`/admin/lookup/${entity}/${n}`, token);

export const adminDeletePost = (token: string, id: string) =>
  request(`/admin/cms/posts/${id}`, token, { method: 'DELETE' });

// Post Categories
export interface PostCategory {
  id: string;
  slug: string;
  name: string;
  desktop_banner_url?: string;
  mobile_banner_url?: string;
  sort_order: number;
}

export const adminGetPostCategories = (token: string) =>
  request<PostCategory[]>('/admin/cms/post-categories', token);

export const adminCreatePostCategory = (token: string, body: Omit<PostCategory, 'id'>) =>
  request<PostCategory>('/admin/cms/post-categories', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdatePostCategory = (token: string, id: string, body: Omit<PostCategory, 'id'>) =>
  request<PostCategory>(`/admin/cms/post-categories/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeletePostCategory = (token: string, id: string) =>
  request(`/admin/cms/post-categories/${id}`, token, { method: 'DELETE' });

export const adminReorderPostCategories = (token: string, ids: string[]) =>
  request<void>(`/admin/cms/post-categories/reorder`, token, {
    method: 'PATCH',
    body: JSON.stringify({ ids })
  });

// Navigation
export interface NavItem {
  id: string;
  menu_id: string;
  parent_id?: string;
  label: string;
  url: string;
  target: string;
  sort_order: number;
  children: NavItem[];
}

export interface NavMenu {
  id: string;
  handle: string;
  name: string;
  items: NavItem[];
  created_at: string;
  updated_at: string;
}

export const adminGetNavMenus = (token: string) =>
  request<NavMenu[]>('/admin/cms/nav', token);

export const adminGetNavMenu = (token: string, id: string) =>
  request<NavMenu>(`/admin/cms/nav/${id}`, token);

export const adminAddNavItem = (token: string, menuID: string, body: Partial<NavItem>) =>
  request<NavItem>(`/admin/cms/nav/${menuID}/items`, token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateNavItem = (token: string, menuID: string, itemID: string, body: Partial<NavItem>) =>
  request<NavItem>(`/admin/cms/nav/${menuID}/items/${itemID}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeleteNavItem = (token: string, menuID: string, itemID: string) =>
  request(`/admin/cms/nav/${menuID}/items/${itemID}`, token, { method: 'DELETE' });

export const adminReplaceNavItems = (token: string, menuID: string, items: Partial<NavItem>[]) =>
  request<NavItem[]>(`/admin/cms/nav/${menuID}/items`, token, { method: 'PUT', body: JSON.stringify(items) });

export const adminReorderNavItems = (token: string, menuID: string, ids: string[]) =>
  request<void>(`/admin/cms/nav/${menuID}/items/reorder`, token, {
    method: 'PATCH',
    body: JSON.stringify({ ids })
  });

// ── Customers ─────────────────────────────────────────────────────────────────

export interface Customer {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  phone?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CustomerAddress {
  id: string;
  customer_id?: string;
  first_name: string;
  last_name: string;
  phone?: string;
  line1: string;
  line2?: string;
  city: string;
  state?: string;
  postal_code: string;
  country: string;
  is_default: boolean;
  created_at: string;
}

export interface CustomerOrderSummary {
  id: string;
  number: number;
  status: string;
  total: number;
  created_at: string;
}

export const adminGetCustomers = (token: string, limit = 50, offset = 0, q = '') => {
  const qs = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (q) qs.set('q', q);
  return request<PagedResponse<Customer>>(`/admin/customers?${qs.toString()}`, token);
};

export const adminGetCustomer = (token: string, id: string) =>
  request<Customer>(`/admin/customers/${id}`, token);

export const adminGetCustomerOrders = (token: string, id: string) =>
  request<CustomerOrderSummary[]>(`/customers/me/orders`, token);

export const adminSendResetPasswordEmail = async (token: string, customerID: string): Promise<void> => {
  const res = await fetch(`${base()}/admin/customers/${customerID}/send-reset-password-email`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` }
  });
  if (res.status === 204) return;
  let msg = `Send failed (${res.status})`;
  try {
    const body = await res.json();
    if (body?.error) msg = body.error;
  } catch {
    // ignore — fall through to generic message
  }
  throw new Error(msg);
};

// ── Discounts (Campaigns + Coupons) ──────────────────────────────────────────

export type DiscountType = 'percentage' | 'fixed';
export type CampaignTargetType = 'all' | 'category' | 'product';

export interface Campaign {
  id: string;
  name: string;
  description?: string;
  discount_type: DiscountType;
  discount_value: number;
  target_type: CampaignTargetType;
  target_id?: string;
  min_order_amount?: number;
  starts_at?: string;
  ends_at?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface Coupon {
  id: string;
  code: string;
  description?: string;
  discount_type: DiscountType;
  discount_value: number;
  min_order_amount?: number;
  max_uses?: number;
  used_count: number;
  starts_at?: string;
  ends_at?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface CampaignInput {
  name: string;
  description?: string;
  discount_type: DiscountType;
  discount_value: number;
  target_type: CampaignTargetType;
  target_id?: string | null;
  min_order_amount?: number | null;
  starts_at?: string | null;
  ends_at?: string | null;
  is_active?: boolean;
}

export interface CouponInput {
  code: string;
  description?: string;
  discount_type: DiscountType;
  discount_value: number;
  min_order_amount?: number | null;
  max_uses?: number | null;
  starts_at?: string | null;
  ends_at?: string | null;
  is_active?: boolean;
}

export const adminListCampaigns = (token: string, limit = 50, offset = 0) => {
  const qs = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  return request<PagedResponse<Campaign>>(`/admin/pricing/campaigns/?${qs.toString()}`, token);
};

export const adminGetCampaign = (token: string, id: string) =>
  request<Campaign>(`/admin/pricing/campaigns/${id}`, token);

export const adminCreateCampaign = (token: string, body: CampaignInput) =>
  request<Campaign>('/admin/pricing/campaigns/', token, {
    method: 'POST',
    body: JSON.stringify(body)
  });

export const adminUpdateCampaign = (token: string, id: string, body: CampaignInput) =>
  request<Campaign>(`/admin/pricing/campaigns/${id}`, token, {
    method: 'PUT',
    body: JSON.stringify(body)
  });

export const adminDeleteCampaign = (token: string, id: string) =>
  request<void>(`/admin/pricing/campaigns/${id}`, token, { method: 'DELETE' });

export const adminListCoupons = (token: string, limit = 50, offset = 0) => {
  const qs = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  return request<PagedResponse<Coupon>>(`/admin/pricing/coupons/?${qs.toString()}`, token);
};

export const adminGetCoupon = (token: string, id: string) =>
  request<Coupon>(`/admin/pricing/coupons/${id}`, token);

export const adminCreateCoupon = (token: string, body: CouponInput) =>
  request<Coupon>('/admin/pricing/coupons/', token, {
    method: 'POST',
    body: JSON.stringify(body)
  });

export const adminUpdateCoupon = (token: string, id: string, body: CouponInput) =>
  request<Coupon>(`/admin/pricing/coupons/${id}`, token, {
    method: 'PUT',
    body: JSON.stringify(body)
  });

export const adminDeleteCoupon = (token: string, id: string) =>
  request<void>(`/admin/pricing/coupons/${id}`, token, { method: 'DELETE' });

// ── Settings ──────────────────────────────────────────────────────────────────

export interface Setting {
  key: string;
  value: string;
  description?: string;
  updated_at: string;
}

export const adminGetSettings = (token: string) =>
  request<Setting[]>('/admin/settings', token);

export const adminBulkUpdateSettings = (token: string, updates: Record<string, string>) =>
  request<Setting[]>('/admin/settings', token, { method: 'PUT', body: JSON.stringify(updates) });

export const adminSendTestEmail = (token: string, to: string) =>
  request<Record<string, never>>('/admin/settings/test-email', token, { method: 'POST', body: JSON.stringify({ to }) });

// ── ShipAny admin ─────────────────────────────────────────────────────────────

export interface ShipanyShipment {
  id: string;
  order_id: string;
  shipany_shipment_id: string;
  tracking_number?: string;
  tracking_url?: string;
  label_url?: string;
  carrier: string;
  service: string;
  fee_hkd: number;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface ShipanyCourier {
  uid: string;
  name: string;
  cour_svc_plans?: { cour_svc_pl: string }[];
}

export const adminTestShipanyConnection = (token: string) =>
  request<{ ok: boolean; message: string }>('/admin/shipany/test-connection', token, { method: 'POST' });

export const adminListShipanyCouriers = async (token: string): Promise<ShipanyCourier[]> => {
  const body = await request<ShipanyCourier[] | { couriers?: ShipanyCourier[] }>('/admin/shipany/couriers', token);
  return Array.isArray(body) ? body : (body.couriers ?? []);
};

export const adminGetShipment = (token: string, orderID: string) =>
  request<ShipanyShipment | null>(`/admin/shipany/orders/${orderID}/shipment`, token);

export const adminCreateShipment = (token: string, orderID: string, override?: { carrier: string; service: string }) =>
  request<ShipanyShipment>(`/admin/shipany/orders/${orderID}/shipment`, token, {
    method: 'POST',
    body: override ? JSON.stringify(override) : '{}'
  });

export const adminRequestShipanyPickup = (token: string, orderID: string) =>
  request<ShipanyShipment>(`/admin/shipany/orders/${orderID}/pickup`, token, { method: 'POST' });

// ── Admin Users ───────────────────────────────────────────────────────────────

export interface AdminUser {
  id: string;
  email: string;
  name: string;
  role: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export const adminGetUsers = (token: string, limit = 50, offset = 0, q = '') => {
  const qs = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (q) qs.set('q', q);
  return request<PagedResponse<AdminUser>>(`/admin/users?${qs.toString()}`, token);
};

export const adminCreateUser = (token: string, body: { email: string; password: string; name: string; role: string }) =>
  request<AdminUser>('/admin/users', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateUser = (token: string, id: string, body: { name: string; role: string; is_active: boolean }) =>
  request<AdminUser>(`/admin/users/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeleteUser = (token: string, id: string) =>
  request(`/admin/users/${id}`, token, { method: 'DELETE' });

// ── Media ─────────────────────────────────────────────────────────────────────

export interface MediaRef {
  type: 'product' | 'post';
  id: string;
  name: string;
}

export interface MediaFile {
  id: string;
  filename: string;
  original_name: string;
  mime_type: string;
  size_bytes: number;
  url: string;
  created_at: string;
  refs: MediaRef[];
  webp_url?: string | null;
  webp_size_bytes?: number | null;
  thumbnail_url?: string | null;
  thumbnail_size_bytes?: number | null;
  video_autoplay?: boolean;
  video_fit?: 'contain' | 'cover';
}

export const adminGetMedia = (token: string) =>
  request<MediaFile[]>('/admin/media', token);

export type AdminMediaType = 'all' | 'image' | 'video' | 'link';

// Paginated variant for the admin media library page. Surfaces X-Total-Count
// so the page can drive infinite scroll + render an accurate header badge for
// the current filter. The `adminGetMedia` overload above stays unchanged so
// media-picker callers (settings, product editor) keep the whole-library fetch.
export const adminGetMediaPage = async (
  token: string,
  opts: { limit: number; offset: number; type?: AdminMediaType },
  init?: RequestInit
): Promise<{ items: MediaFile[]; total: number }> => {
  const qs = new URLSearchParams({
    limit: String(opts.limit),
    offset: String(opts.offset)
  });
  if (opts.type && opts.type !== 'all') qs.set('type', opts.type);
  const res = await fetch(`${base()}/admin/media?${qs.toString()}`, {
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    ...init
  });
  if (!res.ok) throw new Error(`API ${res.status}: /admin/media`);
  const items = (await res.json()) as MediaFile[];
  const totalHeader = res.headers.get('X-Total-Count');
  const total = totalHeader != null ? Number(totalHeader) : items.length;
  return { items, total: Number.isFinite(total) ? total : items.length };
};

export const adminGetMediaFile = (token: string, id: string) =>
  request<MediaFile>(`/admin/media/${id}`, token);

export const adminUpdateMedia = (
  token: string,
  id: string,
  body: { original_name?: string; url?: string; video_autoplay?: boolean; video_fit?: 'contain' | 'cover' }
) =>
  request<MediaFile>(`/admin/media/${id}`, token, {
    method: 'PATCH',
    body: JSON.stringify(body)
  });

export const adminDeleteMedia = (token: string, id: string) =>
  request(`/admin/media/${id}`, token, { method: 'DELETE' });

export const adminAddMediaLink = (
  token: string,
  url: string,
  name: string,
  opts?: { autoplay?: boolean; videoFit?: 'contain' | 'cover' }
) =>
  request<MediaFile>('/admin/media/link', token, {
    method: 'POST',
    body: JSON.stringify({
      url,
      name,
      autoplay: opts?.autoplay ?? false,
      video_fit: opts?.videoFit ?? 'contain'
    })
  });

export const adminUploadMedia = (
  token: string,
  file: File,
  onProgress?: (pct: number) => void
): Promise<MediaFile> => {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', `${base()}/admin/media/upload`);
    xhr.setRequestHeader('Authorization', `Bearer ${token}`);

    xhr.upload.onprogress = (e) => {
      if (!onProgress || !e.lengthComputable) return;
      const pct = Math.min(95, Math.round((e.loaded / e.total) * 95));
      onProgress(pct);
    };

    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText) as MediaFile);
        } catch {
          reject(new Error('Upload failed: invalid response'));
        }
      } else {
        reject(new Error(xhr.responseText || `Upload failed: ${xhr.status}`));
      }
    };
    xhr.onerror = () => reject(new Error('Upload failed: network error'));
    xhr.onabort = () => reject(new Error('Upload aborted'));

    const formData = new FormData();
    formData.append('file', file);
    xhr.send(formData);
  });
};

// ── Redirects (P2 #22) ────────────────────────────────────────────────────────

export type RedirectMatchType = 'exact' | 'wildcard';

export interface Redirect {
  id: string;
  from_path: string;
  to_path: string;
  code: 301 | 302;
  is_active: boolean;
  note?: string;
  match_type: RedirectMatchType;
  created_at: string;
  updated_at: string;
}

export interface RedirectInput {
  from_path: string;
  to_path: string;
  code: 301 | 302;
  is_active: boolean;
  note?: string | null;
  match_type: RedirectMatchType;
}

export const adminListRedirects = (token: string, limit = 50, offset = 0) => {
  const qs = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  return request<PagedResponse<Redirect>>(`/admin/redirects/?${qs.toString()}`, token);
};

export const adminGetRedirect = (token: string, id: string) =>
  request<Redirect>(`/admin/redirects/${id}`, token);

export const adminCreateRedirect = (token: string, body: RedirectInput) =>
  request<Redirect>('/admin/redirects/', token, {
    method: 'POST',
    body: JSON.stringify(body)
  });

export const adminUpdateRedirect = (token: string, id: string, body: RedirectInput) =>
  request<Redirect>(`/admin/redirects/${id}`, token, {
    method: 'PUT',
    body: JSON.stringify(body)
  });

export const adminDeleteRedirect = (token: string, id: string) =>
  request<void>(`/admin/redirects/${id}`, token, { method: 'DELETE' });

// ── Audit log (P2 #17) ────────────────────────────────────────────────────────

export interface AuditRow {
  id: string;
  admin_user_id?: string;
  admin_email?: string;
  action: string;
  entity_type: string;
  entity_id?: string;
  before?: string;
  after?: string;
  ip?: string;
  user_agent?: string;
  created_at: string;
}

export interface AuditList {
  items: AuditRow[];
  total: number;
}

export interface AuditFilters {
  action?: string;
  entity_type?: string;
  admin_user_id?: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
}

export const adminListAuditLog = (token: string, f: AuditFilters = {}) => {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(f)) {
    if (v == null || v === '') continue;
    qs.set(k, String(v));
  }
  const suffix = qs.toString() ? `?${qs.toString()}` : '';
  return request<AuditList>(`/admin/audit-log/${suffix}`, token);
};

// ── Stock movement log (進出記錄) ─────────────────────────────────────────────

export interface StockMovementRow extends VariantHistoryRow {
  product_id?: string;
  product_name?: string;
  variant_sku?: string;
  order_number?: string;
}

export interface StockMovementList {
  items: StockMovementRow[];
  total: number;
}

export interface StockMovementFilters {
  from?: string;
  to?: string;
  reason?: string;
  source?: 'admin' | 'order';
  product_id?: string;
  variant_id?: string;
  q?: string;
  actor_user_id?: string;
  limit?: number;
  offset?: number;
}

function stockHistoryQS(f: StockMovementFilters): string {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(f)) {
    if (v == null || v === '') continue;
    qs.set(k, String(v));
  }
  return qs.toString() ? `?${qs.toString()}` : '';
}

export const adminListStockHistory = (token: string, f: StockMovementFilters = {}) =>
  request<StockMovementList>(`/admin/stock-history/${stockHistoryQS(f)}`, token);

export const adminListProductStockHistory = (
  token: string,
  productID: string,
  f: StockMovementFilters = {}
) => request<StockMovementList>(`/admin/products/${productID}/stock-history${stockHistoryQS(f)}`, token);

// ── Email templates (P2 #20) ──────────────────────────────────────────────────

export interface EmailTemplateListItem {
  key: string;
  display_name: string;
  is_custom: boolean;
  is_enabled: boolean;
  updated_at?: string;
}

export interface EmailTemplateOverride {
  key: string;
  subject: string;
  html: string;
  text: string;
  is_enabled: boolean;
  updated_at: string;
  updated_by?: string;
}

export interface EmailTemplateDetail {
  key: string;
  display_name: string;
  override?: EmailTemplateOverride;
  defaults: { subject: string; html: string; text: string };
  variables: string[];
}

export const adminListEmailTemplates = (token: string) =>
  request<EmailTemplateListItem[]>('/admin/email-templates/', token);

export const adminGetEmailTemplate = (token: string, key: string) =>
  request<EmailTemplateDetail>(`/admin/email-templates/${key}`, token);

export const adminUpsertEmailTemplate = (
  token: string,
  key: string,
  body: { subject: string; html: string; text: string; is_enabled: boolean }
) =>
  request<EmailTemplateOverride>(`/admin/email-templates/${key}`, token, {
    method: 'PUT',
    body: JSON.stringify(body)
  });

export const adminResetEmailTemplate = (token: string, key: string) =>
  request<void>(`/admin/email-templates/${key}/reset`, token, { method: 'POST' });

export const adminTestEmailTemplate = (token: string, key: string, to: string) =>
  request<void>(`/admin/email-templates/${key}/test`, token, {
    method: 'POST',
    body: JSON.stringify({ to })
  });

export const adminPreviewEmailTemplate = (token: string, key: string) =>
  request<{ subject: string; html: string; text: string }>(`/admin/email-templates/${key}/preview`, token);

// ── Analytics (P2 #16) ────────────────────────────────────────────────────────

export interface RevenuePoint {
  date: string;
  revenue: number;
  order_count: number;
}

export interface TopProduct {
  variant_id?: string;
  product_name: string;
  variant_sku: string;
  qty_sold: number;
  revenue: number;
}

export interface TopCustomer {
  customer_id?: string;
  email: string;
  name: string;
  order_count: number;
  total_spent: number;
}

export interface StatusBreakdownPoint {
  status: string;
  count: number;
}

function rangeQs(from?: string, to?: string, extra: Record<string, string> = {}): string {
  const qs = new URLSearchParams(extra);
  if (from) qs.set('from', from);
  if (to) qs.set('to', to);
  return qs.toString() ? `?${qs.toString()}` : '';
}

export const adminGetRevenueTrend = (token: string, from?: string, to?: string) =>
  request<RevenuePoint[]>(`/admin/analytics/revenue${rangeQs(from, to)}`, token);

export const adminGetTopProducts = (token: string, from?: string, to?: string, by: 'qty' | 'revenue' = 'qty') =>
  request<TopProduct[]>(`/admin/analytics/top-products${rangeQs(from, to, { by })}`, token);

export const adminGetTopCustomers = (token: string, from?: string, to?: string) =>
  request<TopCustomer[]>(`/admin/analytics/top-customers${rangeQs(from, to)}`, token);

export const adminGetOrderStatusBreakdown = (token: string, from?: string, to?: string) =>
  request<StatusBreakdownPoint[]>(`/admin/analytics/order-status-breakdown${rangeQs(from, to)}`, token);

export const adminGetRefundTotal = (token: string, from?: string, to?: string) =>
  request<{ refunds: number }>(`/admin/analytics/refund-total${rangeQs(from, to)}`, token);

// ── Forms (CF7-style contact forms) ──────────────────────────────────────────

export interface AdminForm {
  id: string;
  slug: string;
  title: string;
  markup: string;
  fields: import('$lib/shortcodes/types').FormField[];

  mail_to: string;
  mail_from: string;
  mail_subject: string;
  mail_body: string;
  mail_reply_to: string;

  reply_enabled: boolean;
  reply_to_field: string;
  reply_from: string;
  reply_subject: string;
  reply_body: string;

  success_message: string;
  error_message: string;
  recaptcha_action: string;

  created_at: string;
  updated_at: string;
}

export interface FormParseError {
  position: number;
  tag?: string;
  message: string;
}

export interface FormSubmissionRow {
  id: string;
  form_id: string;
  data: Record<string, string>;
  ip?: string;
  user_agent?: string;
  recaptcha_score?: number;
  mail_sent: boolean;
  mail_error?: string;
  created_at: string;
}

export type UpsertFormBody = Omit<AdminForm, 'id' | 'fields' | 'created_at' | 'updated_at'>;

export const adminListForms = (token: string, limit = 50, offset = 0) => {
  const qs = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  return request<PagedResponse<AdminForm>>(`/admin/forms?${qs.toString()}`, token);
};

export const adminGetForm = (token: string, id: string) =>
  request<AdminForm>(`/admin/forms/${id}`, token);

export const adminCreateForm = async (
  token: string,
  body: UpsertFormBody
): Promise<{ ok: true; form: AdminForm } | { ok: false; parseErrors?: FormParseError[]; fields?: Record<string, string>; error?: string }> => {
  const res = await fetch(`${base()}/admin/forms`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body)
  });
  const json = await res.json().catch(() => ({}));
  if (!res.ok) return { ok: false, parseErrors: json.parse_errors, fields: json.fields, error: json.error };
  return { ok: true, form: json as AdminForm };
};

export const adminUpdateForm = async (
  token: string,
  id: string,
  body: UpsertFormBody
): Promise<{ ok: true; form: AdminForm } | { ok: false; parseErrors?: FormParseError[]; fields?: Record<string, string>; error?: string }> => {
  const res = await fetch(`${base()}/admin/forms/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify(body)
  });
  const json = await res.json().catch(() => ({}));
  if (!res.ok) return { ok: false, parseErrors: json.parse_errors, fields: json.fields, error: json.error };
  return { ok: true, form: json as AdminForm };
};

export const adminDeleteForm = (token: string, id: string) =>
  request(`/admin/forms/${id}`, token, { method: 'DELETE' });

export const adminListFormSubmissions = (token: string, id: string, limit = 50, offset = 0) =>
  request<{ items: FormSubmissionRow[]; total: number }>(
    `/admin/forms/${id}/submissions?limit=${limit}&offset=${offset}`,
    token
  );

export const adminGetFormSubmission = (token: string, sid: string) =>
  request<FormSubmissionRow>(`/admin/forms/submissions/${sid}`, token);

export const adminDeleteFormSubmission = (token: string, sid: string) =>
  request(`/admin/forms/submissions/${sid}`, token, { method: 'DELETE' });

// CSV export uses the admin token via Authorization header — the browser
// can't add custom headers to a plain <a download>, so callers fetch the
// blob and trigger a download via createObjectURL.
export const adminFormSubmissionsCsvURL = (id: string) => `/admin/forms/${id}/submissions.csv`;
