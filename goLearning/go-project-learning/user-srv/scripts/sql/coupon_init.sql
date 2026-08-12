-- ========================================
-- 优惠券模板初始化数据
-- ========================================

-- 优惠券模板表
CREATE TABLE IF NOT EXISTS `coupon_templates` (
    `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `created_at` DATETIME(3) DEFAULT NULL,
    `updated_at` DATETIME(3) DEFAULT NULL,
    `deleted_at` DATETIME(3) DEFAULT NULL,
    `name` VARCHAR(50) NOT NULL COMMENT '优惠券名称',
    `type` TINYINT NOT NULL DEFAULT 1 COMMENT '类型：1=满减券 2=折扣券 3=无门槛券',
    `discount_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '优惠金额或折扣率',
    `min_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '最低消费金额',
    `start_time` DATETIME(3) NOT NULL COMMENT '有效期开始',
    `end_time` DATETIME(3) NOT NULL COMMENT '有效期结束',
    `total_count` INT NOT NULL DEFAULT -1 COMMENT '发放总量，-1=无限制',
    `claimed_count` INT NOT NULL DEFAULT 0 COMMENT '已领取数量',
    `per_user_limit` INT NOT NULL DEFAULT 1 COMMENT '每人限领数量',
    `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1=启用 0=禁用',
    PRIMARY KEY (`id`),
    INDEX `idx_coupon_templates_deleted_at` (`deleted_at`),
    INDEX `idx_coupon_templates_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='优惠券模板表';

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
