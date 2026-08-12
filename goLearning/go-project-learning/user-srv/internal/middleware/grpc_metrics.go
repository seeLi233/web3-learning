package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/go-project-learning/project/common/pkg/metrics"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// GrpcMetricsInterceptor 返回一个 gRPC Unary 拦截器，自动采集 gRPC 调用指标
//
// 为什么用拦截器（Interceptor）而不是在每个 Server 方法里手动埋点？
// → 在 6 个 Service（User/OAuth/Address/Member/Coupon/Risk）的几十个方法里
//
//	逐一加指标代码 = 大量重复 + 容易遗漏。拦截器是 AOP 思想——切面织入。
//
// 作用：每个 gRPC Unary 调用自动：
//  1. 记录开始时间
//  2. 执行实际 handler
//  3. 更新 GrpcRequestsTotal（Counter） + GrpcRequestDuration（Histogram）
func GrpcMetricsInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// 1. 记录开始时间
	start := time.Now()

	// 2. 解析 service 名和 method 名
	// info.FullMethod 格式："/user.UserService/GetUser"
	// 为什么用 strings.SplitN 而不是 strings.Split？
	// → SplitN 限制分割次数为 2，防止方法名中包含 / 时被切成多段
	//    而且我们只需要 "UserService" 和 "GetUser" 两个部分
	serviceName, methodName := parseFullMethod(info.FullMethod)

	// 3. 执行真正的 gRPC handler
	resp, err := handler(ctx, req)

	// 4. 提取 gRPC status code
	// 为什么用 grpc status 而不是 err == nil？
	// → gRPC 的 err 可能携带丰富信息（DeadlineExceeded, Unauthenticated...）
	//    直接用 err 判"有没有错"丢失了具体错误类型
	//    status.Code 能区分 OK(0) / Internal(13) / Unavailable(14) 等
	grpcStatus := status.Code(err).String() // 如 "OK" "Internal" "Unauthenticated"

	// 5. Counter：请求计数 +1
	metrics.GrpcRequestsTotal.WithLabelValues(serviceName, methodName, grpcStatus).Inc()

	// 6. Histogram：记录本次调用延迟
	metrics.GrpcRequestDuration.WithLabelValues(serviceName, methodName).Observe(time.Since(start).Seconds())

	return resp, err
}

// parseFullMethod 把 "/user.UserService/GetUser" 解析为 ("UserService", "GetUser")
//
// 为什么单独抽一个函数？
// → 保持拦截器主体清晰；而且这个解析逻辑未来可能用在日志/链路追踪中（可复用）
func parseFullMethod(fullMethod string) (service, method string) {
	// fullMethod = "/user.UserService/GetUser"
	// 去掉开头的 "/"
	fullMethod = strings.TrimPrefix(fullMethod, "/")

	// parts = ["user.UserService", "GetUser"]
	parts := strings.SplitN(fullMethod, "/", 2)
	if len(parts) != 2 {
		return "unknown", "unknown"
	}

	service = parts[0]
	method = parts[1]

	// 去掉包名前缀： "user.UserService" → "UserService"
	// 为什么去掉包名？
	// → 所有服务都在 user-srv 下，包名 user. 是冗余信息
	//    而且 labels 中去掉重复前缀能让 Grafana 面板更简洁
	if idx := strings.LastIndex(service, "."); idx != -1 {
		service = service[idx+1:]
	}

	return service, method
}
