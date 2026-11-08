//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/internal/service"
	"github.com/stretchr/testify/require"
)

// 新 ORM 字段必须与 PostgreSQL 迁移一致；旧订单默认没有赠金。
func TestPaymentOrderBonusAmountRoundTrip(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	user := mustCreateUser(t, tx.Client(), &service.User{Email: "bonus-roundtrip@example.com", Username: "bonus-user", PasswordHash: "hash"})
	for _, bonus := range []float64{0, 20} {
		create := tx.Client().PaymentOrder.Create().
			SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
			SetAmount(100 + bonus).SetPayAmount(104.7).SetFeeFixed(2.5).SetFeeRate(2.2).SetFeeRateAmount(2.2).SetFeeAmount(4.7).
			SetRechargeCode("").SetPaymentType("alipay").SetPaymentTradeNo("").
			SetExpiresAt(time.Now().Add(time.Hour)).SetClientIP("127.0.0.1").SetSrcHost("example.com")
		if bonus > 0 {
			create.SetBonusAmount(bonus)
		}
		order, err := create.Save(ctx)
		require.NoError(t, err)
		loaded, err := tx.Client().PaymentOrder.Get(ctx, order.ID)
		require.NoError(t, err)
		require.Equal(t, bonus, loaded.BonusAmount)
		require.Equal(t, 100+bonus, loaded.Amount)
		require.Equal(t, 104.7, loaded.PayAmount)
		require.Equal(t, 4.7, loaded.FeeAmount)
	}
}
