updated: 2026-05-14

# Domain Migration Guide

How to migrate the Gyeon production deployment to a new public domain (e.g. `gyeon.hk` → `new-domain.com`).

## TL;DR

整個 codebase 對轉 domain 友善 — 所有 internal links 都是 relative，絕對 URL 全部從 **env var** 或 **DB setting** 動態組裝。**不需要改 code**，只要更新兩處設定 + 幾個第三方面板。

## Path / URL audit (current state)

| 類型 | 來源 | 狀態 |
|---|---|---|
| Internal `<a href="/...">`、`goto()`、`redirect()` | hardcoded relative | ✅ relative |
| API 呼叫 (`/api/v1/...`) | `API_BASE` env or relative | ✅ relative |
| 媒體 (`/uploads/...`) | `BASE_URL` env (backend) | ✅ 從 env 組裝 |
| Sitemap / robots / canonical / og:url | `public_base_url` DB setting | ✅ runtime 組裝 |
| Email 內 reset / order 連結 | `public_base_url` DB setting | ✅ runtime 組裝 |
| MCP discovery (`/.well-known/mcp.json`、`<link rel="mcp">`) | `BASE_URL` env + relative | ✅ 從 env 組裝 |
| Cookie domain | 未設 `Domain:` attribute | ✅ 自動跟隨新 domain |
| CORS | `Access-Control-Allow-Origin: *` | ✅ 無 domain 綁定 |

沒有任何 hardcoded `gyeon.hk` 字串；DB migrations 沒寫死 domain（seed 只用 `picsum.photos` 佔位圖）。

## ⚠️ 最容易踩的坑

`BASE_URL`（env）和 `public_base_url`（DB setting）是**兩個獨立來源**，要**同時**更新：

- `BASE_URL` env → 媒體絕對 URL、MCP endpoint、OpenAPI response
- `public_base_url` DB setting → sitemap、robots、canonical、og:url、email 連結

只改一邊會出現「email 用舊 domain、圖片用新 domain」這種混雜狀況。

---

## Migration checklist

### 🔴 必做 — Codebase / Server config

- [ ] **更新 `BASE_URL` env var**（影響 media URL、MCP discovery）
  - 位置：GCP VM `/opt/gyeon/.env`
  - 值：`https://new-domain.com`（production 必須 HTTPS）
  - 之後 `docker compose -f docker-compose.prod.yml --env-file .env up -d` 重啟

- [ ] **更新 `ORIGIN` env var**（frontend SvelteKit form CSRF origin check）
  - 位置：同上 `.env`
  - 值：`https://new-domain.com`
  - 注意：`docker-compose.prod.yml` 將 `ORIGIN` 設為 `${BASE_URL}`，所以改 `BASE_URL` 已連帶處理；但要 verify

- [ ] **不要改 `API_BASE`**
  - 這是 frontend container → backend container 的內部 URL（`http://backend:8080/api/v1`）
  - 用 Docker 內部 hostname，與 public domain 無關

### 🔴 必做 — Admin Settings (DB)

- [ ] 登入 `/admin/settings` 更新 `public_base_url`
  - 值：`https://new-domain.com`
  - 影響：sitemap、robots、canonical link、og:url、email 內所有連結

### 🔴 必做 — 第三方面板（codebase 外）

- [ ] **Google reCAPTCHA**：admin console 加新 domain 到 Authorized Domains
  - Site key / secret key 不變
  - 連結見 `/admin/settings` 的 reCAPTCHA section

- [ ] **Stripe**：Dashboard → Webhooks 更新 callback URL 為 `https://new-domain.com/...`

- [ ] **Shipany**（如有 webhook callback）：更新 callback URL

- [ ] **DNS / SSL cert**：
  - 新 domain 的 A record 指向 GCP VM external IP
  - 簽發 SSL cert（Let's Encrypt / Cloud Load Balancer / Cloudflare）

### 🟡 建議

- [ ] **nginx HSTS header**（HTTPS 後加強安全性）
  - 編輯 `nginx/nginx.conf`，在 server block 加：
    ```
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    ```
  - nginx 本身用 `$host`，**不需要**改 server_name 以外的設定

- [ ] **舊 domain redirect**（SEO 友善的轉移）
  - 在 nginx 設一個 server block：舊 domain 所有 path 301 redirect 到新 domain
  - 保留至少 3–6 個月

- [ ] **重新提交 sitemap**：
  - Google Search Console / Bing Webmaster 加入新 domain 的 property
  - 提交 `https://new-domain.com/sitemap.xml`

### 🟢 不需要做（已自動處理）

- ✅ Codebase 完全不需要改
- ✅ Database content 不需要改（沒寫死 domain）
- ✅ Cookie 會自動 scope 到新 domain（舊 session 失效，用戶要重新登入 — 這是正常的）
- ✅ Media files (`/uploads/*`) 不需要搬移，新 domain 自動 serve

---

## 部署完成後的驗證

1. **Sitemap 檢查**
   ```bash
   curl -s https://new-domain.com/sitemap.xml | head -20
   ```
   確認所有 `<loc>` 都是新 domain。

2. **Email 連結檢查**
   - 從 `/admin/customers` 觸發一封 password reset email
   - 確認 email 內 reset 連結指向新 domain

3. **Media 載入檢查**
   - 打開首頁 (`https://new-domain.com/`)
   - DevTools → Network，確認所有 `/uploads/...` 圖片 200 且來自新 domain

4. **MCP discovery 檢查**
   ```bash
   curl -s https://new-domain.com/.well-known/mcp.json
   ```
   應該回傳 `{"mcp_endpoint":"https://new-domain.com/mcp/sse",...}`。

5. **HTML meta 檢查**
   - View source on `https://new-domain.com/products/<slug>`
   - 確認 `<link rel="canonical">`、`<meta property="og:url">` 都是新 domain

6. **reCAPTCHA 檢查**
   - 嘗試在新 domain 提交 contact form
   - 如果出現 reCAPTCHA error，回 Google admin console 加 domain

---

## 關鍵檔案參考

| 設定 | 檔案 | 用途 |
|---|---|---|
| `BASE_URL` env reading | [backend/cmd/api/main.go:126](backend/cmd/api/main.go) | backend bootstrap |
| `public_base_url` setting | [backend/internal/email/service.go:56](backend/internal/email/service.go) | email link builder |
| `API_BASE` env reading | [frontend/src/hooks.server.ts:6](frontend/src/hooks.server.ts) | SvelteKit → API proxy |
| MCP discovery endpoint | [backend/cmd/api/main.go:327](backend/cmd/api/main.go) | `/.well-known/mcp.json` |
| MCP discovery `<link>` tag | [frontend/src/app.html:14](frontend/src/app.html) | browser-side MCP discovery |
| Sitemap builder | [frontend/src/routes/sitemap.xml/+server.ts:23](frontend/src/routes/sitemap.xml/+server.ts) | SEO |
| Production env template | [.env.example:15](.env.example) | 部署時參考 |
| Production compose | [docker-compose.prod.yml:29](docker-compose.prod.yml) | `BASE_URL` / `ORIGIN` 注入 |
| nginx 設定 | [nginx/nginx.conf](nginx/nginx.conf) | 用 `$host`，無 hardcoded domain |
