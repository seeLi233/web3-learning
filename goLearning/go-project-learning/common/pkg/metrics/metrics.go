package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ==================== HTTP 指标（gateway-api 使用） ====================

// 为什么用 NewCounterVec 而不是 NewCounter？
// → Vec = Vector，带 Label 的版本。不带 Label 的 Counter 只能计一个总数；
//
//	带 Label 的可以按 method/path/status 分组统计——Q1: "GET /users 的 QPS 是多少？"
//
// 作用：gin 中间件每次请求 +1，累计后用于 rate() 计算 QPS
var HttpRequestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "app_http_requests_total", // 指标名：app 前缀区分业务指标 vs 系统指标
		Help: "Total number of Http requests.",
	},
	[]string{"method", "path", "status"}, // 三个维度：什么方法、什么路径、返回什么状态码
)

// 为什么用 HistogramVec？
// → 不是记"每次请求花了多少 ms"，而是记"这个请求落在哪个延迟桶里"
//
//	比如 100ms 的请求落在 .1 桶和 .25 桶里，各桶计数 +1
//
// 作用：计算 P99 延迟——histogram_quantile(0.99, rate(...))
var HttpRequestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "app_http_request_duration_seconds",
		Help: "Http request latency in seconds.",
		// 为什么自定义桶而不是用 DefBuckets？
		// → DefBuckets 从 5ms 到 10s，对 API 服务来说太粗了
		//    自定义桶更精细：50ms/100ms/250ms/500ms/1s/2.5s/5s/10s
		//    面试技巧：桶的分布要根据你的 P99 目标来定——如果 SLO 是 P99<1s，桶要覆盖 0~2s
		Buckets: []float64{0.05, 0.1, 0.25, 1.0, 2.5, 5.0, 10.0},
	},
	[]string{"method", "path"},
)

// ==================== gRPC 指标（user-srv 使用） ====================

// 为什么 gRPC 和 HTTP 分开？
// → 两者的维度不同：HTTP 有 method+path+status，gRPC 有 service+method+status_code
//
//	而且 gRPC 用的是 status code（OK=0, Internal=13），不是 HTTP status
//
// 作用：统计每个 gRPC 方法的调用次数
var GrpcRequestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "app_grpc_requests_total",
		Help: "Total number of gRPC requests.",
	},
	[]string{"service", "method", "status"}, // service=UserService, method=GetUser
)

// 作用：统计 gRPC 方法调用的延迟分布
var GrpcRequestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "app_grpc_request_duration_seconds",
		Help:    "gRPC request latency in seconds.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0},
	},
	[]string{"service", "method"},
)

// ==================== 业务指标（两个服务共享） ====================

// 为什么在线用户用 Gauge？
// → 在线人数是"当前值"——登录+1，登出-1，可增可减
//
//	如果用 Counter，用户登出后数字不降，完全没意义
//
// 作用：Grafana 面板直接显示 → "当前在线: 1,234 人"
var OnlineUsers = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "app_online_users_current",
		Help: "Current number of online users.",
	},
)

// 为什么活跃连接数也用 Gauge？
// → 和在线用户同理——建立连接+1，断开-1
var ActiveConnections = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "app_active_connections",
		Help: "Current number of active connections (DB/Redis/gRPC)",
	},
)

// ==================== 注册与暴露 ====================

// 为什么用 init() 而不是显式调用？
// → init() 在包被 import 时自动执行，保证指标在任何代码使用前就已注册
//
//	缺点是"隐式副作用"——但这个场景下正是我们需要的（引入即注册）
func init() {
	// MustRegister：注册失败直接 panic——指标重复注册是代码 bug，应该立即暴露
	prometheus.MustRegister(HttpRequestsTotal)
	prometheus.MustRegister(HttpRequestDuration)
	prometheus.MustRegister(GrpcRequestsTotal)
	prometheus.MustRegister(GrpcRequestDuration)
	prometheus.MustRegister(OnlineUsers)
	prometheus.MustRegister(ActiveConnections)
}

// MetricsHandler 返回 /metrics 端点的 HTTP Handler
// 为什么用 promhttp.Handler() 而不是自己写？
// → promhttp 自动收集：进程指标（CPU/内存/goroutine）+ 业务指标（上面注册的）
//
//	自己写 TextFormat 费力且容易丢 Go runtime 指标
//
// 作用：挂到 Gin router 或单独的 HTTP server 上
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
