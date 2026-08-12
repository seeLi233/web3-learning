-- ========================================
-- 优惠券测试数据初始化脚本
-- ========================================

-- 使用数据库
USE go_project;

-- 清空旧数据（可选，谨慎使用）
-- TRUNCATE TABLE coupon_templates;
-- TRUNCATE TABLE user_coupons;

-- ========================================
-- 1. 插入优惠券模板
-- ========================================

-- 满减券
INSERT INTO coupon_templates (name, type, discount_amount, min_amount, start_time, end_time, total_count, claimed_count, per_user_limit, status, created_at, updated_at, deleted_at) VALUES
('满100减10', 1, 10.00, 100.00, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 100, 0, 1, 1, NOW(), NOW(), NULL),
('满200减30', 1, 30.00, 200.00, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 50, 0, 2, 1, NOW(), NOW(), NULL),
('满500减80', 1, 80.00, 500.00, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 30, 0, 1, 1, NOW(), NOW(), NULL);

-- 折扣券
INSERT INTO coupon_templates (name, type, discount_amount, min_amount, start_time, end_time, total_count, claimed_count, per_user_limit, status, created_at, updated_at, deleted_at) VALUES
('85折优惠券', 2, 0.85, 0.00, '2026-01-01 00:00:00', '2026-12-31 23:59:59', -1, 0, 3, 1, NOW(), NOW(), NULL),
('9折优惠券', 2, 0.90, 100.00, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 200, 0, 2, 1, NOW(), NOW(), NULL);

-- 无门槛券
INSERT INTO coupon_templates (name, type, discount_amount, min_amount, start_time, end_time, total_count, claimed_count, per_user_limit, status, created_at, updated_at, deleted_at) VALUES
('无门槛5元券', 3, 5.00, 0.00, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 200, 0, 1, 1, NOW(), NOW(), NULL),
('无门槛10元券', 3, 10.00, 0.00, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 100, 0, 1, 1, NOW(), NOW(), NULL);

-- 已过期的优惠券（测试过期场景）
INSERT INTO coupon_templates (name, type, discount_amount, min_amount, start_time, end_time, total_count, claimed_count, per_user_limit, status, created_at, updated_at, deleted_at) VALUES
('已过期优惠券', 1, 20.00, 150.00, '2025-01-01 00:00:00', '2025-12-31 23:59:59', 50, 10, 1, 1, NOW(), NOW(), NULL);

-- 已禁用的优惠券（测试禁用场景）
INSERT INTO coupon_templates (name, type, discount_amount, min_amount, start_time, end_time, total_count, claimed_count, per_user_limit, status, created_at, updated_at, deleted_at) VALUES
('已禁用优惠券', 1, 15.00, 120.00, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 80, 5, 1, 0, NOW(), NOW(), NULL);

-- 库存已满的优惠券（测试库存不足场景）
INSERT INTO coupon_templates (name, type, discount_amount, min_amount, start_time, end_time, total_count, claimed_count, per_user_limit, status, created_at, updated_at, deleted_at) VALUES
('库存已满券', 1, 25.00, 180.00, '2026-01-01 00:00:00', '2026-12-31 23:59:59', 10, 10, 1, 1, NOW(), NOW(), NULL);

-- ========================================
-- 2. 验证数据
-- ========================================
SELECT '优惠券模板数据：' AS info;
SELECT id, name, type, discount_amount, min_amount, total_count, claimed_count, per_user_limit, status,
       DATE_FORMAT(start_time, '%Y-%m-%d') as start_date,
       DATE_FORMAT(end_time, '%Y-%m-%d') as end_date
FROM coupon_templates
ORDER BY id;

-- ========================================
-- 说明
-- ========================================
-- type 字段：1=满减券 2=折扣券 3=无门槛券
-- status 字段：1=启用 0=禁用
-- total_count：-1 表示无限制
-- per_user_limit：每人限领数量
