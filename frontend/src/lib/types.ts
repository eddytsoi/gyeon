export interface Category {
  id: string;
  parent_id?: string;
  slug: string;
  name: string;
  description?: string;
  media_file_id?: string;
  image_url?: string;
  desktop_banner_url?: string;
  mobile_banner_url?: string;
  sort_order: number;
  is_active: boolean;
}

export interface Product {
  id: string;
  number: number;
  category_id?: string;
  category_ids?: string[];
  slug: string;
  name: string;
  subtitle?: string;
  excerpt?: string;
  description?: string;
  how_to_use?: string;
  compatible_surfaces?: string[];
  // Original WooCommerce product SKU captured at import (separate from any
  // generated value). Null/absent for manually-created products.
  wc_sku?: string | null;
  // Hero video + 6 banner / media slot IDs. URL projections (banner_1_url,
  // banner_1_webp_url, …) are populated only on single-product detail reads
  // (GetBySlug / GetByID); list views leave them nil.
  video_id?: string | null;
  banner_1_media_id?: string | null;
  banner_2_media_id?: string | null;
  media_1_media_id?: string | null;
  media_2_media_id?: string | null;
  media_3_media_id?: string | null;
  media_4_media_id?: string | null;
  banner_1_url?: string | null;
  banner_1_webp_url?: string | null;
  banner_2_url?: string | null;
  banner_2_webp_url?: string | null;
  media_1_url?: string | null;
  media_1_webp_url?: string | null;
  media_2_url?: string | null;
  media_2_webp_url?: string | null;
  media_3_url?: string | null;
  media_3_webp_url?: string | null;
  media_4_url?: string | null;
  media_4_webp_url?: string | null;
  status: string;
  kind?: string; // 'simple' | 'bundle'
  // True when the current request's storefront role is allowed to add this
  // product to cart. False means at least one of the product's categories
  // is blocked-for-purchase for the role — price + add-to-cart should hide.
  // Defaults to true when omitted (older clients, untouched admin paths).
  purchasable?: boolean;
  // Per-product override of the site-wide `pdp_taobao_layout_enabled`
  // flag. null/undefined = follow site default, true = force taobao
  // modal, false = force classic inline PDP.
  use_taobao_layout?: boolean | null;
  created_at: string;
  updated_at: string;
  // List-endpoint enrichments (ProductWithMeta on the backend)
  variant_count?: number;
  primary_image_url?: string | null;
  default_variant_id?: string | null;
  default_variant_price?: number | null;
  default_variant_compare_at_price?: number | null;
  default_variant_stock_qty?: number | null;
  default_variant_name?: string | null;
  min_price?: number | null;
  min_compare_at_price?: number | null;
  min_price_stock_qty?: number | null;
}

export interface BundleItem {
  id: string;
  bundle_product_id: string;
  component_variant_id: string;
  quantity: number;
  sort_order: number;
  display_name_override?: string;
  // joined fields
  component_product_name?: string;
  component_product_slug?: string;
  component_product_subtitle?: string;
  component_sku?: string;
  component_variant_name?: string;
  component_price?: number;
  component_stock_qty?: number;
  component_primary_image_url?: string;
}

// PromoBundle is a bundle product associated to a parent product as one
// of the "優惠套裝" rows shown inside the taobao-layout PDP modal. Each
// row is flattened with the bundle's default variant (price /
// compare_at_price / variant_id / stock) and its primary image so the
// storefront can render the modal in a single fetch.
export interface PromoBundle {
  id: string;
  parent_product_id: string;
  bundle_product_id: string;
  sort_order: number;
  slug: string;
  name: string;
  excerpt?: string | null;
  status: string;
  variant_id: string;
  price: number;
  compare_at_price?: number | null;
  stock_qty: number;
  primary_image_url?: string | null;
  created_at: string;
  // Stamped by the backend per the storefront role's purchase-block rules.
  // Default true; false means the underlying bundle product sits in a
  // category the role can't buy from. UI should disable the row instead of
  // calling cart-add (which would 403).
  purchasable?: boolean;
}

export interface Variant {
  id: string;
  product_id: string;
  sku: string;
  // Original WooCommerce variant SKU captured at import (separate from the
  // generated sku). Null/absent for manually-created variants.
  wc_sku?: string | null;
  name?: string;
  price: number;
  compare_at_price?: number;
  stock_qty: number;
  low_stock_threshold?: number;
  weight_grams?: number;
  length_mm?: number;
  width_mm?: number;
  height_mm?: number;
  is_active: boolean;
  product_name?: string;
  image_url?: string;
}

export interface ProductImage {
  id: string;
  product_id: string;
  variant_id?: string;
  media_file_id?: string;
  url: string;
  mime_type?: string;
  thumbnail_url?: string;
  video_autoplay?: boolean;
  video_fit?: 'contain' | 'cover';
  alt_text?: string;
  sort_order: number;
  is_primary: boolean;
}

