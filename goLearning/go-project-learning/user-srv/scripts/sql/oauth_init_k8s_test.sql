-- =====================================================
-- OAuth2.0 测试数据 - K8s 测试环境
-- =====================================================
-- 使用方法:
--   kubectl exec -it deployment/mysql -n go-project-test -- \
--     mysql -uroot -p123456 userdb_test < oauth_init_k8s_test.sql
-- =====================================================

-- 1. 创建 OAuth 客户端（商城前端）
INSERT INTO oauth_clients (client_id, client_secret, name, redirect_uris, scope, created_at, updated_at, deleted_at)
VALUES (
    'mall-frontend',
    'mall-frontend-secret-k8s-test',
    '商城前端',
    '["http://mall.test.com:3000/callback", "http://mall.test.com:8080/callback"]',
    'read write',
    NOW(),
    NOW(),
    NULL
) ON DUPLICATE KEY UPDATE
    client_secret = 'mall-frontend-secret-k8s-test',
    name = '商城前端',
    redirect_uris = '["http://mall.test.com:3000/callback", "http://mall.test.com:8080/callback"]',
    scope = 'read write',
    updated_at = NOW();

-- 2. 创建 OAuth 客户端（管理后台）
INSERT INTO oauth_clients (client_id, client_secret, name, redirect_uris, scope, created_at, updated_at, deleted_at)
VALUES (
    'admin-portal',
    'admin-portal-secret-k8s-test',
    '管理后台',
    '["http://admin.test.com:3001/callback", "http://admin.test.com:8081/callback"]',
    'read write admin',
    NOW(),
    NOW(),
    NULL
) ON DUPLICATE KEY UPDATE
    client_secret = 'admin-portal-secret-k8s-test',
    name = '管理后台',
    redirect_uris = '["http://admin.test.com:3001/callback", "http://admin.test.com:8081/callback"]',
    scope = 'read write admin',
    updated_at = NOW();

-- 3. 创建 OAuth 客户端（测试客户端）
INSERT INTO oauth_clients (client_id, client_secret, name, redirect_uris, scope, created_at, updated_at, deleted_at)
VALUES (
    'test-client',
    'test-client-secret-k8s-test',
    '测试客户端',
    '["http://localhost:3000/callback", "http://localhost:8080/callback"]',
    'read write',
    NOW(),
    NOW(),
    NULL
) ON DUPLICATE KEY UPDATE
    client_secret = 'test-client-secret-k8s-test',
    name = '测试客户端',
    redirect_uris = '["http://localhost:3000/callback", "http://localhost:8080/callback"]',
    scope = 'read write',
    updated_at = NOW();

-- 4. 创建测试用户（如果不存在）
-- 密码: 123456 (bcrypt 加密)
INSERT IGNORE INTO users (username, password, phone, email, status, created_at, updated_at, deleted_at)
VALUES (
    'k8stest',
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    '13700000001',
    'k8s@test.com',
    1,
    NOW(),
    NOW(),
    NULL
);

-- 5. 创建管理员用户
INSERT IGNORE INTO users (username, password, phone, email, status, created_at, updated_at, deleted_at)
VALUES (
    'k8sadmin',
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    '13700000002',
    'k8sadmin@test.com',
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
