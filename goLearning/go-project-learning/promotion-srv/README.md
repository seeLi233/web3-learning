项目骨架
promotion-srv/
├── api/pb/promotion.proto      # RPC：优惠券领取/核销、秒杀扣减资格、满减计算
├── cmd/server/main.go
├── config/config.go            # 秒杀限流配置、优惠券过期规则配置
├── global/global.go
├── internal
│   ├── application/prom_app.go # 绑定优惠券、秒杀下单资格校验、核销优惠券
│   ├── domain
│   │   ├── entity：Coupon、UserCoupon、SeckillGoods
│   │   ├── rule
│   │   │   ├── coupon_rule.go  # 优惠券使用规则、过期规则
│   │   │   └── seckill_rule.go # 秒杀限购、防超卖规则
│   │   └── service/seckill_domain # 秒杀库存预热、资格缓存
│   ├── repository
│   │   ├── db：coupon_repo、user_coupon_repo、seckill_repo
│   │   └── redis：秒杀库存、用户领券记录缓存
│   └── server/prom_server.go
├── pkg/mq # 消费订单消息：订单完成自动核销优惠券
├── scripts/sql/promotion_db.sql
├── go.mod