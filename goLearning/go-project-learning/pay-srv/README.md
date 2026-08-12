项目骨架
pay-srv/
├── api/pb/pay.proto            # RPC：创建支付单、发起退款、查询支付结果
├── cmd/server/main.go
├── config/config.go            # 微信/支付宝商户密钥、回调地址配置(Nacos)
├── global/global.go
├── internal
│   ├── application/pay_app.go  # 生成支付链接、解析第三方回调、更新支付流水
│   ├── domain
│   │   ├── entity：PayRecord、RefundRecord
│   │   ├── rule/pay_rule.go    # 支付状态流转规则、重复回调幂等规则
│   │   └── service/pay_channel # 支付宝/微信渠道领域封装
│   ├── repository/db：pay_repo、refund_repo
│   └── server/pay_server.go
├── pkg
│   ├── alipay # 支付宝SDK封装
│   └── wechat # 微信支付SDK封装
├── scripts/sql/pay_db.sql
├── go.mod