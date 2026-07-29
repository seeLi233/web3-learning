package config

import "os"

type Config struct {
	// 以太坊
	RPCURL       string // HTTP RPC
	WsRPCURL     string // WebSocket RPC (用于订阅)
	ContractAddr string // 要监听的 ERC20 合约地址

	// Redis
	RedisAddr string
	RedisPass string

	// PostgreSQL
	DBHost string
	DBPort string
	DBUser string
	DBPass string
	DBName string
}

func Load() *Config {
	return &Config{
		// 以太坊
		RPCURL:       getEnv("SEPOLIA_RPC_URL", ""),
		WsRPCURL:     getEnv("SEPOLIA_WSS_URL", ""),
		ContractAddr: getEnv("CONTRACT_ADDR", "0x1c7D4B196Cb0C7B01d743Fbc6116a902379C7238"),

		// Redis
		RedisAddr: getEnv("REDIS_ADDR", "localhost:6380"),
		RedisPass: getEnv("REDIS_PASS", ""),

		// PostgreSQL
		DBHost: getEnv("DB_HOST", "localhost"),
		DBPort: getEnv("DB_PORT", "5432"),
		DBUser: getEnv("DB_USER", "postgres"),
		DBPass: getEnv("DB_PASS", "postgres"),
		DBName: getEnv("DB_NAME", "task10"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
