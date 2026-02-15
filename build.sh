#!/bin/bash
echo "📂 正在编译应用 (使用 -tags webkit2_41)..."
# 获取当前真实用户，如果未设置则默认为 jhh
REAL_USER=${REAL_USER:-jhh}
# 使用 sudo 以指定用户身份运行编译命令
sudo -u "$REAL_USER" env "PATH=$PATH" "GOPATH=$GOPATH" /home/jhh/go/bin/wails build -tags webkit2_41

echo "📂 正在编译后台服务..."
# 确保输出目录存在
mkdir -p build/bin
# 编译服务
sudo -u "$REAL_USER" env "PATH=$PATH" "GOPATH=$GOPATH" /usr/local/go/bin/go build -o build/bin/ccsync-service ./cmd/ccsync-service
