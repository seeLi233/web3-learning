# Go 微服务电商项目结构

## 项目概览
- **架构**: 微服务架构，Go Workspace 管理多模块
- **网关**: Gin HTTP API (gateway-api)
- **服务通信**: gRPC + Protobuf
- **服务注册发现**: Consul
- **数据库**: MySQL + GORM
- **缓存**: Redis
- **消息队列**: RabbitMQ
- **监控**: Prometheus + Jaeger (OpenTelemetry)
- **日志**: ELK (Elasticsearch + Filebeat)
- **容器化**: Docker + Docker Compose + K8s

## 目录结构

```
go-project-learning/
├── go.work                          # Go Workspace 配置
├── docker-compose.yml               # 容器编排
│
├── gateway-api/                     # HTTP API 网关 (端口 8080)
│   ├── cmd/api/main.go              # 入口：初始化 Gin、注册路由、Consul
│   ├── api/v1/
│   │   ├── user.go                  # 用户路由定义
│   │   ├── address.go               # 地址路由定义
│   │   ├── member.go                # 会员路由定义
│   │   ├── coupon.go                # 优惠券路由定义
│   │   └── risk.go                  # 风控路由定义
│   ├── config/config.go             # 配置结构体
│   ├── configs/                     # 多环境配置 YAML
│   │   ├── config.dev.yaml
│   │   ├── config.docker.yaml
│   │   ├── config.prod.yaml
│   │   └── config.test.yaml
│   ├── global/global.go             # 全局变量 (gRPC客户端、Redis、配置)
│   ├── internal/
│   │   ├── handler/
│   │   │   ├── auth_handler.go      # 认证接口 (登录限流+失败次数限制)
│   │   │   ├── user_handler.go      # 用户接口 (注册限流)
│   │   │   ├── address_handler.go   # 地址接口
│   │   │   ├── member_handler.go    # 会员接口
│   │   │   ├── coupon_handler.go    # 优惠券接口
│   │   │   └── risk_handler.go      # 风控接口 (黑名单+配置管理)
│   │   ├── middleware/
│   │   │   ├── auth_jwt.go          # JWT 鉴权中间件
│   │   │   ├── cors.go              # 跨域中间件
│   │   │   ├── logger.go            # 请求日志中间件
│   │   │   └── ip_blacklists.go     # IP 黑名单中间件
│   │   └── rpc_client/
│   │       ├── user_rpc.go          # 用户 gRPC 客户端
│   │       ├── address_rpc.go       # 地址 gRPC 客户端
│   │       ├── member_rpc.go        # 会员 gRPC 客户端
│   │       ├── coupon_rpc.go        # 优惠券 gRPC 客户端
│   │       └── risk_rpc.go          # 风控 gRPC 客户端
│   ├── pkg/
│   │   ├── jwt/jwt.go               # JWT 解析、刷新
│   │   ├── resp/resp.go             # 统一响应封装
│   │   └── cookie/cookie.go         # Cookie 操作
│   └── README.md
│
├── user-srv/                        # 用户服务 (gRPC)
│   ├── api/pb/
│   │   ├── user.proto               # Proto 定义 (注册/登录/验证码)
│   │   ├── user.pb.go               # 生成的 Protobuf 代码
│   │   ├── user_grpc.pb.go          # 生成的 gRPC 代码
│   │   ├── address.proto            # 地址 Proto 定义
│   │   ├── member.proto             # 会员 Proto 定义
│   │   ├── coupon.proto             # 优惠券 Proto 定义
│   │   ├── risk.proto               # 风控 Proto 定义
│   │   └── ...                      # 其他生成文件
│   ├── cmd/main.go                  # 入口：初始化 DB/Redis/gRPC 服务
│   ├── configs/                     # 多环境配置
│   ├── global/global.go             # 全局变量
│   ├── internal/
│   │   ├── application/
│   │   │   ├── user_app.go          # 应用层：Register/PhoneLogin/EmailLogin/PwdLogin
│   │   │   ├── address_app.go       # 地址应用层
│   │   │   ├── member_app.go        # 会员应用层
│   │   │   ├── coupon_app.go        # 优惠券应用层
│   │   │   └── risk_config_app.go   # 风控应用层（黑名单+配置管理）
│   │   ├── domain/
│   │   │   ├── entity/
│   │   │   │   ├── user.go          # 用户实体
│   │   │   │   ├── address.go       # 地址实体
│   │   │   │   ├── member.go        # 会员实体
│   │   │   │   ├── coupon.go        # 优惠券实体
│   │   │   │   ├── ip_blacklist.go  # IP 黑名单实体
│   │   │   │   └── risk_config.go   # 风控配置实体
│   │   │   └── service/             # 领域服务
│   │   ├── repository/
│   │   │   ├── db/
│   │   │   │   ├── user_repo.go     # 用户数据访问
│   │   │   │   ├── address_repo.go  # 地址数据访问
│   │   │   │   ├── member_repo.go   # 会员数据访问
│   │   │   │   ├── coupon_repo.go   # 优惠券数据访问
│   │   │   │   ├── ip_blacklist_repo.go  # IP 黑名单数据访问
│   │   │   │   └── risk_config_repo.go   # 风控配置数据访问
│   │   │   └── cache/
│   │   │       ├── code_cache.go    # 验证码缓存
│   │   │       └── user_cache.go    # 用户缓存
│   │   └── server/
│   │       ├── user_server.go       # 用户 gRPC 服务实现
│   │       ├── address_server.go    # 地址 gRPC 服务实现
│   │       ├── member_server.go     # 会员 gRPC 服务实现
│   │       ├── coupon_server.go     # 优惠券 gRPC 服务实现
│   │       └── risk_server.go       # 风控 gRPC 服务实现
│   ├── pkg/
│   │   ├── bcrypt/bcrypt.go         # 密码加密
│   │   ├── jwt/jwt.go               # JWT 签发
│   │   └── sms/sender.go            # 短信发送
│   └── README.md
│
├── order-srv/                       # 订单服务 (gRPC, 待实现)
├── product-srv/                     # 商品服务 (gRPC, 待实现)
├── pay-srv/                         # 支付服务 (gRPC, 待实现)
├── promotion-srv/                   # 促销服务 (gRPC, 待实现)
│
├── common/                          # 公共库
│   └── pkg/
│       ├── config/config.go         # 配置加载
│       ├── consul/consul.go         # Consul 客户端
│       ├── database/db.go           # 数据库连接
│       ├── errorcode/               # 错误码定义
│       ├── logger/logger.go         # 日志
│       ├── mq/rabbitmq.go           # RabbitMQ
│       ├── otel/trace.go            # OpenTelemetry
│       └── redis/
│           ├── redis.go             # Redis 连接
│           └── ratelimit.go         # 滑动窗口限流（Lua 脚本）
│
├── api/order/v1/                    # 订单 Proto 生成代码
├── configs/                         # 全局配置
│   ├── filebeat/filebeat.yml
│   └── prometheus/prometheus.yml
├── deploy/k8s/                      # K8s 部署配置
└── consul-data/                     # Consul 数据目录
```

