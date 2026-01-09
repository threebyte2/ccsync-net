#!/bin/bash

# 剪贴板同步工具 - Linux 依赖安装脚本

echo "=========================================="
echo "剪贴板同步工具 - 安装 Linux 依赖"
echo "=========================================="
echo ""

# 检测Linux发行版
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
else
    echo "无法检测 Linux 发行版"
    exit 1
fi

echo "检测到系统: $PRETTY_NAME"
echo ""

# 检测显示服务器类型
IS_WAYLAND=false
if [ "$XDG_SESSION_TYPE" = "wayland" ] || [ -n "$WAYLAND_DISPLAY" ]; then
    IS_WAYLAND=true
    echo "检测到 Wayland 环境"
else
    echo "检测到 X11 环境"
fi
echo ""

install_tool() {
    local TOOL=$1
    if command -v $TOOL &> /dev/null; then
        echo "✓ $TOOL 已安装"
        $TOOL --version 2>/dev/null || echo "$TOOL (version info unavailable)"
        return 0
    fi

    echo "$TOOL 未安装，正在安装..."
    
    case "$OS" in
        debian|ubuntu|linuxmint|kali)
            echo "使用 apt-get 安装 $TOOL..."
            sudo apt-get update
            sudo apt-get install -y $TOOL
            ;;
        fedora|rhel|centos)
            echo "使用 dnf 安装 $TOOL..."
            sudo dnf install -y $TOOL
            ;;
        arch|manjaro)
            echo "使用 pacman 安装 $TOOL..."
            sudo pacman -S --noconfirm $TOOL
            ;;
        opensuse*|suse)
            echo "使用 zypper 安装 $TOOL..."
            sudo zypper install -y $TOOL
            ;;
        *)
            echo "不支持的发行版: $OS"
            echo "请手动安装 $TOOL"
            exit 1
            ;;
    esac
}

if [ "$IS_WAYLAND" = true ]; then
    # Wayland 环境，优先安装 wl-clipboard
    install_tool "wl-clipboard"
    
    # 验证 wl-copy
    if command -v wl-copy &> /dev/null; then
        echo ""
        echo "✓ wl-clipboard 安装/检查成功！"
    else
        echo ""
        echo "✗ wl-clipboard 安装失败，请手动安装。"
        exit 1
    fi
else
    # X11 环境，安装 xclip
    install_tool "xclip"

    # 验证 xclip
    if command -v xclip &> /dev/null; then
        echo ""
        echo "✓ xclip 安装/检查成功！"
    else
        echo ""
        echo "✗ xclip 安装失败，请手动安装。"
        exit 1
    fi
fi

echo ""
echo "=========================================="
echo "依赖检查完成。"
