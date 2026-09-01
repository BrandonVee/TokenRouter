#!/bin/bash
# =============================================================================
# Sub2API Docker Deployment Preparation Script
# =============================================================================
# This script prepares deployment files for TokenRouter:
#   - Downloads docker-compose.local.yml and .env.example
#   - Generates secure secrets (JWT_SECRET, TOTP_ENCRYPTION_KEY, POSTGRES_PASSWORD)
#   - Creates necessary data directories
#
# After running this script, you can start services with:
#   docker-compose up -d
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# GitHub raw content base URL
GITHUB_RAW_URL="https://raw.githubusercontent.com/BrandonVee/TokenRouter/main/deploy"

# Print colored message
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 生成随机密钥。
generate_secret() {
    openssl rand -hex 32
}

# 检查命令是否存在。
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# 下载到临时文件后再替换，避免网络中断留下半份部署文件。
download_file() {
    local url="$1"
    local target="$2"
    local temp_file

    temp_file=$(mktemp "${target}.tmp.XXXXXX")
    if command_exists curl; then
        curl -sSL "$url" -o "$temp_file"
    else
        wget -q "$url" -O "$temp_file"
    fi
    mv "$temp_file" "$target"
}

# 读取最后一个同名配置，不执行可能包含命令的 .env 文件。
read_env_value() {
    local key="$1"
    local file="$2"

    awk -F= -v expected_key="$key" '
        $1 == expected_key {
            sub(/^[^=]*=/, "")
            value = $0
        }
        END { print value }
    ' "$file"
}

# 缺失或为空时生成密钥，已有值在更新部署文件时保持不变。
ensure_env_secret() {
    local key="$1"
    local file="$2"
    local value

    value=$(read_env_value "$key" "$file")
    if [ -n "$value" ]; then
        return 0
    fi

    value=$(generate_secret)
    if grep -q "^${key}=" "$file"; then
        sed -i.bak "s/^${key}=.*/${key}=${value}/" "$file"
        rm -f "${file}.bak"
    else
        printf '%s=%s\n' "$key" "$value" >> "$file"
    fi
}

# 将新版本样例中新增的变量追加到现有 .env，不覆盖服务器上的自定义值。
merge_env_defaults() {
    local example_file="$1"
    local env_file="$2"
    local line
    local key

    while IFS= read -r line || [ -n "$line" ]; do
        if [[ "$line" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]]; then
            key=${line%%=*}
            if ! grep -q "^${key}=" "$env_file"; then
                printf '%s\n' "$line" >> "$env_file"
            fi
        fi
    done < "$example_file"
}

# Main installation function
main() {
    local mode="${1:-install}"
    local existing_env=false

    case "$mode" in
        install|update)
            ;;
        *)
            print_error "Usage: $0 [install|update]"
            exit 1
            ;;
    esac

    echo ""
    echo "=========================================="
    echo "  TokenRouter Deployment Preparation"
    echo "=========================================="
    echo ""

    # Check if openssl is available
    if ! command_exists openssl; then
        print_error "openssl is not installed. Please install openssl first."
        exit 1
    fi

    if [ -f ".env" ]; then
        existing_env=true
    elif [ "$mode" = "update" ]; then
        print_error "Cannot update: .env does not exist in the current directory."
        exit 1
    fi

    if [ "$existing_env" = true ]; then
        print_info "Existing deployment detected; preserving all current .env values."
        if [ -f "docker-compose.yml" ]; then
            cp docker-compose.yml docker-compose.yml.backup
            print_info "Previous docker-compose.yml saved as docker-compose.yml.backup"
        fi
    fi

    # 更新 Compose 文件；现有文件已在上方留有可恢复副本。
    print_info "Downloading docker-compose.yml..."
    if ! command_exists curl && ! command_exists wget; then
        print_error "Neither curl nor wget is installed. Please install one of them."
        exit 1
    fi
    download_file "${GITHUB_RAW_URL}/docker-compose.local.yml" docker-compose.yml
    print_success "Downloaded docker-compose.yml"

    # 始终刷新样例文件，现有 .env 只增补新变量。
    print_info "Downloading .env.example..."
    download_file "${GITHUB_RAW_URL}/.env.example" .env.example
    print_success "Downloaded .env.example"

    # 首装复制完整样例，更新只追加新版本引入的配置项。
    if [ "$existing_env" = true ]; then
        merge_env_defaults .env.example .env
    else
        cp .env.example .env
    fi

    print_info "Ensuring persistent secrets..."
    echo ""
    ensure_env_secret "JWT_SECRET" .env
    ensure_env_secret "TOTP_ENCRYPTION_KEY" .env
    ensure_env_secret "POSTGRES_PASSWORD" .env

    # 首次部署时保留凭据回显，方便管理员记录；更新时绝不回显已有密钥。
    if [ "$existing_env" = false ]; then
        JWT_SECRET=$(read_env_value JWT_SECRET .env)
        TOTP_ENCRYPTION_KEY=$(read_env_value TOTP_ENCRYPTION_KEY .env)
        POSTGRES_PASSWORD=$(read_env_value POSTGRES_PASSWORD .env)
    fi

    # Create data directories
    print_info "Creating data directories..."
    mkdir -p data postgres_data redis_data
    print_success "Created data directories"

    # Set secure permissions for .env file (readable/writable only by owner)
    chmod 600 .env
    echo ""

    # 首装输出仅显示本次生成的凭据；更新输出不包含任何敏感值。
    echo "=========================================="
    echo "  Preparation Complete!"
    echo "=========================================="
    echo ""
    if [ "$existing_env" = false ]; then
        echo "Generated secure credentials:"
        echo "  POSTGRES_PASSWORD:     ${POSTGRES_PASSWORD}"
        echo "  JWT_SECRET:            ${JWT_SECRET}"
        echo "  TOTP_ENCRYPTION_KEY:   ${TOTP_ENCRYPTION_KEY}"
        echo ""
        print_warning "These credentials are saved in .env. Keep this output private."
    else
        print_success "Persistent credentials were preserved in .env."
    fi
    print_warning "Back up .env securely; do not replace its encryption keys during upgrades."
    echo ""
    echo "Directory structure:"
    echo "  docker-compose.yml        - Docker Compose configuration"
    echo "  .env                      - Environment variables (generated secrets)"
    echo "  .env.example              - Example template (for reference)"
    echo "  data/                     - Application data (will be created on first run)"
    echo "  postgres_data/            - PostgreSQL data"
    echo "  redis_data/               - Redis data"
    echo ""
    echo "Next steps:"
    echo "  1. (Optional) Edit .env to customize configuration"
    echo "  2. Start services:"
    echo "     docker-compose up -d"
    echo ""
    echo "  3. View logs:"
    echo "     docker-compose logs -f sub2api"
    echo ""
    echo "  4. Access Web UI:"
    echo "     http://localhost:8080"
    echo ""
    print_info "If admin password is not set in .env, it will be auto-generated."
    print_info "Check logs for the generated admin password on first startup."
    echo ""
}

# 测试可只加载函数；正常执行脚本时始终进入主流程。
if [ "${TOKENROUTER_DEPLOY_LIB_ONLY:-false}" != "true" ]; then
    main "$@"
fi
