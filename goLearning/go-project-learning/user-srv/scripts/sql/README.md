# OAuth2.0 测试数据初始化指南

## 文件说明

| 文件 | 环境 | 说明 |
|------|------|------|
| `oauth_init_local.sql` | 本地开发环境 | 本地 MySQL 测试数据 |
| `oauth_init_k8s_test.sql` | K8s 测试环境 | K8s 集群测试数据 |

## 前置条件

1. **数据库已创建**
   - 本地: `userdb`
   - K8s: `userdb_test`

2. **表已创建**
   - 启动 user-srv 会自动创建表（GORM AutoMigrate）
   - 或手动执行建表 SQL

## 使用方法

### 本地环境

```bash
# 方式 1: 使用 mysql 命令
mysql -u root -p123456 userdb < user-srv/scripts/sql/oauth_init_local.sql

# 方式 2: 进入 mysql 后执行
mysql -u root -p123456
> USE userdb;
> SOURCE /path/to/oauth_init_local.sql;

# 方式 3: 使用 Docker
docker exec -i mysql mysql -uroot -p123456 userdb < user-srv/scripts/sql/oauth_init_local.sql
```

### K8s 测试环境

```bash
# 方式 1: 使用 kubectl exec
kubectl exec -i deployment/mysql -n go-project-test -- \
  mysql -uroot -p123456 userdb_test < user-srv/scripts/sql/oauth_init_k8s_test.sql

# 方式 2: 复制文件到 Pod 后执行
kubectl cp user-srv/scripts/sql/oauth_init_k8s_test.sql \
  go-project-test/$(kubectl get pods -n go-project-test -l app=mysql -o jsonpath='{.items[0].metadata.name}'):/tmp/
kubectl exec -it deployment/mysql -n go-project-test -- \
  mysql -uroot -p123456 userdb_test -e "SOURCE /tmp/oauth_init_k8s_test.sql;"
```

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

| Client ID | Client Secret | 名称 |
|-----------|---------------|------|
| mall-frontend | mall-frontend-secret-change-in-prod | 商城前端 |
| admin-portal | admin-portal-secret-change-in-prod | 管理后台 |
| mobile-app | mobile-app-secret-change-in-prod | 移动端APP |

### K8s 测试环境

| Client ID | Client Secret | 名称 |
|-----------|---------------|------|
| mall-frontend | mall-frontend-secret-k8s-test | 商城前端 |
| admin-portal | admin-portal-secret-k8s-test | 管理后台 |
| test-client | test-client-secret-k8s-test | 测试客户端 |

## 验证数据

```sql
-- 查看 OAuth 客户端
SELECT client_id, name, scope FROM oauth_clients WHERE deleted_at IS NULL;

-- 查看测试用户
SELECT id, username, phone, email FROM users WHERE deleted_at IS NULL;
```

## 测试 OAuth 流程

### 本地环境测试

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
```

### K8s 测试环境测试

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
```

## 常见问题

### 1. 表不存在

**错误**: `Table 'userdb.oauth_clients' doesn't exist`

**解决**: 启动 user-srv，GORM 会自动创建表
```bash
cd user-srv && go run cmd/main.go
```

### 2. 数据插入失败

**错误**: `Duplicate entry 'mall-frontend' for key 'oauth_clients.client_id'`

**说明**: 数据已存在，SQL 使用了 `ON DUPLICATE KEY UPDATE`，会自动更新
