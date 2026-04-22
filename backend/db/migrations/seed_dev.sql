-- ============================================================
-- Development seed data
-- ============================================================

-- Categories
INSERT INTO categories (id, slug, name, description, sort_order) VALUES
  ('11111111-0000-0000-0000-000000000001', 'clothing',     'Clothing',      'Apparel for all occasions',    1),
  ('11111111-0000-0000-0000-000000000002', 'accessories',  'Accessories',   'Bags, wallets and more',       2),
  ('11111111-0000-0000-0000-000000000003', 'home-living',  'Home & Living', 'Everyday home essentials',     3);

-- Products
INSERT INTO products (id, category_id, slug, name, description) VALUES
  ('22222222-0000-0000-0000-000000000001',
   '11111111-0000-0000-0000-000000000001',
   'classic-white-tee',
   'Classic White Tee',
   'A timeless staple. Crafted from 100% organic cotton with a relaxed fit that works for any occasion.'),

  ('22222222-0000-0000-0000-000000000002',
   '11111111-0000-0000-0000-000000000001',
   'black-denim-jacket',
   'Black Denim Jacket',
   'Raw selvedge denim in midnight black. Structured silhouette, brass hardware, two chest pockets.'),

  ('22222222-0000-0000-0000-000000000003',
   '11111111-0000-0000-0000-000000000001',
   'merino-crewneck',
   'Merino Crewneck',
   'Superfine merino wool. Warm without bulk, packable, and naturally odour-resistant.'),

  ('22222222-0000-0000-0000-000000000004',
   '11111111-0000-0000-0000-000000000002',
   'slim-leather-wallet',
   'Slim Leather Wallet',
   'Full-grain vegetable-tanned leather. Holds 6 cards and cash. Gets better with age.'),

  ('22222222-0000-0000-0000-000000000005',
   '11111111-0000-0000-0000-000000000002',
   'canvas-tote',
   'Canvas Tote Bag',
   'Heavy-duty 16oz canvas. Reinforced handles, internal zip pocket. Built to last.'),

  ('22222222-0000-0000-0000-000000000006',
   '11111111-0000-0000-0000-000000000003',
   'ceramic-mug',
   'Handmade Ceramic Mug',
   'Wheel-thrown stoneware, food-safe glaze. Each piece is slightly unique. Dishwasher safe.'),

  ('22222222-0000-0000-0000-000000000007',
   '11111111-0000-0000-0000-000000000003',
   'linen-cushion-cover',
   'Linen Cushion Cover',
   'Pre-washed Belgian linen. Hidden zip. 50×50cm. The more you wash it, the softer it gets.');

-- ============================================================
-- Variants
-- ============================================================

-- Classic White Tee (S / M / L / XL)
INSERT INTO product_variants (id, product_id, sku, price, stock_qty) VALUES
  ('33333333-0000-0000-0001-000000000001', '22222222-0000-0000-0000-000000000001', 'CWT-S',  199.00, 20),
  ('33333333-0000-0000-0001-000000000002', '22222222-0000-0000-0000-000000000001', 'CWT-M',  199.00, 30),
  ('33333333-0000-0000-0001-000000000003', '22222222-0000-0000-0000-000000000001', 'CWT-L',  199.00, 25),
  ('33333333-0000-0000-0001-000000000004', '22222222-0000-0000-0000-000000000001', 'CWT-XL', 199.00,  8);

-- Black Denim Jacket (S / M / L) — with compare price
INSERT INTO product_variants (id, product_id, sku, price, compare_at_price, stock_qty) VALUES
  ('33333333-0000-0000-0002-000000000001', '22222222-0000-0000-0000-000000000002', 'BDJ-S', 680.00, 880.00, 5),
  ('33333333-0000-0000-0002-000000000002', '22222222-0000-0000-0000-000000000002', 'BDJ-M', 680.00, 880.00, 8),
  ('33333333-0000-0000-0002-000000000003', '22222222-0000-0000-0000-000000000002', 'BDJ-L', 680.00, 880.00, 3);

