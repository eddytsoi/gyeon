-- ============================================================
-- Payment module: Stripe integration
-- ============================================================

-- Payment columns on orders
ALTER TABLE orders
    ADD COLUMN payment_intent_id VARCHAR(255),
    ADD COLUMN payment_status    VARCHAR(50),
    ADD COLUMN payment_method    VARCHAR(50);

CREATE INDEX idx_orders_payment_intent_id ON orders(payment_intent_id);

-- Snapshot the customer email/phone/name on the order itself.
-- For guest checkouts these duplicate the customers row we upsert,
-- but keeping them here means order detail pages don't depend on the
-- customers table for contact info (which can change later).
ALTER TABLE orders
    ADD COLUMN customer_email VARCHAR(255),
    ADD COLUMN customer_phone VARCHAR(50),
    ADD COLUMN customer_name  VARCHAR(255);

-- Payment settings (Stripe)
INSERT INTO site_settings (key, value, description) VALUES
    ('stripe_mode',                 'test',  'Stripe runtime mode: test or live'),
    ('stripe_test_publishable_key', '',      'Stripe test publishable key (pk_test_...)'),
    ('stripe_test_secret_key',      '',      'Stripe test secret key (sk_test_...)'),
    ('stripe_live_publishable_key', '',      'Stripe live publishable key (pk_live_...)'),
    ('stripe_live_secret_key',      '',      'Stripe live secret key (sk_live_...)'),
    ('stripe_save_cards',           'false', 'Allow customers to save cards (UI toggle only for v1)'),
    ('stripe_webhook_secret',       '',      'Stripe webhook signing secret (whsec_...)')
ON CONFLICT (key) DO NOTHING;

-- SMTP settings (Gmail) for transactional email
INSERT INTO site_settings (key, value, description) VALUES
    ('smtp_host',       'smtp.gmail.com', 'SMTP server hostname'),
    ('smtp_port',       '587',            'SMTP server port (587 = STARTTLS)'),
    ('smtp_username',   '',               'SMTP username (Gmail address)'),
    ('smtp_password',   '',               'SMTP password (Gmail App Password — 16 chars, no spaces)'),
    ('smtp_from_email', '',               'Default From email address'),
    ('smtp_from_name',  'Gyeon',          'Default From display name'),
    ('public_base_url', 'http://localhost:5173', 'Public storefront base URL (used for email links)')
ON CONFLICT (key) DO NOTHING;

-- Account setup tokens (one-time, used by guest checkouts to set a password)
CREATE TABLE account_setup_tokens (
    token        VARCHAR(64) PRIMARY KEY,
    customer_id  UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    expires_at   TIMESTAMPTZ NOT NULL,
    consumed_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_account_setup_tokens_customer_id ON account_setup_tokens(customer_id);

-- Terms & Conditions CMS page (formal zh-HK content; sites can edit later via admin)
INSERT INTO cms_pages (slug, title, content, meta_title, meta_desc, is_published)
VALUES (
    'terms-and-conditions',
    '條款與條件',
$$# 條款與條件

歡迎使用本網站。請於使用本網站之服務前，仔細閱讀以下條款與條件。當閣下瀏覽、註冊或進行交易，即表示同意接受本條款之約束。

## 一、接受條款
本條款構成閣下與本公司之間具法律約束力之協議。如不同意任何條款，請立即停止使用本網站。

## 二、服務說明
本網站提供商品展示、線上訂購、付款及配送等服務。本公司保留隨時修改、暫停或終止任何服務之權利，毋須事先通知。

## 三、訂單與付款
- 所有訂單須經本公司確認後方為有效。
- 商品價格以下單時網站顯示之金額為準，並包含適用稅項。
- 付款由 Stripe 安全處理；本公司不會儲存閣下之信用卡完整資料。
- 付款成功後，本公司將以電子郵件向閣下發送訂單確認；訪客客戶之電郵內亦會附上一次性連結，方便閣下完成帳戶註冊。
- 如付款未能於合理時間內完成，本公司有權取消訂單並釋放所保留之庫存。

## 四、運送與交貨
本公司將於收妥款項後安排配送。送達時間視乎地區及物流安排，僅供參考，並不構成保證。

## 五、退換貨政策
- 客戶於收貨後七日內，如商品有品質問題，可申請退換。
- 退換之商品須保持原狀及完整包裝。
- 個人衛生用品、定製商品恕不退換。

## 六、知識產權
本網站之所有內容，包括但不限於文字、圖片、商標、標誌及程式碼，均屬本公司或其授權人所有；未經書面同意，不得複製、轉載或作任何商業用途。

## 七、免責聲明
本公司已盡力確保網站資訊之準確性，惟對因使用本網站而引致之任何直接或間接損失，概不承擔法律責任。

## 八、私隱政策
本公司依照《個人資料（私隱）條例》處理閣下之個人資料，詳情請參閱本網站之私隱聲明。

## 九、法律管轄
本條款受香港特別行政區法律管轄；雙方同意以香港法院為唯一具管轄權之法院。

## 十、條款修改
本公司保留隨時修改本條款之權利；修改後之條款於網站發佈時即時生效。建議閣下定期查閱本頁。

如有任何疑問，歡迎聯絡本公司客戶服務部。
$$,
    '條款與條件',
    '本網站之服務條款與條件',
    TRUE
)
ON CONFLICT (slug) DO NOTHING;