## 认证系统现状

### 用户模型 (User Entity)
```go
type User struct {
    gorm.Model
    Username string // 唯一
    Password string // bcrypt 加密
    Phone    string // 唯一
    Email    string // 唯一
    Status   int    // 1: active, 0: inactive
}
```

### 当前登录方式
1. **手机验证码登录** - SendCode → PhoneLogin
2. **邮箱密码登录** - EmailLogin
3. **通用账号密码登录** - PwdLogin (账号可以是用户名/手机/邮箱)

### Token 机制
- **Access Token**: 2小时过期，HS256 签名
- **Refresh Token**: 用于刷新 access_token
- **Claims**: UserID, Username, Phone

### 中间件链
CORS → Logger → IPBlacklist → JWTAuth (可选) → Handler

## 风控系统设计

### 数据库表结构

#### risk_configs（风控配置）
| 字段 | 类型 | 说明 |
|------|------|------|
| rule_key | varchar(50) unique | 规则键名 |
| rule_value | varchar(100) | 规则值 |
| description | varchar(255) | 描述 |
| status | boolean | 1:启用 0:禁用 |

预置数据：
- `register_ip_limit`: 3（同 IP 最多注册次数）
- `register_time_window`: 600（注册时间窗口，秒）
- `login_max_attempts`: 5（登录失败最大次数）
- `lock_duration`: 900（锁定时长，秒）

#### ip_blacklists（IP 黑名单）
| 字段 | 类型 | 说明 |
|------|------|------|
| ip | varchar(45) | IP 地址（支持 IPv6） |
| source | varchar(50) | 来源：manual/auto |
| reason | varchar(200) | 拉黑原因 |
| user_id | bigint | 操作人 |
| status | boolean | 1:生效 0:失效 |
| deadline | datetime | 过期时间（NULL=永久） |

### 核心业务规则
- **滑动窗口限流**: 使用 Redis ZSET + Lua 脚本实现原子操作
- **IP 黑名单**: 二级缓存（sync.Map + Redis），每次请求都检查 Redis
- **登录失败限制**: 使用 Redis INCR + EXPIRE，从 risk_configs 读取参数
- **IPv4/IPv6 兼容**: 添加黑名单时同时添加两种格式
- **配置热更新**: 修改配置后调用刷新接口同步到 Redis

### 限流算法实现

```lua
-- 滑动窗口限流 Lua 脚本
local key = KEYS[1]
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

-- 删除窗口外的记录
redis.call('ZREMRANGEBYSCORE', key, 0, now - window * 1000)

-- 统计当前窗口内的请求数
local count = redis.call('ZCARD', key)

-- 判断是否超限
if count < limit then
    redis.call('ZADD', key, now, now .. math.random())
    redis.call('EXPIRE', key, window)
    return 1
else
    return 0
end
```

## 会员系统设计

### 数据库表结构

#### member_levels（会员等级配置）
| 字段 | 类型 | 说明 |
|------|------|------|
| level_name | varchar(20) | 等级名称 |
| level_value | int (unique) | 等级值：0/1/2/3 |
| min_growth | int | 最低成长值门槛 |
| max_growth | int | 最高成长值（-1=无上限） |
| discount | decimal(3,2) | 折扣率 |
| status | tinyint | 1:启用 0:禁用 |

预置数据：普通会员(0,0-999) → 银卡(1,1000-4999,98折) → 金卡(2,5000-19999,95折) → 钻石(3,20000+,9折)

#### member_infos（用户会员信息）
| 字段 | 类型 | 说明 |
|------|------|------|
| user_id | bigint (unique) | 用户ID |
| level_id | bigint | 当前等级ID |
| growth_value | int | 当前成长值（可增可减） |
| total_growth | int | 累计成长值（只增不减） |
| level_up_time | datetime | 最近升级时间 |

#### member_growth_logs（成长值变动日志）
| 字段 | 类型 | 说明 |
|------|------|------|
| user_id | bigint (index) | 用户ID |
| change_value | int | 变动值（正=增加，负=减少） |
| source_type | varchar(30) | 来源：order/review/signin/admin/refund |
| source_id | varchar(64) | 来源ID（如订单号） |
| description | varchar(200) | 变动说明 |

### 核心业务规则
- **成长值来源**: 订单完成(1元=1)、商品评价(+10)、每日签到(+5)
- **自动升级**: 增加成长值后立即检查，>= 下一等级门槛即升级
- **自动降级**: 扣减成长值后检查，< 当前等级门槛即降级
- **懒初始化**: 查询时发现无记录自动创建（兼容老用户）
- **初始化 SQL**: `user-srv/scripts/sql/member_levels_init.sql`（K8s 通过 ConfigMap 自动执行）

