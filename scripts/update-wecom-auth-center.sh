#!/bin/bash

# wecom-auth-center 二进制部署在线更新脚本：分页抓取 GitHub 全部 Release，选择版本后自动完成下载校验、停止服务、替换二进制并重启

REPO="zyx3721/wecom-auth-center"
INSTALL_DIR="${INSTALL_DIR:-/data/wecom-auth-center}"
GH_PROXY_PREFIX="${GH_PROXY_PREFIX:-}"
GH_TOKEN="${GH_TOKEN:-}"

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
if [ -z "${RUN_SCRIPT:-}" ]; then
    if [ -f "$INSTALL_DIR/run.sh" ]; then
        RUN_SCRIPT="$INSTALL_DIR/run.sh"
    else
        RUN_SCRIPT="$SCRIPT_DIR/run.sh"
    fi
fi

TMP_DIR=""
CURRENT_VERSION=""
TARGET_TAG=""
TARGET_PRE="false"
ARCH=""
BACKEND_ASSET=""
BACKEND_URL=""
SUMS_URL=""
TAGS=()
PRES=()

echo_log_info() {
    echo -e "$(date +'%F %T') - [Info] $*"
}

echo_log_warn() {
    echo -e "$(date +'%F %T') - [Warn] $*"
}

echo_log_error() {
    echo -e "$(date +'%F %T') - [Error] \033[31m$*\033[0m"
    exit 1
}

cleanup() {
    case "$TMP_DIR" in
        /tmp/wecom-auth-center-update.*) [ -d "$TMP_DIR" ] && rm -rf "$TMP_DIR" ;;
    esac
}
trap cleanup EXIT

# check_env 检查安装目录、启停脚本与基础依赖是否就绪
check_env() {
    [ -d "$INSTALL_DIR" ] || echo_log_error "安装目录 $INSTALL_DIR 不存在，可通过环境变量 INSTALL_DIR 指定"
    [ -f "$RUN_SCRIPT" ] || echo_log_error "启停脚本不存在: $RUN_SCRIPT，可通过环境变量 RUN_SCRIPT 指定"
    command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1 || echo_log_error "系统缺少 curl 或 wget，无法访问 GitHub"
    command -v tar >/dev/null 2>&1 || echo_log_error "系统缺少 tar，无法解压安装包"
}

# detect_arch 识别服务器 CPU 架构并映射为发布包命名后缀
detect_arch() {
    case "$(uname -m)" in
        x86_64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) echo_log_error "不支持的 CPU 架构: $(uname -m)" ;;
    esac
}

# get_current_version 通过 wecom-auth-center -v 读取当前已部署的版本号
get_current_version() {
    if [ -x "$INSTALL_DIR/wecom-auth-center" ]; then
        CURRENT_VERSION=$("$INSTALL_DIR/wecom-auth-center" -v 2>/dev/null | head -n1 | awk '{print $2}')
    fi
}

# parse_releases 从单页 Release JSON 中按字段顺序提取版本号与预发布标记，并过滤草稿
parse_releases() {
    grep -oE '"tag_name": *"[^"]+"|"draft": *(true|false)|"prerelease": *(true|false)' "$1" \
        | sed -E 's/^"tag_name": *"([^"]+)"/\1/; s/^"(draft|prerelease)": *(true|false)$/\2/' \
        | paste - - - \
        | awk -F'\t' '$2 == "false" {print $1 "\t" $3}'
}

# fetch_all_releases 分页抓取仓库全部 Release 并汇总为 "版本号<TAB>预发布标记" 列表
fetch_all_releases() {
    local page=1 count
    local auth_args=()
    [ -n "$GH_TOKEN" ] && auth_args=(-H "Authorization: Bearer $GH_TOKEN")
    : > "$TMP_DIR/releases.txt"
    echo_log_info "正在从 GitHub 抓取 $REPO 全部版本列表..."
    while :; do
        curl -fsS --connect-timeout 10 --max-time 60 "${auth_args[@]}" \
            -H "Accept: application/vnd.github+json" \
            "https://api.github.com/repos/$REPO/releases?per_page=100&page=$page" > "$TMP_DIR/page.json" \
            || echo_log_error "获取 GitHub Release 列表失败（第 $page 页），请检查网络或稍后重试"
        count=$(grep -c '"tag_name"' "$TMP_DIR/page.json" || true)
        [ "$count" -eq 0 ] && break
        parse_releases "$TMP_DIR/page.json" >> "$TMP_DIR/releases.txt"
        [ "$count" -lt 100 ] && break
        page=$((page+1))
    done
    [ -s "$TMP_DIR/releases.txt" ] || echo_log_error "仓库 $REPO 没有已发布的版本，或响应解析失败"
}

