-- 确保表存在（AutoMigrate 已经创建，这里只是保险）
CREATE TABLE IF NOT EXISTS `risk_configs` (
    `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    `created_at`  DATETIME(3) DEFAULT NULL,
    `updated_at`  DATETIME(3) DEFAULT NULL,
    `deleted_at`  DATETIME(3) DEFAULT NULL,
    `rule_key`    VARCHAR(50) NOT NULL,
    `rule_value`  VARCHAR(100) NOT NULL,
    `description` VARCHAR(255),
    `status`      BOOLEAN,
    UNIQUE KEY `idx_rule_key` (`rule_key`),
    INDEX `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 同IP最多注册次数
INSERT INTO risk_configs (rule_key, rule_value, description, status, created_at, updated_at) VALUES ('register_ip_limit', '3', '同IP 10分钟内最多注册次数', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE
    rule_value = '3',
    description = '同IP 10分钟内最多注册次数',
    updated_at = NOW();
-- 注册时间窗口(秒)
INSERT INTO risk_configs (rule_key, rule_value, description, status, created_at, updated_at) VALUES ('register_time_window', '600', '注册时间窗口(秒)', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE
    rule_value = '600',
    description = '注册时间窗口(秒)',
    updated_at = NOW();
-- 登录失败最大次数
INSERT INTO risk_configs (rule_key, rule_value, description, status, created_at, updated_at) VALUES ('login_max_attempts', '5', '登录失败最大次数', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE
    rule_value = '5',
    description = '登录失败最大次数',
    updated_at = NOW();
-- 锁定时长(秒)
INSERT INTO risk_configs (rule_key, rule_value, description, status, created_at, updated_at) VALUES ('lock_duration', '900', '锁定时长(秒)', 1, NOW(), NOW())
ON DUPLICATE KEY UPDATE
    rule_value = '900',
    description = '锁定时长(秒)',
    updated_at = NOW();

-- =====================================================
-- 验证数据
-- =====================================================
SELECT '=== 风控配置 ===' AS '';
SELECT id, rule_key, rule_value, description, status
FROM risk_config
WHERE deleted_at IS NULL;