export interface CartItem {
  id: string;
  cart_id: string;
  variant_id: string;
  quantity: number;
  added_at: string;
  product_name: string;
  product_slug: string;
  sku: string;
  variant_name?: string | null;
  price: number;
  weight_grams?: number;
  image_url?: string;
  kind?: 'simple' | 'bundle';
  children?: CartItemChild[];
}

export interface CartItemChild {
  product_name: string;
  product_slug: string;
  sku: string;
  variant_name?: string | null;
  quantity: number;
  image_url?: string;
}

export interface Cart {
  id: string;
  customer_id?: string;
  session_token?: string;
  items: CartItem[];
}

export interface OrderItem {
  id: string;
  order_id: string;
  variant_id?: string;
  parent_item_id?: string | null;
  product_name: string;
  variant_sku: string;
  variant_attrs?: Record<string, string>;
  unit_price: number;
  quantity: number;
  line_total: number;
}

export interface ShippingAddress {
  first_name: string;
  last_name: string;
  phone?: string;
  line1: string;
  line2?: string;
  city: string;
  state?: string;
  postal_code: string;
  country: string;
}

export interface OrderAppliedPromotion {
  kind: 'campaign' | 'coupon';
  id: string;
  name: string;
  description?: string | null;
  amount: number;
}

export interface Order {
  id: string;
  number: number;
  order_number: string;
  customer_id?: string;
  status: string;
  shipping_address_id?: string;
  shipping_address?: ShippingAddress;
  subtotal: number;
  shipping_fee: number;
  shipping_free?: boolean;
  discount_amount: number;
  applied_promotions?: OrderAppliedPromotion[];
  tax_amount?: number;
  total: number;
  notes?: string;
  customer_email?: string;
  customer_phone?: string;
  customer_name?: string;
  payment_intent_id?: string;
  payment_status?: string;
  payment_method?: string;
  card_brand?: string;
  card_last4?: string;
  paid_at?: string;
  refund_amount?: number;
  refund_reason?: string;
  refunded_at?: string;
  stripe_refund_id?: string;
  selected_carrier?: string;
  selected_service?: string;
  pickup_point_id?: string;
  pickup_point_label?: string;
  items: OrderItem[];
  items_count?: number;
  customer_role?: string;
  created_at: string;
}

export type NoticeRole = 'system' | 'admin' | 'customer';

export interface OrderNotice {
  id: string;
  order_id: string;
  role: NoticeRole;
  status?: string;
  body: string;
  author_id?: string;
  read_at?: string;
  created_at: string;
}

export interface PaymentConfig {
  publishable_key: string;
  mode: 'test' | 'live';
  country: string;
}

export interface CheckoutResult {
  order: Order;
  client_secret: string;
  publishable_key: string;
  mode: 'test' | 'live';
  setup_client_secret?: string;
}

export interface SavedPaymentMethod {
  id: string;
  customer_id: string;
  stripe_pm_id: string;
  brand: string;
  last4: string;
  exp_month: number;
  exp_year: number;
  is_default: boolean;
  created_at: string;
}

export interface CustomerInfoInput {
  first_name: string;
  last_name?: string;
  email: string;
  phone: string;
}

export interface ShippingAddressInput {
  line1: string;
  line2?: string;
  city: string;
  state?: string;
  postal_code?: string;
  country: string;
}

export interface CmsPage {
  id: string;
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

export type CustomerRole = 'customer' | 'installer';

/**
 * Map a CustomerRole to its localised label. Reuses the admin role labels
 * so we don't duplicate the same translation in two namespaces. Anonymous
 * visitors (no role) are treated as `customer`, matching backend behaviour.
 */
import * as m from '$lib/paraglide/messages';
export function customerRoleLabel(role: CustomerRole | string | null | undefined): string {
  return role === 'installer' ? m.admin_role_installer() : m.admin_role_customer();
}

export interface Customer {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  phone?: string;
  is_active: boolean;
  role: CustomerRole;
  created_at: string;
  updated_at: string;
}

export interface CategoryRule {
  role: CustomerRole;
  category_id: string;
  /** PDP resolves and product appears everywhere for this role. */
  can_view: boolean;
  /**
   * Product appears in listings / category nav / search. FALSE = "private
   * link" — PDP-by-slug still works, but the category is hidden from public
   * discovery. The per-role replacement for the pre-migration-103 global
   * `hidden_category_ids` setting.
   */
  is_listed: boolean;
  /** Cart-add accepts variants in this category. */
  can_purchase: boolean;
}

export interface Address {
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

export interface CmsPost {
  id: string;
  category_id?: string;
  category_ids?: string[];
  category_slug?: string;
  category_name?: string;
  slug: string;
  title: string;
  excerpt?: string;
  content: string;
  cover_media_file_id?: string;
  cover_image_url?: string;
  is_published: boolean;
  published_at?: string;
  created_at: string;
  updated_at: string;
}

export interface SocialMediaEntry {
  icon: string;
  url: string;
  label?: string;
  customSvgPath?: string;
}
