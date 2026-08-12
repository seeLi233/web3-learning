#!/bin/bash

# =====================================================
# OAuth2.0 授权码模式测试脚本 - K8s 环境
# =====================================================
# 使用方法:
#   chmod +x scripts/test_oauth_k8s.sh
#   ./scripts/test_oauth_k8s.sh
# =====================================================

set -e

# 配置
GATEWAY_URL="http://localhost:8081"
CLIENT_ID="mall-frontend"
CLIENT_SECRET="mall-frontend-secret-k8s-test"
REDIRECT_URI="http://localhost:3000/callback"
USERNAME="k8stest"
PASSWORD="123456"

echo "=========================================="
echo "OAuth2.0 授权码模式测试"
echo "=========================================="

# 检查 port-forward 是否运行
echo ""
echo "检查 port-forward 状态..."
if ! curl -s "$GATEWAY_URL" > /dev/null 2>&1; then
    echo "❌ 无法连接到 $GATEWAY_URL"
    echo "请先运行: kubectl port-forward svc/gateway-api-svc 8081:8080 -n go-project-test"
    exit 1
fi
echo "✅ 连接成功"

# Step 1: 获取授权码
echo ""
echo "=========================================="
echo "Step 1: 获取授权码"
echo "=========================================="
echo "POST $GATEWAY_URL/api/v1/oauth/authorize"
echo ""

RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/oauth/authorize" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "response_type=code&client_id=$CLIENT_ID&redirect_uri=$REDIRECT_URI&scope=read&state=xyz123&account=$USERNAME&password=$PASSWORD")

echo "响应: $RESPONSE"

# 提取授权码
AUTH_CODE=$(echo $RESPONSE | grep -o '"code":"[^"]*"' | cut -d'"' -f4)

if [ -z "$AUTH_CODE" ]; then
    echo "❌ 获取授权码失败"
    exit 1
fi

echo ""
echo "✅ 授权码: $AUTH_CODE"

# Step 2: 用授权码换取 Token
echo ""
echo "=========================================="
echo "Step 2: 用授权码换取 Token"
echo "=========================================="
echo "POST $GATEWAY_URL/api/v1/oauth/token"
echo ""

RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/oauth/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=authorization_code&code=$AUTH_CODE&redirect_uri=$REDIRECT_URI&client_id=$CLIENT_ID&client_secret=$CLIENT_SECRET")

echo "响应: $RESPONSE"

# 提取 Token
ACCESS_TOKEN=$(echo $RESPONSE | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
REFRESH_TOKEN=$(echo $RESPONSE | grep -o '"refresh_token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$ACCESS_TOKEN" ]; then
    echo "❌ 获取 Token 失败"
    exit 1
fi

echo ""
echo "✅ Access Token: $ACCESS_TOKEN"
echo "✅ Refresh Token: $REFRESH_TOKEN"

# Step 3: 获取用户信息
echo ""
echo "=========================================="
echo "Step 3: 获取用户信息"
echo "=========================================="
echo "GET $GATEWAY_URL/api/v1/oauth/userinfo"
echo ""

RESPONSE=$(curl -s "$GATEWAY_URL/api/v1/oauth/userinfo" \
  -H "Authorization: Bearer $ACCESS_TOKEN")

echo "响应: $RESPONSE"

# 检查是否成功
if echo "$RESPONSE" | grep -q '"user_id"'; then
    echo ""
    echo "✅ 获取用户信息成功"
else
    echo ""
    echo "❌ 获取用户信息失败"
fi

# Step 4: 刷新 Token
echo ""
echo "=========================================="
echo "Step 4: 刷新 Token"
echo "=========================================="
echo "POST $GATEWAY_URL/api/v1/oauth/token"
echo ""

RESPONSE=$(curl -s -X POST "$GATEWAY_URL/api/v1/oauth/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=refresh_token&refresh_token=$REFRESH_TOKEN&client_id=$CLIENT_ID&client_secret=$CLIENT_SECRET")

echo "响应: $RESPONSE"

# 提取新的 Token
NEW_ACCESS_TOKEN=$(echo $RESPONSE | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
NEW_REFRESH_TOKEN=$(echo $RESPONSE | grep -o '"refresh_token":"[^"]*"' | cut -d'"' -f4)

if [ -z "$NEW_ACCESS_TOKEN" ]; then
    echo "❌ 刷新 Token 失败"
else
    echo ""
    echo "✅ 新 Access Token: $NEW_ACCESS_TOKEN"
    echo "✅ 新 Refresh Token: $NEW_REFRESH_TOKEN"
fi

echo ""
echo "=========================================="
echo "测试完成！"
echo "=========================================="