# show_version_list 清屏展示全部可选版本，并为预发布与当前已安装版本添加标注
show_version_list() {
    clear
    local total
    total=$(wc -l < "$TMP_DIR/releases.txt" | tr -d ' ')
    if [ -n "$CURRENT_VERSION" ]; then
        echo_log_info "当前系统 wecom-auth-center 版本为 \033[32m$CURRENT_VERSION\033[0m ，仓库 $REPO 可安装的版本有（共 $total 个）：\n"
    else
        echo_log_warn "未检测到已安装的 wecom-auth-center 版本，仓库 $REPO 可安装的版本有（共 $total 个）：\n"
    fi
    local n=0 tag pre marks
    while IFS=$'\t' read -r tag pre; do
        n=$((n+1))
        TAGS+=("$tag")
        PRES+=("${pre:-false}")
        marks=""
        [ "${pre:-false}" = "true" ] && marks="$marks \033[33m[预发布]\033[0m"
        [ "$tag" = "$CURRENT_VERSION" ] && marks="$marks \033[32m(已安装)\033[0m"
        printf "   %3s     %-10s%b\n" "$n" "$tag" "$marks"
    done < "$TMP_DIR/releases.txt"
    echo ""
}

# select_version 循环等待用户输入编号并校验范围，确定目标安装版本
select_version() {
    local number
    while true; do
        read -rp "请输入要安装的版本对应编号: " number
        if [ -z "$number" ]; then
            echo_log_warn "\033[33m编号不能为空，请重新输入!\033[0m"
        elif ! [[ "$number" =~ ^[0-9]+$ ]] || [ "$number" -lt 1 ] || [ "$number" -gt "${#TAGS[@]}" ]; then
            echo_log_warn "\033[33m编号不在范围内，请重新输入!\033[0m"
        else
            TARGET_TAG="${TAGS[$((number-1))]}"
            TARGET_PRE="${PRES[$((number-1))]}"
            break
        fi
    done
}

# confirm_install 展示目标版本并等待用户二次确认
confirm_install() {
    local action="安装" tip="" yn
    [ "$TARGET_TAG" = "$CURRENT_VERSION" ] && action="重新安装"
    [ "$TARGET_PRE" = "true" ] && tip="（预发布版本，可能不稳定）"
    while true; do
        read -rp "即将${action} wecom-auth-center ${TARGET_TAG}${tip}，是否继续? [y/N]: " yn
        case "$yn" in
            [Yy]*) break ;;
            [Nn]*|"")
                echo_log_info "已取消本次更新"
                exit 0
                ;;
            *) echo_log_warn "\033[33m请输入 y 或 n\033[0m" ;;
        esac
    done
}

# prepare_assets 按发布资产命名规则拼接目标版本的下载地址
prepare_assets() {
    local ver="${TARGET_TAG#v}"
    local base="https://github.com/$REPO/releases/download/$TARGET_TAG"
    BACKEND_ASSET="wecom-auth-center_${ver}_linux_${ARCH}.tar.gz"
    BACKEND_URL="${GH_PROXY_PREFIX}${base}/${BACKEND_ASSET}"
    SUMS_URL="${GH_PROXY_PREFIX}${base}/SHA256SUMS"
}

# check_url 预检下载地址有效性，避免下载阶段才发现版本资产不存在
check_url() {
    local url=$1
    if command -v curl >/dev/null 2>&1; then
        curl --head --silent --fail --connect-timeout 5 -L "$url" > /dev/null
    else
        wget --spider -q --timeout=5 --tries=1 "$url"
    fi
    [ $? -ne 0 ] && echo_log_error "下载地址无效，请检查该版本的发布资产是否存在: $url"
}

# download_file 优先使用 curl 下载文件，缺失时回退 wget
download_file() {
    local url=$1 dest=$2
    if command -v curl >/dev/null 2>&1; then
        curl -fL --connect-timeout 10 --retry 3 --retry-delay 2 --progress-bar -o "$dest" "$url"
    else
        wget -q --show-progress -O "$dest" "$url"
    fi
    [ $? -ne 0 ] && echo_log_error "下载失败: $url"
}

# download_packages 预检并下载后端安装包与校验和文件
download_packages() {
    echo ""
    echo_log_info "\033[32m开始下载 wecom-auth-center ${TARGET_TAG} 安装包...\033[0m"
    check_url "$BACKEND_URL"
    echo_log_info "下载安装包 $BACKEND_ASSET"
    download_file "$BACKEND_URL" "$TMP_DIR/$BACKEND_ASSET"
    echo_log_info "下载校验和文件 SHA256SUMS"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL --connect-timeout 10 --max-time 60 -o "$TMP_DIR/SHA256SUMS" "$SUMS_URL" || rm -f "$TMP_DIR/SHA256SUMS"
    else
        wget -q -T 10 -O "$TMP_DIR/SHA256SUMS" "$SUMS_URL" || rm -f "$TMP_DIR/SHA256SUMS"
    fi
    [ -s "$TMP_DIR/SHA256SUMS" ] || echo_log_warn "校验和文件不可用，将跳过完整性校验"
}