## 技术栈版本
- Go 1.26.4
- Gin (HTTP 框架)
- gRPC + Protobuf
- GORM (ORM)
- golang-jwt/jwt/v5
- Consul (服务发现)
- Redis, MySQL, RabbitMQ

---

## 当前项目状态（2026-06-11 更新）

### ✅ 已完成功能

#### 1. 用户服务 (user-srv)
- **用户注册**: CreateUser - 支持用户名、手机号、邮箱唯一性检查
- **多种登录方式**:
  - 手机验证码登录: SendCode + PhoneLogin
  - 邮箱密码登录: EmailLogin
  - 通用账号密码登录: PwdLogin（支持用户名/手机/邮箱）
- **用户信息修改**: UpdateUser - 支持更新用户名、手机号、邮箱，带唯一性校验
- **JWT 认证**: Access Token（2小时）+ Refresh Token
- **用户查询**: GetUser（按ID）、ListUser（分页）

#### 2. 收货地址服务 (Address CRUD)
- **创建地址**: CreateAddress - 支持设置默认地址（自动取消其他默认）
- **获取地址**: GetAddress（按ID）
- **获取地址列表**: ListAddress（从 JWT 获取用户ID）
- **更新地址**: UpdateAddress - 支持更新所有字段和默认地址状态
- **删除地址**: DeleteAddress（软删除）

#### 3. 会员等级服务 (Member Level) — 2026-06-09 完成
- **会员等级体系**: 普通会员(0) / 银卡(1) / 金卡(2) / 钻石(3)
- **成长值管理**: 原子操作增加/扣减成长值（gorm.Expr 并发安全）
- **自动升级**: 成长值达到阈值自动升级，支持跨级升级
- **自动降级**: 扣减成长值后自动检查降级
- **会员权益**: 根据等级返回累积式权益列表
- **成长值日志**: 分页查询变动记录，支持多来源（order/review/signin/admin/refund）
- **懒初始化**: 查询会员信息时自动初始化老用户（兼容功能上线前的用户）
- **用户注册联动**: 注册时自动初始化默认会员（普通会员）
- **K8s 数据初始化**: ConfigMap + /docker-entrypoint-initdb.d 自动执行预置 SQL

#### 4. 优惠券服务 (Coupon) — 2026-06-10 完成
- **优惠券模板管理**: 支持满减券、折扣券、无门槛券三种类型
- **优惠券领取**: 防重复领取、库存校验、每人限领数量控制
- **优惠券使用**: 订单支付时使用，状态流转（未使用→已使用）
- **有效期管理**: 支持有效期开始/结束时间控制
- **可用优惠券筛选**: 根据订单金额自动筛选可用优惠券
- **库存管理**: 原子递增已领取数量，支持无限制发放（total_count=-1）

#### 5. 风控服务 (Risk Control) — 2026-06-11 完成
- **IP 黑名单管理**: 支持添加、删除、查询黑名单
- **滑动窗口限流**: Redis + Lua 脚本实现原子限流
- **登录限流**: 同一 IP 每分钟最多 10 次登录请求
- **登录失败限制**: 连续失败 5 次锁定 15 分钟（从配置读取）
- **注册限流**: 同一 IP 每分钟最多 3 次注册
- **风控配置管理**: 支持动态配置限流参数
- **二级缓存**: 本地缓存（sync.Map）+ Redis，实时检查
- **IPv4/IPv6 兼容**: 添加黑名单时同时支持两种格式

#### 6. 网关服务 (gateway-api)
- **HTTP API 网关**: Gin 框架，端口 8080
- **JWT 认证中间件**: 自动解析 Token，提取用户信息
- **统一响应封装**: resp.OK() / resp.Error()
- **gRPC 客户端**: 连接 user-srv 服务（用户/地址/会员/优惠券/风控）
- **黑名单中间件**: 自动拦截黑名单 IP
- **限流中间件**: 登录、注册接口限流

### 📁 关键文件位置

| 功能 | 文件路径 |
|------|----------|
| 用户 Proto 定义 | `user-srv/api/pb/user.proto` |
| 地址 Proto 定义 | `user-srv/api/pb/address.proto` |
| 会员 Proto 定义 | `user-srv/api/pb/member.proto` |
| 优惠券 Proto 定义 | `user-srv/api/pb/coupon.proto` |
| 用户业务逻辑 | `user-srv/internal/application/user_app.go` |
| 地址业务逻辑 | `user-srv/internal/application/address_app.go` |
| 会员业务逻辑 | `user-srv/internal/application/member_app.go` |
| 优惠券业务逻辑 | `user-srv/internal/application/coupon_app.go` |
| 用户数据访问 | `user-srv/internal/repository/db/user_repo.go` |
| 地址数据访问 | `user-srv/internal/repository/db/address_repo.go` |
| 会员数据访问 | `user-srv/internal/repository/db/member_repo.go` |
| 优惠券数据访问 | `user-srv/internal/repository/db/coupon_repo.go` |
| 用户 HTTP 处理器 | `gateway-api/internal/handler/user_handler.go` |
| 地址 HTTP 处理器 | `gateway-api/internal/handler/address_handler.go` |
| 会员 HTTP 处理器 | `gateway-api/internal/handler/member_handler.go` |
| 优惠券 HTTP 处理器 | `gateway-api/internal/handler/coupon_handler.go` |
| 用户路由 | `gateway-api/api/v1/user.go` |
| 地址路由 | `gateway-api/api/v1/address.go` |
| 会员路由 | `gateway-api/api/v1/member.go` |
| 优惠券路由 | `gateway-api/api/v1/coupon.go` |
| 风控 Proto 定义 | `user-srv/api/pb/risk.proto` |
| IP 黑名单实体 | `user-srv/internal/domain/entity/ip_blacklist.go` |
| 风控配置实体 | `user-srv/internal/domain/entity/risk_config.go` |
| IP 黑名单数据访问 | `user-srv/internal/repository/db/ip_blacklist_repo.go` |
| 风控配置数据访问 | `user-srv/internal/repository/db/risk_config_repo.go` |
| 风控业务逻辑 | `user-srv/internal/application/risk_config_app.go` |
| 风控 gRPC 实现 | `user-srv/internal/server/risk_server.go` |
| 滑动窗口限流 | `common/pkg/redis/ratelimit.go` |
| IP 黑名单中间件 | `gateway-api/internal/middleware/ip_blacklists.go` |
| 风控 HTTP 处理器 | `gateway-api/internal/handler/risk_handler.go` |
| 风控路由 | `gateway-api/api/v1/risk.go` |
| 风控 RPC 客户端 | `gateway-api/internal/rpc_client/risk_rpc.go` |
| 会员等级初始化 SQL | `user-srv/scripts/sql/member_levels_init.sql` |
| 风控配置初始化 SQL | `user-srv/scripts/sql/risk_config_init.sql` |
| IP 黑名单初始化 SQL | `user-srv/scripts/sql/ip_blacklists_init.sql` |
| K8s MySQL+初始化配置 | `deploy/k8s/mysql-test.yaml` (含 ConfigMap) |
| K8s 风控 SQL Job | `deploy/k8s/risk-sql-job.yaml` |
| K8s 部署配置 | `deploy/k8s/` 目录 |

