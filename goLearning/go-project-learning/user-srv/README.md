项目骨架
user-srv/
├── api                           # proto文件 + 自动生成gRPC代码
│   ├── pb
│   │   ├── user.proto            # proto接口定义：注册/登录/地址/会员/黑名单rpc方法
│   │   ├── user.pb.go
│   │   └── user_grpc.pb.go
├── cmd
│   └── server
│       └── main.go               # 启动入口：nacos、mysql、redis、注册gRPC服务到Nacos
├── config                        # 配置结构体：数据库、redis、短信、jwt密钥、风控阈值
│   └── config.go
├── global                        # 全局实例：db、redis、config
│   └── global.go
├── internal
│   ├── application               # 应用层：业务编排【重点】
│   │   └── user_app.go
│   │       // 方法：Register、CodeLogin、PwdLogin、UpdateUser、AddressCRUD、BindCoupon、AddBlack
│   ├── domain                    # 领域层：实体+领域规则（风控、加密、会员、黑名单逻辑）
│   │   ├── entity                # 领域实体：User、UserAddress、Member、BlackUser
│   │   │   ├── user.go
│   │   │   ├── address.go
│   │   │   ├── member.go
│   │   │   └── black.go
│   │   ├── rule                  # 领域规则（核心业务约束）
│   │   │   ├── risk_rule.go      # 风控规则：频繁登录、异常注册、短信频次限制
│   │   │   ├── encrypt_rule.go   # AES手机号加密、脱敏规则
│   │   │   └── member_rule.go   # 会员升级规则
│   │   └── service               # 领域服务
│   │       ├── sms_domain.go     # 短信领域：验证码生成、发送校验
│   │       └── jwt_domain.go      # jwt签发、黑名单校验
│   ├── repository                # 仓储层：DB+Redis数据存取，只做数据操作
│   │   ├── db                    # mysql操作
│   │   │   ├── user_repo.go
│   │   │   ├── address_repo.go
│   │   │   ├── member_repo.go
│   │   │   ├── black_repo.go
│   │   │   └── coupon_rel_repo.go
│   │   └── redis                 # redis缓存、限流、验证码、jwt黑名单
│   │       ├── code_cache.go
│   │       ├── limit_cache.go
│   │       ├── user_cache.go
│   │       └── jwt_black_cache.go
│   └── server                   # gRPC服务实现，实现proto定义的接口
│       └── user_server.go
├── pkg                           # 公共工具
│   ├── aes                       # AES加密解密（手机号加密）
│   ├── bcrypt                    # 密码加密
│   ├── sms                       # 对接第三方短信SDK
│   ├── mq                        # rocketMQ生产者：注册消息、会员变更消息
│   └── snowflake                 # 雪花ID生成
├── scripts                       # 脚本
│   ├── sql                       # 建表SQL
│   │   └── user_db.sql
│   └── docker                    # Dockerfile、docker-compose.yml
├── go.mod
├── go.sum
└── README.md