# verify_packages 基于 SHA256SUMS 校验已下载安装包的完整性
verify_packages() {
    [ -s "$TMP_DIR/SHA256SUMS" ] || return 0
    command -v sha256sum >/dev/null 2>&1 || { echo_log_warn "系统缺少 sha256sum，跳过完整性校验"; return 0; }
    grep "${BACKEND_ASSET}\$" "$TMP_DIR/SHA256SUMS" | (cd "$TMP_DIR" && sha256sum -c -) \
        || echo_log_error "安装包完整性校验失败: $BACKEND_ASSET"
    echo_log_info "安装包完整性校验通过"
}

# extract_packages 解压安装包到临时目录并校验关键文件存在
extract_packages() {
    echo_log_info "解压安装包"
    mkdir -p "$TMP_DIR/backend"
    tar -xzf "$TMP_DIR/$BACKEND_ASSET" -C "$TMP_DIR/backend" || echo_log_error "安装包解压失败"
    [ -f "$TMP_DIR/backend/linux_${ARCH}/wecom-auth-center" ] || echo_log_error "安装包内未找到 wecom-auth-center 二进制文件"
}

# stop_services 通过启停脚本停止 wecom-auth-center 服务
stop_services() {
    echo ""
    echo_log_info "停止 wecom-auth-center 服务"
    "$RUN_SCRIPT" stop
}

# install_packages 备份并原子替换后端二进制，保留原有属主权限
install_packages() {
    echo ""
    echo_log_info "替换二进制文件"
    if [ -f "$INSTALL_DIR/wecom-auth-center" ]; then
        cp -f "$INSTALL_DIR/wecom-auth-center" "$INSTALL_DIR/wecom-auth-center.bak" || echo_log_error "备份二进制文件失败"
        chmod --reference="$INSTALL_DIR/wecom-auth-center.bak" "$TMP_DIR/backend/linux_${ARCH}/wecom-auth-center" 2>/dev/null
        chown --reference="$INSTALL_DIR/wecom-auth-center.bak" "$TMP_DIR/backend/linux_${ARCH}/wecom-auth-center" 2>/dev/null
    fi
    chmod +x "$TMP_DIR/backend/linux_${ARCH}/wecom-auth-center"
    cp -f "$TMP_DIR/backend/linux_${ARCH}/wecom-auth-center" "$INSTALL_DIR/wecom-auth-center.new" || echo_log_error "写入新二进制文件失败"
    mv -f "$INSTALL_DIR/wecom-auth-center.new" "$INSTALL_DIR/wecom-auth-center" || echo_log_error "替换二进制文件失败"
    echo_log_info "新版本文件已就绪"
}

# rollback 新版本启动异常时恢复旧版二进制并重启服务
rollback() {
    echo_log_warn "新版本启动异常，正在回滚旧版本..."
    if [ -f "$INSTALL_DIR/wecom-auth-center.bak" ]; then
        mv -f "$INSTALL_DIR/wecom-auth-center.bak" "$INSTALL_DIR/wecom-auth-center" && chmod +x "$INSTALL_DIR/wecom-auth-center"
    fi
    echo_log_info "使用旧版本重新启动服务"
    "$RUN_SCRIPT" start
    echo_log_error "已回滚到旧版本并重启，请检查 $INSTALL_DIR/app.log 排查原因"
}

# start_services 启动服务并做进程存活检查，失败时自动回滚
start_services() {
    echo ""
    echo_log_info "启动 wecom-auth-center 服务"
    "$RUN_SCRIPT" start
    sleep 3
    local pid ok=1
    pid=$(cat "$INSTALL_DIR/app.pid" 2>/dev/null)
    { [ -z "$pid" ] || ! kill -0 "$pid" 2>/dev/null; } && ok=0
    [ $ok -eq 0 ] && rollback
    rm -f "$INSTALL_DIR/wecom-auth-center.bak"
    local new_version
    new_version=$("$INSTALL_DIR/wecom-auth-center" -v 2>/dev/null | head -n1 | awk '{print $2}')
    echo_log_info "\033[33m更新完成，当前版本: ${new_version:-$TARGET_TAG}\033[0m"
}

# main 编排更新全流程：环境检查、版本抓取、交互选择、下载校验、停服替换与重启验证
main() {
    check_env
    detect_arch
    get_current_version
    TMP_DIR=$(mktemp -d /tmp/wecom-auth-center-update.XXXXXX)
    fetch_all_releases
    show_version_list
    select_version
    confirm_install
    prepare_assets
    download_packages
    verify_packages
    extract_packages
    stop_services
    install_packages
    start_services
}

main "$@"