### 🔌 API 接口列表

#### 用户相关接口
| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/v1/user/register` | 用户注册 | ❌ |
| POST | `/api/v1/user/login/password` | 密码登录 | ❌ |
| POST | `/api/v1/user/login/phone` | 手机验证码登录 | ❌ |
| POST | `/api/v1/user/login/email` | 邮箱密码登录 | ❌ |
| POST | `/api/v1/user/send-code` | 发送验证码 | ❌ |
| GET | `/api/v1/user/:id` | 获取用户信息 | ✅ |
| GET | `/api/v1/user/list` | 获取用户列表 | ✅ |
| PUT | `/api/v1/user/profile` | 修改用户信息 | ✅ |

#### 地址相关接口
| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/v1/address` | 创建地址 | ✅ |
| GET | `/api/v1/address/:id` | 获取地址 | ✅ |
| GET | `/api/v1/address/list` | 获取地址列表 | ✅ |
| PUT | `/api/v1/address/:id` | 更新地址 | ✅ |
| DELETE | `/api/v1/address/:id` | 删除地址 | ✅ |

#### 会员相关接口
| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/v1/member/info` | 获取会员信息（懒初始化） | ✅ |
| POST | `/api/v1/member/growth/add` | 增加成长值（自动升级） | ✅ |
| POST | `/api/v1/member/growth/deduct` | 扣减成长值（自动降级） | ✅ |
| GET | `/api/v1/member/growth/logs` | 成长值变动日志（分页） | ✅ |
| GET | `/api/v1/member/benefits` | 获取会员权益列表 | ✅ |
| GET | `/api/v1/member/levels` | 获取所有等级配置 | ✅ |

#### 优惠券相关接口
| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| GET | `/api/v1/coupon/list` | 获取可领取的优惠券列表 | ✅ |
| POST | `/api/v1/coupon/claim/:id` | 领取优惠券 | ✅ |
| GET | `/api/v1/coupon/my` | 获取我的优惠券列表 | ✅ |
| GET | `/api/v1/coupon/available` | 获取可用优惠券（下单时） | ✅ |
| POST | `/api/v1/coupon/use/:id` | 使用优惠券 | ✅ |
| GET | `/api/v1/coupon/detail/:id` | 获取优惠券详情 | ✅ |

#### 风控相关接口
| 方法 | 路径 | 说明 | 认证 |
|------|------|------|------|
| POST | `/api/v1/risk/blacklist` | 添加黑名单 | ✅ |
| DELETE | `/api/v1/risk/blacklist/:id` | 移除黑名单 | ✅ |
| GET | `/api/v1/risk/blacklist` | 获取黑名单列表（分页） | ✅ |
| GET | `/api/v1/risk/config` | 获取风控配置列表 | ✅ |
| PUT | `/api/v1/risk/config` | 更新风控配置 | ✅ |
| POST | `/api/v1/risk/config/refresh` | 刷新配置到 Redis | ✅ |

### ⚠️ 待解决问题

#### 1. Docker 镜像构建失败
**问题**: Go 1.26.4 版本太新，Docker Hub 上的 `golang:1.26-alpine` 镜像可能不存在
**错误信息**: `429 Too Many Requests` 或镜像不存在
**解决方案**:
```dockerfile
# 修改 deploy/Dockerfile.user-srv 和 deploy/Dockerfile.gateway-api
# 将
FROM golang:1.26-alpine AS builder
# 改为
FROM golang:1.23-alpine AS builder
```

#### 2. 测试环境部署
**状态**: K8s 配置文件已准备好，待修复 Docker 镜像问题后部署
**部署命令**:
```bash
# 构建镜像
docker build -f deploy/Dockerfile.user-srv -t user-srv:latest .
docker build -f deploy/Dockerfile.gateway-api -t gateway-api:latest .

# 部署到 K8s
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/mysql-test.yaml    # 含会员等级初始化 ConfigMap
kubectl apply -f deploy/k8s/redis-test.yaml
kubectl apply -f deploy/k8s/consul-test.yaml
kubectl apply -f deploy/k8s/user-srv-test.yaml
kubectl apply -f deploy/k8s/gateway-api-test.yaml

# 执行风控 SQL 初始化（K8s Job）
kubectl apply -f deploy/k8s/risk-sql-job.yaml
kubectl logs -l job-name=risk-sql-init -n go-project-test  # 查看执行日志
```

### 🚀 本地开发环境

#### 启动基础设施
```bash
cd D:\web3-learning\goLearning\go-project-learning
docker-compose up -d mysql redis consul
```

#### 启动服务
```bash
# 启动 user-srv
cd user-srv
go run cmd/main.go

