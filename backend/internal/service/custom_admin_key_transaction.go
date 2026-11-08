package service

import (
	"context"
	dbent "github.com/BrandonVee/TokenRouter/ent"
	"github.com/BrandonVee/TokenRouter/internal/pkg/adminaccess"
)

func lockScopedAdminAccount(ctx context.Context, tx *dbent.Tx, id int64) error {
	_, err := tx.Client().ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('scoped_admin_account:' || $1::text, 0))`, id)
	return err
}

// AfterAccountCommit 将不参与回滚的缓存和上游副作用延迟到事务提交成功后。
// 普通管理员没有外层事务时立即执行，保留原有行为。
func AfterAccountCommit(ctx context.Context, run func(context.Context)) {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		tx.OnCommit(func(next dbent.Committer) dbent.Committer {
			return dbent.CommitFunc(func(commitCtx context.Context, tx *dbent.Tx) error {
				if err := next.Commit(commitCtx, tx); err != nil {
					return err
				}
				run(dbent.NewTxContext(ctx, nil))
				return nil
			})
		})
	} else {
		run(ctx)
	}
}

// LockScopedAdminPrincipal 在资源写事务内重验认证版本，并持锁至提交，避免撤销或收权后仍落库。
func LockScopedAdminPrincipal(ctx context.Context, tx *dbent.Tx) error {
	p := adminaccess.FromContext(ctx)
	if p == nil || p.KeyVersion.IsZero() {
		return nil
	}
	rows, err := tx.Client().QueryContext(ctx, `SELECT k.id FROM scoped_admin_api_keys k JOIN users u ON u.id=k.created_by
 WHERE k.id=$1 AND k.updated_at=$2 AND k.created_by=$3 AND k.enabled AND k.revoked_at IS NULL AND k.deleted_at IS NULL
 AND (k.expires_at IS NULL OR k.expires_at>NOW()) AND u.role='admin' AND u.status='active' AND u.deleted_at IS NULL
 FOR SHARE OF k,u`, p.KeyID, p.KeyVersion, p.CreatorID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return ErrScopedAdminKeyInvalid
	}
	return rows.Err()
}
