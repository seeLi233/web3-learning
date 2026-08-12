项目骨架
order-srv/
├── api/pb/order.proto          # RPC：创建订单、取消订单、查询订单、修改订单状态
├── cmd/server/main.go
├── config/config.go            # 配置：超时取消时长、分库分表配置
├── global/global.go
├── internal
│   ├── application/order_app.go # 下单主流程：调商品扣库存→生成订单→发MQ→调用支付创建支付单
│   ├── domain
│   │   ├── entity              # OrderMain、OrderItem
│   │   ├── rule/order_rule.go   # 订单超时规则、取消回滚库存规则
│   │   └── service
│   │       ├── rpc_domain.go   # 跨服务RPC封装：调用User/Product/Promotion
│   │       └── delaymq.go      # 延时取消订单MQ领域逻辑
│   ├── repository
│   │   ├── db：order_main_repo、order_item_repo
│   │   └── redis：订单防重复幂等缓存
│   └── server/order_server.go
├── pkg/mq # RocketMQ生产者/消费者（订单创建、支付成功回调消费）
├── scripts/sql/order_db.sql
├── go.mod