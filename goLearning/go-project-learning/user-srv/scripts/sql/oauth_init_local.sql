-- =====================================================
-- OAuth2.0 测试数据 - 本地环境
-- =====================================================
-- 使用方法:
--   mysql -u root -p123456 userdb < oauth_init_local.sql
-- =====================================================

-- 1. 创建 OAuth 客户端（商城前端）
INSERT INTO oauth_clients (client_id, client_secret, name, redirect_uris, scope, created_at, updated_at, deleted_at)
VALUES (
    'mall-frontend',
    'mall-frontend-secret-change-in-prod',
    '商城前端',
    '["http://localhost:3000/callback", "http://localhost:8080/callback"]',
    'read write',
    NOW(),
    NOW(),
    NULL
) ON DUPLICATE KEY UPDATE
    client_secret = 'mall-frontend-secret-change-in-prod',
    name = '商城前端',
    redirect_uris = '["http://localhost:3000/callback", "http://localhost:8080/callback"]',
    scope = 'read write',
    updated_at = NOW();

-- 2. 创建 OAuth 客户端（管理后台）
INSERT INTO oauth_clients (client_id, client_secret, name, redirect_uris, scope, created_at, updated_at, deleted_at)
VALUES (
    'admin-portal',
    'admin-portal-secret-change-in-prod',
    '管理后台',
    '["http://localhost:3001/callback", "http://localhost:8081/callback"]',
    'read write admin',
    NOW(),
    NOW(),
    NULL
) ON DUPLICATE KEY UPDATE
    client_secret = 'admin-portal-secret-change-in-prod',
    name = '管理后台',
    redirect_uris = '["http://localhost:3001/callback", "http://localhost:8081/callback"]',
    scope = 'read write admin',
    updated_at = NOW();

-- 3. 创建 OAuth 客户端（移动端 APP）
INSERT INTO oauth_clients (client_id, client_secret, name, redirect_uris, scope, created_at, updated_at, deleted_at)
VALUES (
    'mobile-app',
    'mobile-app-secret-change-in-prod',
    '移动端APP',
    '["myapp://callback", "exp://localhost:19000/--/callback"]',
    'read write',
    NOW(),
    NOW(),
    NULL
) ON DUPLICATE KEY UPDATE
    client_secret = 'mobile-app-secret-change-in-prod',
    name = '移动端APP',
    redirect_uris = '["myapp://callback", "exp://localhost:19000/--/callback"]',
    scope = 'read write',
    updated_at = NOW();

-- 4. 创建测试用户（如果不存在）
-- 密码: 123456 (bcrypt 加密)
INSERT IGNORE INTO users (username, password, phone, email, status, created_at, updated_at, deleted_at)
VALUES (
    'testuser',
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    '13800138000',
    'test@example.com',
    1,
    NOW(),
    NOW(),
    NULL
);

-- 5. 创建另一个测试用户
INSERT IGNORE INTO users (username, password, phone, email, status, created_at, updated_at, deleted_at)
VALUES (
    'admin',
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    '13800138001',
    'admin@example.com',
    1,
    NOW(),
    NOW(),
    NULL
);

-- 6. 清理过期数据（可选）
DELETE FROM authorization_codes WHERE expires_at < NOW();
DELETE FROM o_auth_access_tokens WHERE expires_at < NOW();
DELETE FROM o_auth_refresh_tokens WHERE expires_at < NOW();

-- =====================================================
-- 验证数据
-- =====================================================

-- 查看 OAuth 客户端
SELECT '=== OAuth 客户端 ===' AS '';
SELECT client_id, name, scope FROM oauth_clients WHERE deleted_at IS NULL;

-- 查看测试用户
SELECT '=== 测试用户 ===' AS '';
SELECT id, username, phone, email FROM users WHERE deleted_at IS NULL;
