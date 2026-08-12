package otel

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-project-learning/project/common/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func InitTracer(serviceName, host string, port int) func() {
	// cfg := configs.Conf.Jaeger

	ctx := context.Background()
	// 链接 jaeger otlp grpc
	client := otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(fmt.Sprintf("%s:%d", host, port)),
		otlptracegrpc.WithInsecure(),
	)
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		// panic(fmt.Sprintf("创建 OTLP Trace Exporter 失败: %v", err))
		logger.Fatal("创建 OTLP Trace Exporter 失败", zap.Error(err))
	}

	res, _ := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(5*time.Second)),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return func() {
		_ = tp.Shutdown(ctx)
	}
}

// GetTracer 获取追踪器
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func GetOtelMiddleware() gin.HandlerFunc {
	// 获取全局 Tracer
	tracer := otel.Tracer("gin-gateway")

	return func(c *gin.Context) {
		ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		spanName := c.FullPath()
		if spanName == "" {
			spanName = "unknown-route"
		}
		ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		span.SetAttributes(
			semconv.HTTPRequestMethodKey.String(c.Request.Method),
			semconv.URLPath(c.Request.URL.Path),
			attribute.String("http.route", c.FullPath()),
		)

		c.Request = c.Request.WithContext(ctx)

		c.Next()

		status := c.Writer.Status()
		span.SetAttributes(semconv.HTTPResponseStatusCode(status))
		if status >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", status))
		}
	}

}