# 启动 gateway-api（新终端）
cd gateway-api
go run cmd/api/main.go
```

#### 测试 API
```bash
# 注册用户
curl -X POST http://localhost:8080/api/v1/user/register \
  -H "Content-Type: application/json" \
  -d '{"name":"testuser","password":"123456","phone":"13800138000","email":"test@example.com"}'

# 登录获取 Token
curl -X POST http://localhost:8080/api/v1/user/login/password \
  -H "Content-Type: application/json" \
  -d '{"account":"testuser","password":"123456"}'

# 修改用户信息（使用获取到的 Token）
curl -X PUT http://localhost:8080/api/v1/user/profile \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your_token>" \
  -d '{"username":"new_name","phone":"13900139000","email":"new@example.com"}'
```

### 📚 下次学习建议

1. **修复部署问题**: 修改 Dockerfile 中的 Go 版本，完成测试环境部署
2. **继续开发服务**:
   - 商品服务 (product-srv) — 电商核心
   - 订单服务 (order-srv) — 电商核心
   - 支付服务 (pay-srv)
   - 促销服务 (promotion-srv)
3. **优化现有功能**:
   - 添加分布式链路追踪（Jaeger）
   - 添加限流和熔断（Sentinel）
   - 配置 CI/CD 流水线
   - 配置监控和告警（Prometheus + Grafana）
4. **技术深化**:
   - 学习分布式事务（Saga/TCC）
   - 学习消息队列（RabbitMQ）实战
   - 学习 Elasticsearch 搜索

---

## 🚀 部署指南（Kind K8s 环境）

### 部署脚本

项目根目录下有 `deploy.sh` 脚本，可以一键部署：

```bash
# 执行部署脚本
./deploy.sh
```

### 手动部署步骤

```bash
# 1. 构建镜像
docker build --no-cache -f deploy/Dockerfile.gateway-api -t gateway-api:latest .
docker build --no-cache -f deploy/Dockerfile.user-srv -t user-srv:latest .

# 2. 加载镜像到 Kind（关键步骤！）
kind load docker-image gateway-api:latest --name go-project
kind load docker-image user-srv:latest --name go-project

# 3. 重启 Deployment
kubectl rollout restart deployment/gateway-api -n go-project-test
kubectl rollout restart deployment/user-srv -n go-project-test

# 4. 等待部署完成
kubectl rollout status deployment/gateway-api -n go-project-test
kubectl rollout status deployment/user-srv -n go-project-test

# 5. 验证部署
kubectl logs -l app=gateway-api -n go-project-test --tail=30 | grep "address"
```

### 部署问题排查

#### 问题：代码更新后 K8s 没有生效

**现象**：
- 本地代码已经修改并构建了新的 Docker 镜像
- K8s 中的 Pod 已经重启
- 但是新功能没有生效，日志中没有新的路由

**原因**：
Docker 和 Kind K8s 使用不同的容器运行时，镜像存储不共享：

| 组件 | 运行时 | 镜像存储 |
|------|--------|----------|
| Docker Desktop | Docker (containerd) | Docker 镜像仓库 |
| Kind K8s | containerd | 独立的 containerd 镜像仓库 |

**解决方案**：
使用 `kind load docker-image` 将 Docker 镜像加载到 Kind：

```bash
# 构建镜像
docker build -f deploy/Dockerfile.gateway-api -t gateway-api:latest .

# 加载到 Kind（关键步骤！）
kind load docker-image gateway-api:latest --name go-project

# 重启 Deployment
kubectl rollout restart deployment/gateway-api -n go-project-test
```

**验证方法**：
```bash
# 检查 Kind 中的镜像创建时间
docker exec go-project-control-plane crictl images | grep gateway-api

# 检查 Pod 日志中的路由注册
kubectl logs -l app=gateway-api -n go-project-test | grep "address"
```

#### 问题：kubectl port-forward 访问失败

**现象**：
- 使用 `curl http://172.20.0.4:30081` 无法访问
- 返回空响应或连接错误

**原因**：
Kind 集群运行在 Docker 内部，节点 IP 从 Windows 主机无法直接访问

**解决方案**：
使用 `kubectl port-forward` 端口转发：

```bash
# 端口转发
kubectl port-forward svc/gateway-api-svc 8080:8080 -n go-project-test &

# 使用 localhost 访问
curl http://127.0.0.1:8080/api/v1/user/register
```

#### 问题：Git Bash 路径转换错误

**现象**：
```
ls: cannot access 'C:/Users/41482/AppData/Local/Temp/': No such file or directory
```

**原因**：
Git Bash (MSYS) 会自动将 `/tmp/` 等路径转换为 Windows 路径

**解决方案**：
使用 `MSYS_NO_PATHCONV=1` 禁用路径转换：

```bash
MSYS_NO_PATHCONV=1 docker exec go-project-control-plane ls /tmp/
```

---

## 📅 学习计划

### 2026-06-09 完成情况

#### ✅ 任务1：用户中心 - 会员等级功能 — 已完成

**数据库表**：
- `member_levels` — 会员等级配置（普通/银卡/金卡/钻石）
- `member_infos` — 用户会员信息（成长值、等级、升级时间）
- `member_growth_logs` — 成长值变动日志

**核心知识点收获**：
1. GORM `Updates + map` 原子操作（并发安全）
2. GORM `Save` vs `Updates` 的区别（Save 更新所有字段，Updates 只更新指定字段）
3. Proto `decimal.Decimal` → `float64` 类型转换（InexactFloat64）
4. gRPC `UnimplementedXxxServer` 用值类型嵌入，不用指针
5. 懒初始化模式（查不到记录时自动创建默认数据）
6. K8s ConfigMap + /docker-entrypoint-initdb.d 初始化数据库
7. 分层架构：app 层负责业务兜底，repo 层只管数据库操作

**踩坑记录**：
- `Save()` 更新嵌套关联结构体导致 level_id 更新失败 → 改用 `Updates(map)`
- `Find()` 查不到记录不报错 → 改用 `First()`
- `*UnimplementedMemberServiceServer` 指针嵌入导致 nil panic → 改为值类型
- 老用户没有 member_info 记录 → 懒初始化兜底

