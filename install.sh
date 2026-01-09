#!/bin/bash

# 设置错误处理
set -e

# 1. 环境与变量准备
REAL_USER=${SUDO_USER:-$USER}
USER_HOME=$(getent passwd "$REAL_USER" | cut -d: -f6)

# 确保在 sudo 运行下也能找到用户的 go, wails, npm
USER_PATH=$(sudo -u "$REAL_USER" bash -c 'echo $PATH')
export GOPATH=$(sudo -u "$REAL_USER" go env GOPATH 2>/dev/null || echo "$USER_HOME/go")
export PATH="$USER_PATH:/usr/local/go/bin:$GOPATH/bin:$USER_HOME/.local/bin:$PATH"

APP_NAME="ccsync-net"
DISPLAY_NAME="CCSync 指纹浏览器管理器"
ICON_SOURCE="build/ccsync-net.png"
INSTALL_DIR="$USER_HOME/.local/bin"
ICON_DIR="$USER_HOME/.local/share/icons"
DESKTOP_DIR="$USER_HOME/.local/share/applications"

echo "🚀 开始安装 $DISPLAY_NAME..."

# 2. 权限清理 (防止之前的 sudo 编译导致 permission denied)
echo "🧹 正在清理文件权限..."
chown -R "$REAL_USER:$REAL_USER" .

# 3. 确保安装目录存在
sudo -u "$REAL_USER" mkdir -p "$INSTALL_DIR"
sudo -u "$REAL_USER" mkdir -p "$ICON_DIR"
sudo -u "$REAL_USER" mkdir -p "$DESKTOP_DIR"

# 4. 编译项目 (必须以原始用户身份运行，以避免 npm/wails 环境冲突)
echo "📂 正在编译应用 (使用 -tags webkit2_41)..."
sudo -u "$REAL_USER" env "PATH=$PATH" "GOPATH=$GOPATH" wails build -tags webkit2_41

# 5. 部署文件
echo "📦 部署二进制文件与图标..."
cp "build/bin/$APP_NAME" "$INSTALL_DIR/"
chown "$REAL_USER:$REAL_USER" "$INSTALL_DIR/$APP_NAME"
chmod +x "$INSTALL_DIR/$APP_NAME"

if [ -f "$ICON_SOURCE" ]; then
    cp "$ICON_SOURCE" "$ICON_DIR/$APP_NAME.png"
    chown "$REAL_USER:$REAL_USER" "$ICON_DIR/$APP_NAME.png"
fi

# 6. 创建快捷方式
echo "🖥️ 创建桌面快捷方式..."
cat > "$DESKTOP_DIR/$APP_NAME.desktop" <<EOF
[Desktop Entry]
Name=$DISPLAY_NAME
Comment=Professional Browser Profile Manager
Exec=$INSTALL_DIR/$APP_NAME
Icon=$ICON_DIR/$APP_NAME.png
Type=Application
Categories=Unknown;
Keywords=browser;profile;manager;
StartupNotify=true
Terminal=false
EOF
chown "$REAL_USER:$REAL_USER" "$DESKTOP_DIR/$APP_NAME.desktop"

echo "✅ 安装完成！"
echo "您现在可以从菜单启动 '$DISPLAY_NAME'。"
