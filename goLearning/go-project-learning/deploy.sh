#!/bin/bash
set -e

NAMESPACE="go-project-test"
CLUSTER="go-project"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "========================================="
echo "🚀 Go 微服务项目部署脚本"
echo "========================================="

# 1. 构建镜像
echo ""
echo "📦 [1/5] 构建 Docker 镜像..."
echo "-----------------------------------------"
docker build --no-cache -f "$SCRIPT_DIR/deploy/Dockerfile.gateway-api" -t gateway-api:latest "$SCRIPT_DIR"
docker build --no-cache -f "$SCRIPT_DIR/deploy/Dockerfile.user-srv" -t user-srv:latest "$SCRIPT_DIR"
echo "✅ 镜像构建完成"

# 2. 加载镜像到 Kind
echo ""
echo "📦 [2/5] 加载镜像到 Kind 集群..."
echo "-----------------------------------------"
kind load docker-image gateway-api:latest --name $CLUSTER
kind load docker-image user-srv:latest --name $CLUSTER
echo "✅ 镜像加载完成"

# 3. 重启 Deployment
echo ""
echo "🔄 [3/5] 重启 Deployment..."
echo "-----------------------------------------"
kubectl rollout restart deployment/gateway-api -n $NAMESPACE
kubectl rollout restart deployment/user-srv -n $NAMESPACE
echo "✅ Deployment 已重启"

# 4. 等待部署完成
echo ""
echo "⏳ [4/5] 等待部署完成..."
echo "-----------------------------------------"
kubectl rollout status deployment/gateway-api -n $NAMESPACE --timeout=120s
kubectl rollout status deployment/user-srv -n $NAMESPACE --timeout=120s
echo "✅ 部署完成"

# 5. 验证部署
echo ""
echo "🔍 [5/5] 验证部署..."
echo "-----------------------------------------"
echo ""
echo "Pod 状态:"
kubectl get pods -n $NAMESPACE -l "app in (gateway-api, user-srv)"
echo ""
echo "路由注册:"
kubectl logs -l app=gateway-api -n $NAMESPACE --tail=30 | grep -E "address|profile" || echo "未找到路由日志"
echo ""

# 获取访问地址
NODE_PORT=$(kubectl get svc gateway-api-svc -n $NAMESPACE -o jsonpath='{.spec.ports[0].nodePort}')
echo "========================================="
echo "✅ 部署完成！"
echo "========================================="
echo ""
echo "📋 访问信息:"
echo "   NodePort: $NODE_PORT"
echo ""
echo "📋 测试命令:"
echo "   kubectl port-forward svc/gateway-api-svc 8080:8080 -n $NAMESPACE"
echo "   curl http://127.0.0.1:8080/api/v1/user/register"
echo ""