---

### 2026-06-10 完成情况

#### ✅ 任务2：用户中心 - 优惠券绑定功能 — 已完成

**数据库表**：
- `coupon_templates` — 优惠券模板（满减券/折扣券/无门槛券）
- `user_coupons` — 用户优惠券（状态：未使用/已使用/已过期）

**核心知识点收获**：
1. 优惠券模板设计：支持多种类型（满减/折扣/无门槛）
2. 库存管理：原子递增 `claimed_count`，使用 `gorm.Expr` 并发安全
3. 防重复领取：`RowsAffected == 0` 校验库存 + 用户领取上限检查
4. 有效期校验：`StartTime` / `EndTime` 时间范围控制
5. 状态流转：未使用(1) → 已使用(2)，使用时记录 `used_at` 和 `order_id`
6. 关联查询：`Preload("Template")` 加载优惠券模板信息
7. 可用优惠券筛选：JOIN 查询 + 多条件过滤（状态、有效期、最低消费）

**文件清单**：
- Proto 定义：`user-srv/api/pb/coupon.proto`
- 实体定义：`user-srv/internal/domain/entity/coupon.go`
- 业务逻辑：`user-srv/internal/application/coupon_app.go`
- 数据访问：`user-srv/internal/repository/db/coupon_repo.go`
- gRPC 实现：`user-srv/internal/server/coupon_server.go`
- RPC 客户端：`gateway-api/internal/rpc_client/coupon_rpc.go`
- HTTP 处理：`gateway-api/internal/handler/coupon_handler.go`
- 路由定义：`gateway-api/api/v1/coupon.go`

**新增 API 接口**：
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/coupon/list` | 获取可领取的优惠券列表 |
| POST | `/api/v1/coupon/claim/:id` | 领取优惠券 |
| GET | `/api/v1/coupon/my` | 获取我的优惠券列表 |
| GET | `/api/v1/coupon/available` | 获取可用优惠券（下单时） |
| POST | `/api/v1/coupon/use/:id` | 使用优惠券 |
| GET | `/api/v1/coupon/detail/:id` | 获取优惠券详情 |

---

### 2026-06-11 完成情况

#### ✅ 任务3：用户中心 - 安全加固（黑名单 + 风控）— 已完成

**数据库表**：
- `risk_configs` — 风控配置（限流参数、锁定时长等）
- `ip_blacklists` — IP 黑名单（支持 IPv6、过期时间）

**核心知识点收获**：
1. GORM 实体设计规范（gorm.Model、tag 约束、索引设计）
2. key-value 配置模式（灵活存储不同类型的规则）
3. 滑动窗口限流算法（Redis ZSET + Lua 原子操作）
4. 二级缓存策略（sync.Map + Redis，实时检查）
5. 微服务架构（Gateway 不直接访问数据库，通过 Redis 同步）
6. 登录安全（IP 限流 + 账号失败次数限制 + 账号锁定）
7. IPv4/IPv6 兼容处理
8. K8s Job 增量 SQL 部署

**踩坑记录**：
- `redis.Set` 返回的错误没有检查 → 添加错误处理
- IPv6 地址 `::1` 和 IPv4 `127.0.0.1` 不匹配 → 同时添加两种格式
- 本地缓存 "allowed" 导致黑名单不生效 → 每次都检查 Redis
- Redis 端口配置不一致（6379 vs 6380）→ 统一配置
- risk_config_app.go 表名不一致 → 统一使用 `risk_configs`

**文件清单**：
- Proto 定义：`user-srv/api/pb/risk.proto`
- 实体定义：`user-srv/internal/domain/entity/ip_blacklist.go`、`risk_config.go`
- 业务逻辑：`user-srv/internal/application/risk_config_app.go`
- 数据访问：`user-srv/internal/repository/db/ip_blacklist_repo.go`、`risk_config_repo.go`
- gRPC 实现：`user-srv/internal/server/risk_server.go`
- 限流组件：`common/pkg/redis/ratelimit.go`
- 中间件：`gateway-api/internal/middleware/ip_blacklists.go`
- HTTP 处理：`gateway-api/internal/handler/risk_handler.go`
- 路由定义：`gateway-api/api/v1/risk.go`
- RPC 客户端：`gateway-api/internal/rpc_client/risk_rpc.go`
- SQL 脚本：`user-srv/scripts/sql/risk_config_init.sql`、`ip_blacklists_init.sql`

**新增 API 接口**：
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/risk/blacklist` | 添加黑名单 |
| DELETE | `/api/v1/risk/blacklist/:id` | 移除黑名单 |
| GET | `/api/v1/risk/blacklist` | 获取黑名单列表 |
| GET | `/api/v1/risk/config` | 获取风控配置列表 |
| PUT | `/api/v1/risk/config` | 更新风控配置 |
| POST | `/api/v1/risk/config/refresh` | 刷新配置到 Redis |

---

### 学习目标

- [x] 完成会员等级功能开发
- [x] 完成优惠券绑定功能开发
- [x] 完成风控功能开发（黑名单+限流）
- [x] 代码通过 code review
- [x] 部署到测试环境并验证
- [x] 更新 project_structure.md 文档

---

### 数据库增量更新指南

#### 背景
K8s 的 `/docker-entrypoint-initdb.d` 只在 MySQL **首次启动**时执行。如果需要新增表或修改表结构，不能简单更新 ConfigMap，需要手动增量更新。

#### 方式一：一行命令执行 SQL 文件（推荐）

```bash
# 获取 MySQL Pod 名称
POD_NAME=$(kubectl get pods -l app=mysql -n go-project-test -o jsonpath='{.items[0].metadata.name}')

# 执行增量 SQL 文件
kubectl exec -i $POD_NAME -n go-project-test -- mysql -uroot -p123456 userdb_test < user-srv/scripts/sql/coupon_tables.sql
```

