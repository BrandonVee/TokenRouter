#!/bin/bash

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/../.." && pwd)
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

fail() {
    printf 'deployment secrets test failed: %s\n' "$1" >&2
    exit 1
}

assert_env_value() {
    local file="$1"
    local key="$2"
    local expected="$3"
    local actual

    actual=$(awk -F= -v expected_key="$key" '$1 == expected_key { sub(/^[^=]*=/, ""); value = $0 } END { print value }' "$file")
    [ "$actual" = "$expected" ] || fail "$key was changed unexpectedly"
}

# Docker 更新必须保留旧值，仅补充样例中新出现的变量。
TOKENROUTER_DEPLOY_LIB_ONLY=true source "$ROOT_DIR/deploy/docker-deploy.sh"
printf '%s\n' \
    'JWT_SECRET=existing-jwt' \
    'TOTP_ENCRYPTION_KEY=existing-totp' \
    'POSTGRES_PASSWORD=existing-postgres' \
    'CUSTOM_VALUE=server-value' > "$TEMP_DIR/.env"
printf '%s\n' \
    'JWT_SECRET=' \
    'TOTP_ENCRYPTION_KEY=' \
    'POSTGRES_PASSWORD=' \
    'CUSTOM_VALUE=example-value' \
    'NEW_SETTING=example-default' > "$TEMP_DIR/.env.example"

merge_env_defaults "$TEMP_DIR/.env.example" "$TEMP_DIR/.env"
ensure_env_secret JWT_SECRET "$TEMP_DIR/.env"
ensure_env_secret TOTP_ENCRYPTION_KEY "$TEMP_DIR/.env"
ensure_env_secret POSTGRES_PASSWORD "$TEMP_DIR/.env"

assert_env_value "$TEMP_DIR/.env" JWT_SECRET existing-jwt
assert_env_value "$TEMP_DIR/.env" TOTP_ENCRYPTION_KEY existing-totp
assert_env_value "$TEMP_DIR/.env" POSTGRES_PASSWORD existing-postgres
assert_env_value "$TEMP_DIR/.env" CUSTOM_VALUE server-value
assert_env_value "$TEMP_DIR/.env" NEW_SETTING example-default

# 空密钥只生成一次，重复执行升级辅助函数时必须保持相同值。
printf 'TOTP_ENCRYPTION_KEY=\n' > "$TEMP_DIR/generated.env"
ensure_env_secret TOTP_ENCRYPTION_KEY "$TEMP_DIR/generated.env"
generated_key=$(read_env_value TOTP_ENCRYPTION_KEY "$TEMP_DIR/generated.env")
[[ "$generated_key" =~ ^[0-9a-f]{64}$ ]] || fail "generated TOTP key is not 32-byte hex"
ensure_env_secret TOTP_ENCRYPTION_KEY "$TEMP_DIR/generated.env"
assert_env_value "$TEMP_DIR/generated.env" TOTP_ENCRYPTION_KEY "$generated_key"

# systemd 安装器使用相同的幂等规则，并把持久环境文件挂入服务单元。
TOKENROUTER_DEPLOY_LIB_ONLY=true source "$ROOT_DIR/deploy/install.sh"
printf 'TOTP_ENCRYPTION_KEY=systemd-existing\n' > "$TEMP_DIR/systemd.env"
ensure_env_secret TOTP_ENCRYPTION_KEY "$TEMP_DIR/systemd.env"
assert_env_value "$TEMP_DIR/systemd.env" TOTP_ENCRYPTION_KEY systemd-existing
grep -Fq 'EnvironmentFile=-${ENV_FILE}' "$ROOT_DIR/deploy/install.sh" || fail "systemd unit does not load the persistent environment file"
grep -Fq 'ensure_service_environment_file' "$ROOT_DIR/deploy/install.sh" || fail "legacy systemd unit migration is missing"

printf 'deployment secrets test passed\n'
