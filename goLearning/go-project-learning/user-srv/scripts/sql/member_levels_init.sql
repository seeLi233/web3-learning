-- =====================================================
-- 会员等级预置数据 - 本地环境
-- =====================================================
-- 使用方法:
--   方式1 (命令行): mysql -u root -p123456 userdb < member_levels_init.sql
--   方式2 (Navicat): 直接复制粘贴执行
--   方式3 (Go 启动时): 代码里自动初始化（见下方说明）
-- =====================================================

-- 确保表存在（AutoMigrate 已经创建，这里只是保险）
CREATE TABLE IF NOT EXISTS `member_levels` (
    `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    `created_at`  DATETIME(3) DEFAULT NULL,
    `updated_at`  DATETIME(3) DEFAULT NULL,
    `deleted_at`  DATETIME(3) DEFAULT NULL,
    `level_name`  VARCHAR(20)  NOT NULL,
    `level_value` INT          NOT NULL,
    `min_growth`  INT          NOT NULL DEFAULT 0,
    `max_growth`  INT          NOT NULL DEFAULT 0,
    `discount`    DECIMAL(3,2) NOT NULL DEFAULT 1.00,
    `icon`        VARCHAR(255) DEFAULT '',
    `description` VARCHAR(500) DEFAULT '',
    `status`      TINYINT      NOT NULL DEFAULT 1,
    UNIQUE KEY `idx_level_value` (`level_value`),
    INDEX `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- =====================================================
-- 插入 4 个等级（用 level_value 做唯一判断，重复执行不会报错）
-- =====================================================

-- 1. 普通会员（默认等级，0 成长值起步）
INSERT INTO member_levels (level_name, level_value, min_growth, max_growth, discount, icon, description, status, created_at, updated_at)
VALUES ('普通会员', 0, 0, 999, 1.00, 'icon_normal', '注册即享，基础购物权益', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE
    level_name = '普通会员',
    min_growth = 0,
    max_growth = 999,
    discount = 1.00,
    icon = 'icon_normal',
    description = '注册即享，基础购物权益',
    updated_at = NOW();

-- 2. 银卡会员（1000 成长值升级，98 折）
INSERT INTO member_levels (level_name, level_value, min_growth, max_growth, discount, icon, description, status, created_at, updated_at)
VALUES ('银卡会员', 1, 1000, 4999, 0.98, 'icon_silver', '满1000成长值自动升级，享98折+包邮券', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE
    level_name = '银卡会员',
    min_growth = 1000,
    max_growth = 4999,
    discount = 0.98,
    icon = 'icon_silver',
    description = '满1000成长值自动升级，享98折+包邮券',
    updated_at = NOW();

-- 3. 金卡会员（5000 成长值升级，95 折）
INSERT INTO member_levels (level_name, level_value, min_growth, max_growth, discount, icon, description, status, created_at, updated_at)
VALUES ('金卡会员', 2, 5000, 19999, 0.95, 'icon_gold', '满5000成长值自动升级，享95折+专属折扣', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE
    level_name = '金卡会员',
    min_growth = 5000,
    max_growth = 19999,
    discount = 0.95,
    icon = 'icon_gold',
    description = '满5000成长值自动升级，享95折+专属折扣',
    updated_at = NOW();

-- 4. 钻石会员（20000 成长值升级，9 折）
INSERT INTO member_levels (level_name, level_value, min_growth, max_growth, discount, icon, description, status, created_at, updated_at)
VALUES ('钻石会员', 3, 20000, -1, 0.90, 'icon_diamond', '满20000成长值自动升级，享9折+专属客服+优先发货', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE
    level_name = '钻石会员',
    min_growth = 20000,
    max_growth = -1,
    discount = 0.90,
    icon = 'icon_diamond',
    description = '满20000成长值自动升级，享9折+专属客服+优先发货',
    updated_at = NOW();

-- =====================================================
-- 验证数据
-- =====================================================
SELECT '=== 会员等级配置 ===' AS '';
SELECT id, level_name, level_value, min_growth, max_growth, discount, status
FROM member_levels
WHERE deleted_at IS NULL
ORDER BY level_value;
