-- 确保表存在（AutoMigrate 已经创建，这里只是保险）
CREATE TABLE IF NOT EXISTS `ip_blacklists` (
    `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    `created_at`  DATETIME(3) DEFAULT NULL,
    `updated_at`  DATETIME(3) DEFAULT NULL,
    `deleted_at`  DATETIME(3) DEFAULT NULL,
    `ip`          VARCHAR(45) NOT NULL,
    `source`      VARCHAR(50),
    `reason`      VARCHAR(200),
    `user_id`     BIGINT,
    `status`      BOOLEAN,
    `deadline`    DATETIME(3) DEFAULT NULL,
    INDEX `idx_ip` (`ip`),
    INDEX `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


-- =====================================================
-- 验证数据
-- =====================================================
SELECT '=== 黑名单 ===' AS '';
SELECT id, ip, source, reason, user_id, status
FROM ip_blacklists
WHERE deleted_at IS NULL;