项目骨架
gateway-user/
├── cmd
│   └── api
│       └── main.go               # 程序入口：初始化gin、注册路由、nacos、grpc客户端
├── api                           # 接口定义+swagger注解
│   ├── v1
│   │   └── user.go               # /api/v1/user 所有http接口定义
├── config                        # 配置结构体，从Nacos拉取配置
│   └── config.go
├── global                        # 全局变量：grpc客户端、redis、配置实例
│   └── global.go
├── internal
│   ├── handler                   # http控制器，接收请求，调用rpc
│   │   └── user_handler.go
│   ├── middleware                # 网关中间件
│   │   ├── auth_jwt.go           # JWT鉴权中间件
│   │   ├── limiter.go            # 接口限流中间件（登录防刷）
│   │   ├── cors.go               # 跨域
│   │   └── logger.go             # 请求日志
│   └── rpc_client                # gRPC客户端初始化，对接user-srv
│       └── user_rpc.go
├── pkg                           # 公共工具包
│   ├── jwt                       # jwt生成/解析
│   ├── resp                      # 统一返回封装：Success/Error
│   └── validator                 # 参数自定义校验
├── go.mod
├── go.sum
└── README.md