项目骨架
product-srv/
├── api
│   └── pb
│       ├── product.proto       # RPC：商品新增、上下架、查SKU、预扣库存、查类目
│       ├── product.pb.go
│       └── product_grpc.pb.go
├── cmd
│   └── server/main.go          # 服务启动：nacos/mysql/redis/注册Nacos
├── config/config.go             # 配置：DB、Redis、ES、OSS、库存阈值
├── global/global.go             # 全局DB/Redis/ES客户端
├── internal
│   ├── application
│   │   └── product_app.go      # 应用层：编排新增商品、扣库存、上架下架、库存回滚
│   ├── domain
│   │   ├── entity              # 领域实体：Spu、Sku、Category
│   │   │   ├── spu.go
│   │   │   ├── sku.go
│   │   │   └── category.go
│   │   ├── rule
│   │   │   ├── stock_rule.go   # 库存规则：超卖校验、预扣规则
│   │   │   └── shelf_rule.go   # 商品上下架业务规则
│   │   └── service
│   │       ├── es_domain.go    # ES商品同步领域服务
│   │       └── oss_domain.go   # 商品图片上传OSS
│   ├── repository
│   │   ├── db                  # MySQL CRUD：spu/sku/category
│   │   │   ├── spu_repo.go
│   │   │   ├── sku_repo.go
│   │   │   └── category_repo.go
│   │   └── redis               # 商品缓存、热点库存缓存、分布式锁
│   │       ├── goods_cache.go
│   │       └── stock_cache.go
│   └── server/product_server.go # GRPC接口实现
├── pkg
│   ├── oss                     # 对象存储SDK封装
│   ├── esclient                # Elasticsearch封装
│   └── mq                      # MQ生产者：商品变更同步ES
├── scripts/sql/product_db.sql   # 建表：spu、sku、category
├── go.mod
└── go.sum