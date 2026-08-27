package service

import (
	"testing"

	dbent "github.com/BrandonVee/TokenRouter/ent"
	"github.com/BrandonVee/TokenRouter/internal/domain"
	"github.com/BrandonVee/TokenRouter/internal/payment"
	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
)

// TestValidateInvoiceOrdersUsesPayAmount 确保合并开票仅以用户实付金额累计。
func TestValidateInvoiceOrdersUsesPayAmount(t *testing.T) {
	dailyLimit := 12.5
	orders := []*dbent.PaymentOrder{
		{
			Status:    OrderStatusCompleted,
			OrderType: payment.OrderTypeBalance,
			Amount:    100,
			PayAmount: 88.5,
			ProviderSnapshot: map[string]interface{}{
				"currency": "USD",
			},
		},
		{
			Status:    OrderStatusCompleted,
			OrderType: payment.OrderTypeSubscription,
			Amount:    999,
			PayAmount: 11.5,
			ProviderSnapshot: map[string]interface{}{
				"currency": "USD",
			},
			PlanSnapshot: domain.SubscriptionPlanSnapshot{Name: "Pro", ValidityDays: 30, DailyLimitUSD: &dailyLimit},
		},
	}

	currency, total, err := validateInvoiceOrders(orders)
	if err != nil {
		t.Fatalf("validate invoice orders: %v", err)
	}
	if currency != "USD" || total != 100 {
		t.Fatalf("currency=%q total=%v, want USD and 100", currency, total)
	}
	snapshot := invoiceOrderSnapshot(orders[1])
	if snapshot["plan_name"] != "Pro" || snapshot["daily_limit_usd"] != &dailyLimit {
		t.Fatalf("subscription snapshot=%#v, want plan and quota fields", snapshot)
	}
}

// TestValidateInvoiceOrdersRejectsInvalidSources 验证状态、币种和订单类型边界。
func TestValidateInvoiceOrdersRejectsInvalidSources(t *testing.T) {
	completedBalance := &dbent.PaymentOrder{Status: OrderStatusCompleted, OrderType: payment.OrderTypeBalance, PayAmount: 10}
	completedSubscription := &dbent.PaymentOrder{Status: OrderStatusCompleted, OrderType: payment.OrderTypeSubscription, PayAmount: 10, ProviderSnapshot: map[string]interface{}{"currency": "USD"}}

	_, _, err := validateInvoiceOrders([]*dbent.PaymentOrder{completedBalance, completedSubscription})
	if infraerrors.Reason(err) != "INVOICE_CURRENCY_MISMATCH" {
		t.Fatalf("currency mismatch reason=%q, err=%v", infraerrors.Reason(err), err)
	}
	_, _, err = validateInvoiceOrders([]*dbent.PaymentOrder{{Status: "PENDING", OrderType: payment.OrderTypeBalance, PayAmount: 10}})
	if infraerrors.Reason(err) != "INVOICE_ORDER_INELIGIBLE" {
		t.Fatalf("pending order reason=%q, err=%v", infraerrors.Reason(err), err)
	}
	_, _, err = validateInvoiceOrders([]*dbent.PaymentOrder{{Status: OrderStatusCompleted, OrderType: "other", PayAmount: 10}})
	if infraerrors.Reason(err) != "INVOICE_ORDER_INELIGIBLE" {
		t.Fatalf("unsupported order reason=%q, err=%v", infraerrors.Reason(err), err)
	}
}

// TestNormalizeInvoiceAttachmentType 拒绝仅靠声明类型伪装的非发票文件。
func TestNormalizeInvoiceAttachmentType(t *testing.T) {
	testCases := []struct {
		name        string
		data        []byte
		contentType string
		ok          bool
	}{
		{name: "pdf", data: []byte("%PDF-1.7\ncontent"), contentType: "application/pdf", ok: true},
		{name: "png", data: []byte{'\x89', 'P', 'N', 'G', '\r', '\n', '\x1a', '\n'}, contentType: "image/png", ok: true},
		{name: "jpeg", data: []byte{'\xff', '\xd8', '\xff', '\xe0', 0, 0, 0, 0}, contentType: "image/jpeg", ok: true},
		{name: "spoofed type", data: []byte("not a document"), contentType: "application/pdf", ok: false},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, ok := normalizeInvoiceAttachmentType(testCase.data, testCase.contentType)
			if ok != testCase.ok {
				t.Fatalf("ok=%v, want %v", ok, testCase.ok)
			}
		})
	}
}

// TestNormalizeInvoiceType 验证个人默认、企业识别与非法输入的处理。
func TestNormalizeInvoiceType(t *testing.T) {
	if got := normalizeInvoiceType(""); got != InvoiceTypePersonal {
		t.Fatalf("empty type=%q, want %q", got, InvoiceTypePersonal)
	}
	if got := normalizeInvoiceType(" enterprise "); got != InvoiceTypeEnterprise {
		t.Fatalf("enterprise type=%q, want %q", got, InvoiceTypeEnterprise)
	}
	if got := normalizeInvoiceType("other"); got != "" {
		t.Fatalf("invalid type=%q, want empty", got)
	}
}