#### 方式二：手动进入 MySQL 执行

```bash
# 进入 Pod
kubectl exec -it $POD_NAME -n go-project-test -- bash

# 连接数据库
mysql -uroot -p123456 userdb_test

# 执行 SQL
CREATE TABLE IF NOT EXISTS `user_coupons` (...);
```

#### 方式三：使用 K8s Job（正式环境推荐）

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: mysql-migration-xxx
  namespace: go-project-test
spec:
  template:
    spec:
      containers:
        - name: mysql-client
          image: mysql:8.0
          command: ["/bin/sh", "-c"]
          args:
            - mysql -h mysql-svc -uroot -p123456 userdb_test < /sql/migration.sql;
          volumeMounts:
            - name: sql-volume
              mountPath: /sql
      volumes:
        - name: sql-volume
          configMap:
            name: migration-sql
      restartPolicy: Never
  backoffLimit: 3
```

#### 注意事项
1. 增量 SQL 使用 `CREATE TABLE IF NOT EXISTS` 防止重复创建
2. 使用 `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`（MySQL 8.0+）安全添加字段
3. 生产环境建议使用 K8s Job，便于审计和重试
4. 本地开发直接用方式一最方便

---

### 2026-06-11 学习计划

#### ✅ 任务3：用户中心 - 安全加固（黑名单 + 风控）— 已完成

**功能需求**：
1. **IP 黑名单**
   - IP 黑名单表设计 ✓
   - 黑名单中间件（拦截请求）✓
   - 黑名单管理接口（增删查）✓

2. **用户风控**
   - 异常注册检测（同 IP 短时间大量注册）✓
   - 频繁登录限流（滑动窗口算法）✓
   - 登录失败次数限制（连续失败锁定）✓

3. **限流算法**
   - 滑动窗口计数器 ✓
   - Redis + Lua 脚本实现原子限流 ✓

**技术要点**：
1. Redis 限流实现（ZSET + Lua 脚本）✓
2. 滑动窗口算法原理与实现 ✓
3. Gin 中间件链设计 ✓
4. IP 获取与伪造防护（c.ClientIP()）✓
5. 黑名单缓存策略（本地缓存 + Redis）✓
6. IPv4/IPv6 兼容处理 ✓

**核心知识点收获**：
1. GORM 实体设计规范（gorm.Model、tag 约束、索引设计）
2. key-value 配置模式（灵活存储不同类型的规则）
3. 滑动窗口限流算法（Redis ZSET + Lua 原子操作）
4. 二级缓存策略（sync.Map + Redis，实时检查）
5. 微服务架构（Gateway 不直接访问数据库，通过 Redis 同步）
6. 登录安全（IP 限流 + 账号失败次数限制 + 账号锁定）

**踩坑记录**：
- `redis.Set` 返回的错误没有检查 → 添加错误处理
- IPv6 地址 `::1` 和 IPv4 `127.0.0.1` 不匹配 → 同时添加两种格式
- 本地缓存 "allowed" 导致黑名单不生效 → 每次都检查 Redis
- Redis 端口配置不一致（6379 vs 6380）→ 统一配置

---

## 🔧 常见问题解决

### 1. K8s ConfigMap 更新后 Pod 未生效

**问题描述**：
更新了 ConfigMap，但 Pod 中的配置没有变化。

**原因**：
ConfigMap 更新后，已经运行的 Pod 不会自动重新加载配置。

**解决方案**：
```bash
# 方式一：重启 Deployment（推荐）
kubectl rollout restart deployment/<deployment-name> -n <namespace>

# 方式二：删除 Pod（会自动重建）
kubectl delete pod -l app=<app-label> -n <namespace>

# 方式三：查看滚动更新状态
kubectl rollout status deployment/<deployment-name> -n <namespace>
```

**验证方法**：
```bash
# 查看 Pod 日志
kubectl logs -l app=<app-label> -n <namespace> --tail=50

# 查看 ConfigMap 内容
kubectl get configmap <configmap-name> -n <namespace> -o yaml
```

---

### 2. K8s 中 Redis 连接失败（dial tcp :0）

**问题描述**：
```
FATAL   redis/redis.go:41       Redis 连接失败  {"error": "dial tcp :0: connect: connection refused"}
```

**原因**：
Redis 的 host 和 port 为空，说明配置没有正确加载。

**排查步骤**：
```bash
# 1. 检查 ConfigMap 是否包含 redis 配置
kubectl get configmap <configmap-name> -n <namespace> -o yaml

# 2. 检查 Pod 中的配置文件
kubectl exec -it <pod-name> -n <namespace> -- cat /app/configs/config.test.yaml

# 3. 检查环境变量
kubectl exec -it <pod-name> -n <namespace> -- env | grep -i redis
```

**解决方案**：
```bash
# 更新 ConfigMap
kubectl create configmap <configmap-name> -n <namespace> \
  --from-file=config.test.yaml=<local-config-path> \
  --dry-run=client -o yaml | kubectl apply -f -