-- Merino Crewneck (S / M / L) in two colours
INSERT INTO product_variants (id, product_id, sku, price, stock_qty) VALUES
  ('33333333-0000-0000-0003-000000000001', '22222222-0000-0000-0000-000000000003', 'MCN-OATMEAL-S', 450.00, 12),
  ('33333333-0000-0000-0003-000000000002', '22222222-0000-0000-0000-000000000003', 'MCN-OATMEAL-M', 450.00, 15),
  ('33333333-0000-0000-0003-000000000003', '22222222-0000-0000-0000-000000000003', 'MCN-NAVY-S',    450.00, 10),
  ('33333333-0000-0000-0003-000000000004', '22222222-0000-0000-0000-000000000003', 'MCN-NAVY-M',    450.00,  7);

-- Slim Leather Wallet (Black / Tan)
INSERT INTO product_variants (id, product_id, sku, price, stock_qty) VALUES
  ('33333333-0000-0000-0004-000000000001', '22222222-0000-0000-0000-000000000004', 'SLW-BLK', 320.00, 18),
  ('33333333-0000-0000-0004-000000000002', '22222222-0000-0000-0000-000000000004', 'SLW-TAN', 320.00, 14);

-- Canvas Tote (Natural / Black) — with sale price
INSERT INTO product_variants (id, product_id, sku, price, compare_at_price, stock_qty) VALUES
  ('33333333-0000-0000-0005-000000000001', '22222222-0000-0000-0000-000000000005', 'CTB-NAT', 150.00, 200.00, 40),
  ('33333333-0000-0000-0005-000000000002', '22222222-0000-0000-0000-000000000005', 'CTB-BLK', 150.00, 200.00, 35);

-- Ceramic Mug (White / Charcoal)
INSERT INTO product_variants (id, product_id, sku, price, stock_qty) VALUES
  ('33333333-0000-0000-0006-000000000001', '22222222-0000-0000-0000-000000000006', 'CMG-WHT', 98.00, 22),
  ('33333333-0000-0000-0006-000000000002', '22222222-0000-0000-0000-000000000006', 'CMG-CHR', 98.00, 15);

-- Linen Cushion Cover (Natural / Slate)
INSERT INTO product_variants (id, product_id, sku, price, stock_qty) VALUES
  ('33333333-0000-0000-0007-000000000001', '22222222-0000-0000-0000-000000000007', 'LCC-NAT', 188.00, 20),
  ('33333333-0000-0000-0007-000000000002', '22222222-0000-0000-0000-000000000007', 'LCC-SLT', 188.00, 12);

-- ============================================================
-- Images  (picsum.photos — consistent per seed key)
-- ============================================================
INSERT INTO product_images (product_id, url, alt_text, sort_order, is_primary) VALUES
  ('22222222-0000-0000-0000-000000000001', 'https://picsum.photos/seed/cwt/600/600',   'Classic White Tee',    0, TRUE),
  ('22222222-0000-0000-0000-000000000002', 'https://picsum.photos/seed/bdj/600/600',   'Black Denim Jacket',   0, TRUE),
  ('22222222-0000-0000-0000-000000000002', 'https://picsum.photos/seed/bdj2/600/600',  'Black Denim Jacket 2', 1, FALSE),
  ('22222222-0000-0000-0000-000000000003', 'https://picsum.photos/seed/mcn/600/600',   'Merino Crewneck',      0, TRUE),
  ('22222222-0000-0000-0000-000000000004', 'https://picsum.photos/seed/slw/600/600',   'Slim Leather Wallet',  0, TRUE),
  ('22222222-0000-0000-0000-000000000005', 'https://picsum.photos/seed/ctb/600/600',   'Canvas Tote Bag',      0, TRUE),
  ('22222222-0000-0000-0000-000000000006', 'https://picsum.photos/seed/cmg/600/600',   'Ceramic Mug',          0, TRUE),
  ('22222222-0000-0000-0000-000000000007', 'https://picsum.photos/seed/lcc/600/600',   'Linen Cushion Cover',  0, TRUE);
