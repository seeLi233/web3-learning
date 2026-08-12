# 测试脚本使用指南

## 文件说明

| 文件 | 环境 | 说明 |
|------|------|------|
| `test_oauth_local.sh` | 本地环境 | 测试 OAuth2.0 授权码模式 |
| `test_oauth_k8s.sh` | K8s 测试环境 | 测试 OAuth2.0 授权码模式 |

## 前置条件

### 本地环境

1. **启动 user-srv**
   ```bash
   cd user-srv && go run cmd/main.go
   ```

2. **启动 gateway-api**
   ```bash
   cd gateway-api && go run cmd/api/main.go
   ```

3. **初始化测试数据**
   ```bash
   mysql -u root -p123456 userdb < user-srv/scripts/sql/oauth_init_local.sql
   ```

### K8s 测试环境

1. **启动 port-forward**
   ```bash
   kubectl port-forward svc/gateway-api-svc 8081:8080 -n go-project-test
   ```

2. **初始化测试数据**
   ```bash
   kubectl exec -i deployment/mysql -n go-project-test -- \
     mysql -uroot -p123456 userdb_test < user-srv/scripts/sql/oauth_init_k8s_test.sql
   ```

## 使用方法

### 本地环境测试

```bash
# 添加执行权限
chmod +x scripts/test_oauth_local.sh

# 运行测试
./scripts/test_oauth_local.sh
```

### K8s 测试环境测试

```bash
# 添加执行权限
chmod +x scripts/test_oauth_k8s.sh

# 运行测试
./scripts/test_oauth_k8s.sh
```

## 测试流程

脚本会自动执行以下步骤：

1. **获取授权码**
   - 调用 `/api/v1/oauth/authorize`
   - 使用测试账号登录
   - 返回授权码

2. **换取 Token**
   - 调用 `/api/v1/oauth/token`
   - 使用授权码换取 access_token 和 refresh_token

3. **获取用户信息**
   - 调用 `/api/v1/oauth/userinfo`
   - 使用 access_token 获取用户信息

4. **刷新 Token**
   - 调用 `/api/v1/oauth/token`
   - 使用 refresh_token 获取新的 access_token

## 测试账号

### 本地环境

| 账号 | 密码 | 邮箱 |
|------|------|------|
| testuser | 123456 | test@example.com |
| admin | 123456 | admin@example.com |

### K8s 测试环境

| 账号 | 密码 | 邮箱 |
|------|------|------|
| k8stest | 123456 | k8s@test.com |
| k8sadmin | 123456 | k8sadmin@test.com |

## OAuth 客户端

### 本地环境

| Client ID | Client Secret |
|-----------|---------------|
| mall-frontend | mall-frontend-secret-change-in-prod |
| admin-portal | admin-portal-secret-change-in-prod |
| mobile-app | mobile-app-secret-change-in-prod |

### K8s 测试环境

| Client ID | Client Secret |
|-----------|---------------|
| mall-frontend | mall-frontend-secret-k8s-test |
| test-client | test-client-secret-k8s-test |

## 手动测试

如果想手动测试，可以参考以下命令：

### 本地环境

```bash
# 1. 获取授权码
curl -X POST http://localhost:8080/api/v1/oauth/authorize \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "response_type=code&client_id=mall-frontend&redirect_uri=http://localhost:3000/callback&scope=read&state=xyz123&account=testuser&password=123456"

# 2. 换取 Token
curl -X POST http://localhost:8080/api/v1/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=authorization_code&code=YOUR_CODE&redirect_uri=http://localhost:3000/callback&client_id=mall-frontend&client_secret=mall-frontend-secret-change-in-prod"

# 3. 获取用户信息
curl http://localhost:8080/api/v1/oauth/userinfo \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"

# 4. 刷新 Token
curl -X POST http://localhost:8080/api/v1/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=refresh_token&refresh_token=YOUR_REFRESH_TOKEN&client_id=mall-frontend&client_secret=mall-frontend-secret-change-in-prod"
```

### K8s 测试环境

```bash
# 1. 启动端口转发
kubectl port-forward svc/gateway-api-svc 8081:8080 -n go-project-test

# 2. 获取授权码
curl -X POST http://localhost:8081/api/v1/oauth/authorize \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "response_type=code&client_id=mall-frontend&redirect_uri=http://localhost:3000/callback&scope=read&state=xyz123&account=k8stest&password=123456"

# 3. 换取 Token
curl -X POST http://localhost:8081/api/v1/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=authorization_code&code=YOUR_CODE&redirect_uri=http://localhost:3000/callback&client_id=mall-frontend&client_secret=mall-frontend-secret-k8s-test"

# 4. 获取用户信息
curl http://localhost:8081/api/v1/oauth/userinfo \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN"

# 5. 刷新 Token
curl -X POST http://localhost:8081/api/v1/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=refresh_token&refresh_token=YOUR_REFRESH_TOKEN&client_id=mall-frontend&client_secret=mall-frontend-secret-k8s-test"
```

## 常见问题

### 1. 连接失败

**错误**: `❌ 无法连接到 http://localhost:8080`

**解决**: 确保服务已启动
```bash
# 本地环境
cd user-srv && go run cmd/main.go &
cd gateway-api && go run cmd/api/main.go &

# K8s 环境
kubectl port-forward svc/gateway-api-svc 8081:8080 -n go-project-test
```

### 2. 获取授权码失败

**错误**: `❌ 获取授权码失败`

**解决**: 检查测试数据是否存在
```sql
-- 本地环境
SELECT * FROM oauth_clients WHERE client_id = 'mall-frontend';
SELECT * FROM users WHERE username = 'testuser';

-- K8s 环境
kubectl exec -i deployment/mysql -n go-project-test -- \
  mysql -uroot -p123456 userdb_test -e "SELECT * FROM oauth_clients; SELECT * FROM users;"
```

### 3. 换取 Token 失败

**错误**: `❌ 获取 Token 失败`

**解决**: 
- 检查授权码是否过期（5分钟有效期）
- 检查 client_secret 是否正确
- 检查 redirect_uri 是否与获取授权码时一致