# 重启 Deployment
kubectl rollout restart deployment/<deployment-name> -n <namespace>
```

---

### 3. IPv6 地址导致黑名单不生效

**问题描述**：
添加了黑名单，但请求没有被拦截。

**原因**：
- 客户端使用 IPv6 地址（如 `::1`）
- 黑名单中只有 IPv4 地址（如 `127.0.0.1`）
- IP 地址不匹配

**解决方案**：
在添加黑名单时，同时添加 IPv4 和 IPv6 的 key：
```go
keys := []string{"ip_blacklist:" + ip}
if ip == "127.0.0.1" {
    keys = append(keys, "ip_blacklist:::1")
} else if ip == "::1" {
    keys = append(keys, "ip_blacklist:127.0.0.1")
}
```

**验证方法**：
```bash
# 检查 Redis 中的 key
docker exec <redis-container> redis-cli keys "ip_blacklist:*"
```

---

### 4. 本地缓存导致黑名单不实时生效

**问题描述**：
更新了 Redis 中的黑名单，但请求仍然通过。

**原因**：
中间件先检查本地缓存（sync.Map），如果缓存中有 "allowed"，直接放行，不检查 Redis。

**解决方案**：
修改中间件逻辑，每次都检查 Redis：
```go
func IPBlacklist() gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()

        // 1. 检查本地缓存（仅用于快速拒绝）
        if value, ok := sm.Load(ip); ok {
            if value.(string) == "blacklisted" {
                resp.Error(c, 403, "IP 已被封禁")
                c.Abort()
                return
            }
        }

        // 2. 检查 Redis（确保实时性）
        redisKey := "ip_blacklist:" + ip
        exists, err := redis.Exists(c, redisKey)
        if err == nil && exists {
            sm.Store(ip, "blacklisted")
            resp.Error(c, 403, "IP 已被封禁")
            c.Abort()
            return
        }

        // 3. 未命中黑名单
        sm.Store(ip, "allowed")
        c.Next()
    }
}
```

---

### 5. Redis 端口配置不一致

**问题描述**：
user-srv 和 gateway-api 连接的 Redis 不是同一个。

**原因**：
两个服务的配置文件中 Redis 端口不一致：
- user-srv: `port: 6380`
- gateway-api: `port: 6379`

**解决方案**：
统一配置，确保两个服务连接同一个 Redis：
```yaml
# config.dev.yaml (两个服务都需要)
redis:
  host: 127.0.0.1
  port: 6380  # 统一端口
  password: ""
  db: 0
  pool_size: 10
```

**验证方法**：
```bash
# 检查两个服务的 Redis 连接
docker exec <redis-container> redis-cli client list
```

---

### 6. K8s Job 执行 SQL 失败

**问题描述**：
Job 状态显示 `Error` 或 `CrashLoopBackOff`。

**排查步骤**：
```bash
# 1. 查看 Job 状态
kubectl get jobs -n <namespace>

# 2. 查看 Pod 日志
kubectl logs -l job-name=<job-name> -n <namespace>

# 3. 查看 Pod 事件
kubectl describe pod -l job-name=<job-name> -n <namespace>
```

**常见原因**：
- MySQL 连接信息错误（host、port、密码）
- SQL 语法错误
- 表已存在（使用 `IF NOT EXISTS` 防止）

**解决方案**：
```bash
# 手动测试 SQL
kubectl exec -it <mysql-pod> -n <namespace> -- mysql -uroot -p123456 userdb_test -e "SELECT 1;"

# 查看 Job 详细信息
kubectl describe job <job-name> -n <namespace>
```

---

### 7. 限流参数不生效

**问题描述**：
修改了数据库中的风控配置，但限流行为没有变化。

**原因**：
配置只更新了数据库，没有同步到 Redis。

**解决方案**：
```bash
# 调用刷新接口
curl -X POST http://<gateway-url>/api/v1/risk/config/refresh \
  -H "Authorization: Bearer <token>"

# 或者重启 user-srv（会自动加载配置）
kubectl rollout restart deployment/user-srv -n <namespace>
```

**验证方法**：
```bash
# 检查 Redis 中的配置
docker exec <redis-container> redis-cli get "risk_config:login_max_attempts"
```

---

### 8. 端口被占用（Windows）

**问题描述**：
```
listen tcp :8080: bind: Only one usage of each socket address
```

**解决方案**：
```powershell
# 查找占用端口的进程
netstat -ano | findstr :8080

# 杀掉进程（PowerShell）
Stop-Process -Id <PID> -Force

# 或者使用 cmd
taskkill /PID <PID> /F
```

---

### 9. Go 代码修改后 K8s 未生效

**问题描述**：
修改了 Go 代码，但 K8s 中的服务没有更新。

**原因**：
Docker 和 Kind K8s 使用不同的容器运行时，镜像存储不共享。

**解决方案**：
```bash
# 1. 重新构建镜像
docker build -f deploy/Dockerfile.<service> -t <image-name>:latest .

# 2. 加载镜像到 Kind
kind load docker-image <image-name>:latest --name <cluster-name>

# 3. 重启 Deployment
kubectl rollout restart deployment/<deployment-name> -n <namespace>

# 4. 验证镜像更新
docker exec <control-plane> crictl images | grep <image-name>
```

---

### 10. 风控配置热更新流程

**完整流程**：
```bash
# 1. 更新数据库配置
mysql -h <host> -uroot -p123456 userdb_test -e "
UPDATE risk_configs SET rule_value='3' WHERE rule_key='login_max_attempts';
"

# 2. 刷新到 Redis
curl -X POST http://<gateway-url>/api/v1/risk/config/refresh \
  -H "Authorization: Bearer <token>"

# 3. 验证配置
curl http://<gateway-url>/api/v1/risk/config \
  -H "Authorization: Bearer <token>"
```

---

## 📚 参考命令速查

### K8s 常用命令
```bash
# 查看 Pod 状态
kubectl get pods -n <namespace>

# 查看日志
kubectl logs -l app=<label> -n <namespace> --tail=50 -f

# 进入 Pod
kubectl exec -it <pod-name> -n <namespace> -- /bin/sh

# 查看 ConfigMap
kubectl get configmap -n <namespace>
kubectl get configmap <name> -n <namespace> -o yaml

# 更新 ConfigMap
kubectl create configmap <name> -n <namespace> \
  --from-file=<key>=<file> --dry-run=client -o yaml | kubectl apply -f -

# 重启 Deployment
kubectl rollout restart deployment/<name> -n <namespace>
kubectl rollout status deployment/<name> -n <namespace>
```

### Redis 调试命令
```bash
# 查看所有 key
docker exec <container> redis-cli keys "*"

# 查看特定 key
docker exec <container> redis-cli get "<key>"
docker exec <container> redis-cli exists "<key>"
docker exec <container> redis-cli ttl "<key>"

# 删除 key
docker exec <container> redis-cli del "<key>"
```
