package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/BrandonVee/TokenRouter/ent"
	"github.com/BrandonVee/TokenRouter/ent/securitysecret"
	"github.com/BrandonVee/TokenRouter/internal/config"
)

const (
	securitySecretKeyJWT            = "jwt_secret"
	securitySecretKeyTOTPEncryption = "totp_encryption_key"
	securitySecretReadRetryMax      = 5
	securitySecretReadRetryWait     = 10 * time.Millisecond
)

var readRandomBytes = rand.Read

func ensureBootstrapSecrets(ctx context.Context, client *ent.Client, cfg *config.Config) error {
	if client == nil {
		return fmt.Errorf("nil ent client")
	}
	if cfg == nil {
		return fmt.Errorf("nil config")
	}

	cfg.JWT.Secret = strings.TrimSpace(cfg.JWT.Secret)
	if cfg.JWT.Secret != "" {
		storedSecret, err := createSecuritySecretIfAbsent(ctx, client, securitySecretKeyJWT, cfg.JWT.Secret)
		if err != nil {
			return fmt.Errorf("persist jwt secret: %w", err)
		}
		if storedSecret != cfg.JWT.Secret {
			log.Println("Warning: configured JWT secret mismatches persisted value; using persisted secret for cross-instance consistency.")
		}
		cfg.JWT.Secret = storedSecret
	} else {
		secret, created, err := getOrCreateGeneratedSecuritySecret(ctx, client, securitySecretKeyJWT, 32)
		if err != nil {
			return fmt.Errorf("ensure jwt secret: %w", err)
		}
		cfg.JWT.Secret = secret

		if created {
			log.Println("Warning: JWT secret auto-generated and persisted to database. Consider rotating to a managed secret for production.")
		}
	}

	if err := ensureTOTPEncryptionKey(ctx, client, cfg); err != nil {
		return fmt.Errorf("ensure totp encryption key: %w", err)
	}
	return nil
}

// ensureTOTPEncryptionKey 将 TOTP 密钥持久化到数据库，保证缺省配置在重启和多实例间保持一致。
func ensureTOTPEncryptionKey(ctx context.Context, client *ent.Client, cfg *config.Config) error {
	configured := strings.TrimSpace(cfg.Totp.EncryptionKey)
	var (
		stored  string
		created bool
		err     error
	)

	if configured != "" {
		if err := validateTOTPEncryptionKey(configured); err != nil {
			return err
		}
		stored, err = createSecuritySecretIfAbsent(ctx, client, securitySecretKeyTOTPEncryption, configured)
		if err == nil && stored != configured {
			log.Println("Warning: configured TOTP encryption key mismatches persisted value; using persisted key for cross-instance consistency.")
		}
	} else {
		stored, created, err = getOrCreateGeneratedSecuritySecret(ctx, client, securitySecretKeyTOTPEncryption, 32)
	}
	if err != nil {
		return fmt.Errorf("persist key: %w", err)
	}
	if err := validateTOTPEncryptionKey(stored); err != nil {
		return fmt.Errorf("stored key: %w", err)
	}

	cfg.Totp.EncryptionKey = stored
	// 数据库记录已经提供跨重启稳定性，后续服务可以安全保存加密凭据。
	cfg.Totp.EncryptionKeyConfigured = true
	if created {
		log.Println("Warning: TOTP encryption key auto-generated and persisted to database. Keep security_secrets in backups; set TOTP_ENCRYPTION_KEY before first startup for external secret management.")
	}
	return nil
}

// validateTOTPEncryptionKey 校验 AES-256 所需的 32 字节十六进制密钥格式。
func validateTOTPEncryptionKey(value string) error {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("totp encryption key must be 64 hexadecimal characters: %w", err)
	}
	if len(decoded) != 32 {
		return fmt.Errorf("totp encryption key must be 32 bytes (64 hexadecimal characters), got %d bytes", len(decoded))
	}
	return nil
}

func getOrCreateGeneratedSecuritySecret(ctx context.Context, client *ent.Client, key string, byteLength int) (string, bool, error) {
	existing, err := client.SecuritySecret.Query().Where(securitysecret.KeyEQ(key)).Only(ctx)
	if err == nil {
		value := strings.TrimSpace(existing.Value)
		if len([]byte(value)) < 32 {
			return "", false, fmt.Errorf("stored secret %q must be at least 32 bytes", key)
		}
		return value, false, nil
	}
	if !ent.IsNotFound(err) {
		return "", false, err
	}

	generated, err := generateHexSecret(byteLength)
	if err != nil {
		return "", false, err
	}

	if err := client.SecuritySecret.Create().
		SetKey(key).
		SetValue(generated).
		OnConflictColumns(securitysecret.FieldKey).
		DoNothing().
		Exec(ctx); err != nil {
		if !isSQLNoRowsError(err) {
			return "", false, err
		}
	}

	stored, err := querySecuritySecretWithRetry(ctx, client, key)
	if err != nil {
		return "", false, err
	}
	value := strings.TrimSpace(stored.Value)
	if len([]byte(value)) < 32 {
		return "", false, fmt.Errorf("stored secret %q must be at least 32 bytes", key)
	}
	return value, value == generated, nil
}

func createSecuritySecretIfAbsent(ctx context.Context, client *ent.Client, key, value string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]byte(value)) < 32 {
		return "", fmt.Errorf("secret %q must be at least 32 bytes", key)
	}

	if err := client.SecuritySecret.Create().
		SetKey(key).
		SetValue(value).
		OnConflictColumns(securitysecret.FieldKey).
		DoNothing().
		Exec(ctx); err != nil {
		if !isSQLNoRowsError(err) {
			return "", err
		}
	}

	stored, err := querySecuritySecretWithRetry(ctx, client, key)
	if err != nil {
		return "", err
	}
	storedValue := strings.TrimSpace(stored.Value)
	if len([]byte(storedValue)) < 32 {
		return "", fmt.Errorf("stored secret %q must be at least 32 bytes", key)
	}
	return storedValue, nil
}

func querySecuritySecretWithRetry(ctx context.Context, client *ent.Client, key string) (*ent.SecuritySecret, error) {
	var lastErr error
	for attempt := 0; attempt <= securitySecretReadRetryMax; attempt++ {
		stored, err := client.SecuritySecret.Query().Where(securitysecret.KeyEQ(key)).Only(ctx)
		if err == nil {
			return stored, nil
		}
		if !isSecretNotFoundError(err) {
			return nil, err
		}
		lastErr = err
		if attempt == securitySecretReadRetryMax {
			break
		}

		timer := time.NewTimer(securitySecretReadRetryWait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, lastErr
}

func isSecretNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return ent.IsNotFound(err) || isSQLNoRowsError(err)
}

func isSQLNoRowsError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows in result set")
}

func generateHexSecret(byteLength int) (string, error) {
	if byteLength <= 0 {
		byteLength = 32
	}
	buf := make([]byte, byteLength)
	if _, err := readRandomBytes(buf); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
