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
  status: string;
  kind?: string; // 'simple' | 'bundle'
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

export interface Variant {
  id: string;
  product_id: string;
  sku: string;
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
  discount_amount: number;
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
  postal_code: string;
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
