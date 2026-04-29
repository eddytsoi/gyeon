import type { Category, Order, Product, Variant, ProductImage } from '$lib/types';

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

// Products (admin uses same endpoints, protected by token for mutations)
export const adminGetProducts = (token: string, limit = 50, offset = 0) =>
  request<Product[]>(`/products?limit=${limit}&offset=${offset}`, token);

export const adminCreateProduct = (token: string, body: Partial<Product>) =>
  request<Product>('/products', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateProduct = (token: string, id: string, body: Partial<Product> & { is_active: boolean }) =>
  request<Product>(`/products/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeleteProduct = (token: string, id: string) =>
  request(`/products/${id}`, token, { method: 'DELETE' });

export const adminGetProduct = (token: string, id: string) =>
  request<Product>(`/products/${id}`, token);

export const adminGetVariants = (token: string, productID: string) =>
  request<Variant[]>(`/products/${productID}/variants`, token);

export const adminCreateVariant = (token: string, productID: string, body: Partial<Variant>) =>
  request<Variant>(`/products/${productID}/variants`, token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateVariant = (token: string, productID: string, variantID: string, body: Partial<Variant> & { is_active: boolean }) =>
  request<Variant>(`/products/${productID}/variants/${variantID}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeleteVariant = (token: string, productID: string, variantID: string) =>
  request(`/products/${productID}/variants/${variantID}`, token, { method: 'DELETE' });

export const adminAdjustStock = (token: string, productID: string, variantID: string, delta: number) =>
  request<Variant>(`/products/${productID}/variants/${variantID}/stock`, token, { method: 'POST', body: JSON.stringify({ delta }) });

export const adminGetImages = (token: string, productID: string) =>
  request<ProductImage[]>(`/products/${productID}/images`, token);

export const adminAddImage = (token: string, productID: string, body: Partial<ProductImage>) =>
  request<ProductImage>(`/products/${productID}/images`, token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateImage = (token: string, productID: string, imageID: string, body: Partial<ProductImage>) =>
  request<ProductImage>(`/products/${productID}/images/${imageID}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeleteImage = (token: string, productID: string, imageID: string) =>
  request(`/products/${productID}/images/${imageID}`, token, { method: 'DELETE' });

// Categories
export const adminGetCategories = (token: string) =>
  request<Category[]>('/categories', token);

export const adminCreateCategory = (token: string, body: Partial<Category>) =>
  request<Category>('/categories', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdateCategory = (token: string, id: string, body: Partial<Category> & { is_active: boolean }) =>
  request<Category>(`/categories/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeleteCategory = (token: string, id: string) =>
  request<void>(`/categories/${id}`, token, { method: 'DELETE' });

// Orders
export const adminGetOrders = (token: string, limit = 50, offset = 0) =>
  request<Order[]>(`/orders?limit=${limit}&offset=${offset}`, token);

export const adminGetOrder = (token: string, id: string) =>
  request<Order>(`/orders/${id}`, token);

export const adminUpdateOrderStatus = (token: string, id: string, status: string, note?: string) =>
  request<Order>(`/orders/${id}/status`, token, {
    method: 'POST',
    body: JSON.stringify({ status, note })
  });

export const adminDeleteOrder = (token: string, id: string) =>
  request(`/admin/orders/${id}`, token, { method: 'DELETE' });

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
  created_at: string;
  updated_at: string;
}

export interface CmsPost {
  id: string;
  number: number;
  category_id?: string;
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
export const adminGetPages = (token: string) =>
  request<CmsPage[]>('/admin/cms/pages', token);

export const adminGetPage = (token: string, id: string) =>
  request<CmsPage>(`/admin/cms/pages/${id}`, token);

export const adminCreatePage = (token: string, body: Partial<CmsPage>) =>
  request<CmsPage>('/admin/cms/pages', token, { method: 'POST', body: JSON.stringify(body) });

export const adminUpdatePage = (token: string, id: string, body: Partial<CmsPage> & { is_published: boolean }) =>
  request<CmsPage>(`/admin/cms/pages/${id}`, token, { method: 'PUT', body: JSON.stringify(body) });

export const adminDeletePage = (token: string, id: string) =>
  request(`/admin/cms/pages/${id}`, token, { method: 'DELETE' });

// Posts
export const adminGetPosts = (token: string, limit = 50, offset = 0) =>
  request<CmsPost[]>(`/admin/cms/posts?limit=${limit}&offset=${offset}`, token);

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

export const adminGetCustomers = (token: string, limit = 50, offset = 0) =>
  request<Customer[]>(`/admin/customers?limit=${limit}&offset=${offset}`, token);

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

export const adminTestShipanyConnection = (token: string) =>
  request<{ ok: boolean; message: string }>('/admin/shipany/test-connection', token, { method: 'POST' });

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

export const adminGetUsers = (token: string) =>
  request<AdminUser[]>('/admin/users', token);

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
}

export const adminGetMedia = (token: string) =>
  request<MediaFile[]>('/admin/media', token);

export const adminGetMediaFile = (token: string, id: string) =>
  request<MediaFile>(`/admin/media/${id}`, token);

export const adminUpdateMedia = (
  token: string,
  id: string,
  body: { original_name?: string; url?: string }
) =>
  request<MediaFile>(`/admin/media/${id}`, token, {
    method: 'PATCH',
    body: JSON.stringify(body)
  });

export const adminDeleteMedia = (token: string, id: string) =>
  request(`/admin/media/${id}`, token, { method: 'DELETE' });

export const adminAddMediaLink = (token: string, url: string, name: string) =>
  request<MediaFile>('/admin/media/link', token, {
    method: 'POST',
    body: JSON.stringify({ url, name })
  });

export const adminUploadMedia = async (token: string, file: File): Promise<MediaFile> => {
  const formData = new FormData();
  formData.append('file', file);
  const res = await fetch(`${base()}/admin/media/upload`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
    body: formData
  });
  if (!res.ok) {
    const msg = await res.text().catch(() => `${res.status}`);
    throw new Error(msg || `Upload failed: ${res.status}`);
  }
  return res.json() as Promise<MediaFile>;
};
