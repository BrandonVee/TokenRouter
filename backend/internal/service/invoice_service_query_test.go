//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/BrandonVee/TokenRouter/ent/invoicerequestitem"
	"github.com/BrandonVee/TokenRouter/internal/payment"
	"github.com/stretchr/testify/require"
)

// TestInvoiceForeignKeySelections 验证外键列查询不会错误追加实体主键。
func TestInvoiceForeignKeySelections(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, err := client.User.Create().SetEmail("invoice-query@example.com").SetPasswordHash("hash").SetUsername("invoice-query-user").Save(ctx)
	require.NoError(t, err)
	order, err := client.PaymentOrder.Create().
		SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
		SetAmount(50).SetPayAmount(50).SetFeeRate(0).SetRechargeCode("INVOICE-QUERY").
		SetOutTradeNo("invoice-query-order").SetPaymentType(payment.TypeAlipay).SetPaymentTradeNo("invoice-query-trade").
		SetOrderType(payment.OrderTypeBalance).SetStatus(OrderStatusCompleted).SetExpiresAt(time.Now().Add(time.Hour)).
		SetPaidAt(time.Now()).SetClientIP("127.0.0.1").SetSrcHost("api.example.com").Save(ctx)
	require.NoError(t, err)
	request, err := client.InvoiceRequest.Create().
		SetUserID(user.ID).SetRequestNo("INV-QUERY-1").SetStatus(InvoiceStatusIssued).SetCurrency("CNY").SetTotalAmount(50).
		SetInvoiceTitle("测试抬头").SetRecipientEmail(user.Email).SetAccountEmail(user.Email).Save(ctx)
	require.NoError(t, err)
	_, err = client.InvoiceRequestItem.Create().
		SetInvoiceRequestID(request.ID).SetPaymentOrderID(order.ID).SetOrderNo(order.OutTradeNo).SetOrderType(order.OrderType).
		SetCurrency("CNY").SetPayAmount(order.PayAmount).SetCreditedAmount(order.Amount).SetProductSnapshot(map[string]any{}).Save(ctx)
	require.NoError(t, err)

	invoiceService := &InvoiceService{entClient: client}
	eligible, _, err := invoiceService.ListEligibleOrders(ctx, user.ID, 1, 20)
	require.NoError(t, err)
	require.Empty(t, eligible)

	var requestIDs []int64
	err = client.InvoiceRequestItem.Query().Where(invoicerequestitem.PaymentOrderIDEQ(order.ID)).Select(invoicerequestitem.FieldInvoiceRequestID).Scan(ctx, &requestIDs)
	require.NoError(t, err)
	require.Equal(t, []int64{request.ID}, requestIDs)
	issued, err := (&PaymentService{entClient: client}).hasIssuedInvoiceOrder(ctx, order.ID)
	require.NoError(t, err)
	require.True(t, issued)
}
