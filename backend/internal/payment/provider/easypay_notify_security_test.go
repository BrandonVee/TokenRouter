package provider

// 易支付回调安全回归测试：防"下单签名复用为伪造支付成功回调"攻击。
// 攻击原理：签名基础串按 key 排序后直接 k=v& 拼接（值不转义），且 popup 模式把
// 下单签名暴露在支付 URL 里；若 return_url 的 query 携带 trade_status，伪造回调时
// 让其升为顶层参数，重排拼接后与下单签名串逐字节一致，验签即通过。
// 修复后由两层设防：CanonicalizeReturnURL 剥离用户 query（注入源关闭），
// VerifyNotification 参数白名单（即使拿到旧签名也无法携带白名单外参数）。

import (
	"context"
	"net/url"
	"testing"

	"github.com/BrandonVee/TokenRouter/internal/payment"
)

func newEasyPayForSecurityTest() *EasyPay {
	return &EasyPay{config: map[string]string{
		"pid":       "1000",
		"pkey":      "MERCHANT_SECRET_KEY",
		"apiBase":   "https://pay.example.com",
		"notifyUrl": "https://site.example.com/api/v1/payment/webhook/easypay",
		"returnUrl": "https://site.example.com/payment/result",
	}}
}

// PoC 攻击载荷必须被拒：return_url（白名单外）携带走私意图，直接拒绝。
func TestEasyPayVerifyNotificationRejectsForgedCallback(t *testing.T) {
	t.Parallel()

	e := newEasyPayForSecurityTest()
	// 攻击者从自己的支付 URL 里拿到的下单签名（其 return_url 值末尾藏着 trade_status）
	poisonedReturnURL := "https://site.example.com/payment/result?order_id=99&out_trade_no=ORDER123&status=success&trade_status=TRADE_SUCCESS"
	createParams := map[string]string{
		"pid": "1000", "type": "alipay", "out_trade_no": "ORDER123",
		"notify_url": e.config["notifyUrl"], "return_url": poisonedReturnURL,
		"name": "balance recharge", "money": "650.00",
	}
	sign := easyPaySign(createParams, e.config["pkey"])

	// 伪造回调：return_url 只编码到 status=success，&trade_status 以裸 & 升为顶层参数
	prefix := "https://site.example.com/payment/result?order_id=99&out_trade_no=ORDER123&status=success"
	cb := url.Values{}
	cb.Set("pid", "1000")
	cb.Set("type", "alipay")
	cb.Set("out_trade_no", "ORDER123")
	cb.Set("notify_url", e.config["notifyUrl"])
	cb.Set("name", "balance recharge")
	cb.Set("money", "650.00")
	cb.Set("return_url", prefix)
	rawCallback := cb.Encode() + "&trade_status=TRADE_SUCCESS&sign=" + sign + "&sign_type=MD5"

	n, err := e.VerifyNotification(context.Background(), rawCallback, nil)
	if err == nil {
		t.Fatalf("forged callback must be rejected, got notification: status=%q amount=%v", n.Status, n.Amount)
	}
}

// 完整回放支付 URL 必须被拒：popup 支付 URL 的 query 含白名单外的 return_url。
func TestEasyPayVerifyNotificationRejectsPaymentURLReplay(t *testing.T) {
	t.Parallel()

	e := newEasyPayForSecurityTest()
	returnURL := "https://site.example.com/payment/result?order_id=99&out_trade_no=ORDER123&status=success&trade_status=TRADE_SUCCESS"
	createParams := map[string]string{
		"pid": "1000", "type": "alipay", "out_trade_no": "ORDER123",
		"notify_url": e.config["notifyUrl"], "return_url": returnURL,
		"name": "balance recharge", "money": "650.00",
	}
	sign := easyPaySign(createParams, e.config["pkey"])

	// 把 submit.php 支付 URL 的完整 query 原样重放到 webhook（签名本身有效）
	cb := url.Values{}
	for k, v := range createParams {
		cb.Set(k, v)
	}
	cb.Set("sign", sign)
	cb.Set("sign_type", signTypeMD5)

	n, err := e.VerifyNotification(context.Background(), cb.Encode(), nil)
	if err == nil {
		t.Fatalf("payment URL replay must be rejected, got notification: status=%q", n.Status)
	}
}

// 真实回调（白名单内参数 + 有效签名）照常通过。
func TestEasyPayVerifyNotificationAcceptsGenuineCallback(t *testing.T) {
	t.Parallel()

	e := newEasyPayForSecurityTest()
	params := map[string]string{
		"pid": "1000", "trade_no": "2026100712345", "out_trade_no": "ORDER123",
		"type": "alipay", "name": "balance recharge", "money": "650.00",
		"trade_status": tradeStatusSuccess, "param": "",
	}
	sign := easyPaySign(params, e.config["pkey"])

	cb := url.Values{}
	for k, v := range params {
		cb.Set(k, v)
	}
	cb.Set("sign", sign)
	cb.Set("sign_type", signTypeMD5)

	n, err := e.VerifyNotification(context.Background(), cb.Encode(), nil)
	if err != nil {
		t.Fatalf("genuine callback should pass: %v", err)
	}
	if n.Status != payment.ProviderStatusSuccess {
		t.Fatalf("status = %q, want success", n.Status)
	}
	if n.Amount != 650.00 {
		t.Fatalf("amount = %v, want 650", n.Amount)
	}
	if n.OrderID != "ORDER123" || n.TradeNo != "2026100712345" {
		t.Fatalf("order = %q, tradeNo = %q", n.OrderID, n.TradeNo)
	}
}

// 空值未知参数也必须被拒：整类"拼接走私"设防不豁免空值。
func TestEasyPayVerifyNotificationRejectsUnknownEmptyParam(t *testing.T) {
	t.Parallel()

	e := newEasyPayForSecurityTest()
	params := map[string]string{
		"pid": "1000", "trade_no": "2026100712345", "out_trade_no": "ORDER123",
		"type": "alipay", "name": "balance recharge", "money": "650.00",
		"trade_status": tradeStatusSuccess,
	}
	sign := easyPaySign(params, e.config["pkey"])

	cb := url.Values{}
	for k, v := range params {
		cb.Set(k, v)
	}
	cb.Set("device", "") // 白名单外的空值参数
	cb.Set("sign", sign)
	cb.Set("sign_type", signTypeMD5)

	if _, err := e.VerifyNotification(context.Background(), cb.Encode(), nil); err == nil {
		t.Fatal("unknown empty param must be rejected")
	}
}
