package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dbent "github.com/BrandonVee/TokenRouter/ent"
	"github.com/BrandonVee/TokenRouter/ent/invoiceattachment"
	"github.com/BrandonVee/TokenRouter/ent/invoicedelivery"
	"github.com/BrandonVee/TokenRouter/ent/invoicerequest"
	"github.com/BrandonVee/TokenRouter/ent/invoicerequestitem"
	"github.com/BrandonVee/TokenRouter/ent/paymentorder"
	"github.com/BrandonVee/TokenRouter/internal/payment"
	infraerrors "github.com/BrandonVee/TokenRouter/internal/pkg/errors"
)

const (
	InvoiceStatusSubmitted = "SUBMITTED"
	InvoiceStatusRejected  = "REJECTED"
	InvoiceStatusApproved  = "APPROVED"
	InvoiceStatusIssued    = "ISSUED"
	InvoiceStatusSent      = "SENT"

	InvoiceTypePersonal   = "PERSONAL"
	InvoiceTypeEnterprise = "ENTERPRISE"

	invoiceAttachmentMaxBytes       int64 = 10 * 1024 * 1024
	invoiceEmailAttachmentsMaxBytes int64 = 20 * 1024 * 1024
	invoiceAttachmentsMaxPerRequest       = 10
)

// InvoiceCreateInput 是用户提交发票申请时冻结的资料。
type InvoiceCreateInput struct {
	OrderIDs       []int64
	InvoiceType    string
	InvoiceTitle   string
	TaxID          string
	BankName       string
	BankAccount    string
	RecipientEmail string
	Remark         string
}

// InvoiceIssueInput 是管理员录入的开票信息。
type InvoiceIssueInput struct {
	InvoiceNumber string
	IssuedAt      time.Time
	Remark        string
}

// InvoiceService 管理支付订单的人工开票生命周期。
type InvoiceService struct {
	entClient    *dbent.Client
	emailService *EmailService
	notification *NotificationEmailService
	fileStorage  *FileStorageService
}

// ProvideInvoiceService 组装发票服务，并通过统一文件存储解析附件后端。
func ProvideInvoiceService(entClient *dbent.Client, emailService *EmailService, notification *NotificationEmailService, fileStorage *FileStorageService) *InvoiceService {
	return &InvoiceService{
		entClient:    entClient,
		emailService: emailService,
		notification: notification,
		fileStorage:  fileStorage,
	}
}

// Create 为当前用户创建一份已提交的发票申请。
func (s *InvoiceService) Create(ctx context.Context, userID int64, input InvoiceCreateInput) (*dbent.InvoiceRequest, error) {
	if s == nil || s.entClient == nil {
		return nil, fmt.Errorf("invoice service is not configured")
	}
	input.InvoiceTitle = strings.TrimSpace(input.InvoiceTitle)
	input.InvoiceType = normalizeInvoiceType(input.InvoiceType)
	input.TaxID = strings.TrimSpace(input.TaxID)
	input.BankName = strings.TrimSpace(input.BankName)
	input.BankAccount = strings.TrimSpace(input.BankAccount)
	input.RecipientEmail = strings.TrimSpace(input.RecipientEmail)
	input.Remark = strings.TrimSpace(input.Remark)
	if input.InvoiceTitle == "" {
		return nil, infraerrors.BadRequest("INVOICE_TITLE_REQUIRED", "invoice title is required")
	}
	if input.InvoiceType == "" {
		return nil, infraerrors.BadRequest("INVOICE_TYPE_INVALID", "invoice type must be PERSONAL or ENTERPRISE")
	}
	if input.InvoiceType == InvoiceTypeEnterprise {
		if input.TaxID == "" {
			return nil, infraerrors.BadRequest("INVOICE_TAX_ID_REQUIRED", "enterprise invoice tax ID is required")
		}
	}
	if input.InvoiceType == InvoiceTypePersonal {
		input.TaxID = ""
		input.BankName = ""
		input.BankAccount = ""
	}
	orderIDs := uniquePositiveInvoiceIDs(input.OrderIDs)
	if len(orderIDs) == 0 {
		return nil, infraerrors.BadRequest("INVOICE_ORDERS_REQUIRED", "select at least one completed order")
	}

	user, err := s.entClient.User.Get(ctx, userID)
	if err != nil {
		return nil, infraerrors.NotFound("USER_NOT_FOUND", "user not found")
	}
	accountEmail := strings.TrimSpace(user.Email)
	recipient := input.RecipientEmail
	if recipient == "" {
		recipient = accountEmail
	}
	if _, err := mail.ParseAddress(recipient); err != nil {
		return nil, infraerrors.BadRequest("INVALID_INVOICE_EMAIL", "recipient email is invalid")
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("start invoice transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	orders, err := tx.PaymentOrder.Query().Where(paymentorder.IDIn(orderIDs...), paymentorder.UserIDEQ(userID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query invoice orders: %w", err)
	}
	if len(orders) != len(orderIDs) {
		return nil, infraerrors.BadRequest("INVOICE_ORDER_NOT_FOUND", "some selected orders do not belong to the current user")
	}
	occupied, err := tx.InvoiceRequestItem.Query().Where(invoicerequestitem.PaymentOrderIDIn(orderIDs...), invoicerequestitem.ActiveEQ(true)).Exist(ctx)
	if err != nil {
		return nil, fmt.Errorf("check invoice order occupancy: %w", err)
	}
	if occupied {
		return nil, infraerrors.Conflict("INVOICE_ORDER_OCCUPIED", "some selected orders already have an invoice application")
	}

	currency, total, err := validateInvoiceOrders(orders)
	if err != nil {
		return nil, err
	}
	requestNo, err := newInvoiceRequestNo()
	if err != nil {
		return nil, fmt.Errorf("generate invoice request number: %w", err)
	}
	create := tx.InvoiceRequest.Create().
		SetUserID(userID).
		SetRequestNo(requestNo).
		SetStatus(InvoiceStatusSubmitted).
		SetCurrency(currency).
		SetTotalAmount(total).
		SetInvoiceType(input.InvoiceType).
		SetInvoiceTitle(input.InvoiceTitle).
		SetRecipientEmail(recipient).
		SetAccountEmail(accountEmail)
	if input.TaxID != "" {
		create.SetTaxID(input.TaxID)
	}
	if input.BankName != "" {
		create.SetBankName(input.BankName)
	}
	if input.BankAccount != "" {
		create.SetBankAccount(input.BankAccount)
	}
	if input.Remark != "" {
		create.SetRemark(input.Remark)
	}
	request, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create invoice request: %w", err)
	}
	for _, order := range orders {
		item := tx.InvoiceRequestItem.Create().
			SetInvoiceRequestID(request.ID).
			SetPaymentOrderID(order.ID).
			SetOrderNo(order.OutTradeNo).
			SetOrderType(order.OrderType).
			SetCurrency(currency).
			SetPayAmount(order.PayAmount).
			SetCreditedAmount(order.Amount).
			SetProductSnapshot(invoiceOrderSnapshot(order)).
			SetNillablePaidAt(order.PaidAt)
		if code := strings.TrimSpace(order.RechargeCode); code != "" {
			item.SetRechargeCode(code)
		}
		if _, err := item.Save(ctx); err != nil {
			if dbent.IsConstraintError(err) {
				return nil, infraerrors.Conflict("INVOICE_ORDER_OCCUPIED", "some selected orders already have an invoice application")
			}
			return nil, fmt.Errorf("create invoice item: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit invoice request: %w", err)
	}
	return request, nil
}

// ListEligibleOrders 返回尚未被申请占用的可开票订单。
func (s *InvoiceService) ListEligibleOrders(ctx context.Context, userID int64, page, pageSize int) ([]*dbent.PaymentOrder, int, error) {
	// 通过实体查询读取订单 ID，避免 Ent 在 Select + Scan 时额外带出主键导致列数不匹配。
	occupiedItems, err := s.entClient.InvoiceRequestItem.Query().Where(invoicerequestitem.ActiveEQ(true)).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("query occupied invoice orders: %w", err)
	}
	activeOrderIDs := make([]int64, 0, len(occupiedItems))
	for _, item := range occupiedItems {
		activeOrderIDs = append(activeOrderIDs, item.PaymentOrderID)
	}
	query := s.entClient.PaymentOrder.Query().Where(
		paymentorder.UserIDEQ(userID),
		paymentorder.StatusEQ(OrderStatusCompleted),
		paymentorder.OrderTypeIn(payment.OrderTypeBalance, payment.OrderTypeSubscription),
		paymentorder.PayAmountGT(0),
	)
	if len(activeOrderIDs) > 0 {
		query = query.Where(paymentorder.IDNotIn(activeOrderIDs...))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count eligible invoice orders: %w", err)
	}
	pageSize, page = applyPagination(pageSize, page)
	orders, err := query.Order(dbent.Desc(paymentorder.FieldCompletedAt)).Limit(pageSize).Offset((page - 1) * pageSize).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("query eligible invoice orders: %w", err)
	}
	result := make([]*dbent.PaymentOrder, 0, len(orders))
	for _, order := range orders {
		if _, _, err := validateInvoiceOrders([]*dbent.PaymentOrder{order}); err == nil {
			result = append(result, order)
		}
	}
	return result, total, nil
}

// ListUserRequests 返回当前用户的发票申请。
func (s *InvoiceService) ListUserRequests(ctx context.Context, userID int64, page, pageSize int) ([]*dbent.InvoiceRequest, int, error) {
	query := s.entClient.InvoiceRequest.Query().Where(invoicerequest.UserIDEQ(userID))
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count user invoice requests: %w", err)
	}
	pageSize, page = applyPagination(pageSize, page)
	items, err := query.Order(dbent.Desc(invoicerequest.FieldCreatedAt)).Limit(pageSize).Offset((page - 1) * pageSize).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list user invoice requests: %w", err)
	}
	return items, total, nil
}

// ListAdminRequests 返回管理员可筛选的发票申请。
func (s *InvoiceService) ListAdminRequests(ctx context.Context, status, keyword string, page, pageSize int) ([]*dbent.InvoiceRequest, int, error) {
	query := s.entClient.InvoiceRequest.Query()
	if status = strings.TrimSpace(status); status != "" {
		query = query.Where(invoicerequest.StatusEQ(status))
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		if userID, parseErr := strconv.ParseInt(keyword, 10, 64); parseErr == nil && userID > 0 {
			query = query.Where(invoicerequest.UserIDEQ(userID))
		} else {
			query = query.Where(invoicerequest.AccountEmailContainsFold(keyword))
		}
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count invoice requests: %w", err)
	}
	pageSize, page = applyPagination(pageSize, page)
	items, err := query.Order(dbent.Desc(invoicerequest.FieldCreatedAt)).Limit(pageSize).Offset((page - 1) * pageSize).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list invoice requests: %w", err)
	}
	return items, total, nil
}

// GetUserRequest 返回属于当前用户的发票申请。
func (s *InvoiceService) GetUserRequest(ctx context.Context, requestID, userID int64) (*dbent.InvoiceRequest, error) {
	request, err := s.entClient.InvoiceRequest.Get(ctx, requestID)
	if err != nil {
		return nil, infraerrors.NotFound("INVOICE_REQUEST_NOT_FOUND", "invoice request not found")
	}
	if request.UserID != userID {
		return nil, infraerrors.Forbidden("INVOICE_REQUEST_FORBIDDEN", "no permission for this invoice request")
	}
	return request, nil
}

// GetAdminRequest 返回管理员可读取的发票申请。
func (s *InvoiceService) GetAdminRequest(ctx context.Context, requestID int64) (*dbent.InvoiceRequest, error) {
	request, err := s.entClient.InvoiceRequest.Get(ctx, requestID)
	if err != nil {
		return nil, infraerrors.NotFound("INVOICE_REQUEST_NOT_FOUND", "invoice request not found")
	}
	return request, nil
}

// GetRequestItems 返回被冻结的订单明细。
func (s *InvoiceService) GetRequestItems(ctx context.Context, requestID int64) ([]*dbent.InvoiceRequestItem, error) {
	return s.entClient.InvoiceRequestItem.Query().Where(invoicerequestitem.InvoiceRequestIDEQ(requestID)).Order(dbent.Asc(invoicerequestitem.FieldID)).All(ctx)
}

// GetAttachments 返回当前申请的附件元数据。
func (s *InvoiceService) GetAttachments(ctx context.Context, requestID int64) ([]*dbent.InvoiceAttachment, error) {
	return s.entClient.InvoiceAttachment.Query().Where(invoiceattachment.InvoiceRequestIDEQ(requestID)).Order(dbent.Asc(invoiceattachment.FieldCreatedAt)).All(ctx)
}

// ListDeliveries 返回管理员核对邮件投递所需的历史记录。
func (s *InvoiceService) ListDeliveries(ctx context.Context, requestID int64) ([]*dbent.InvoiceDelivery, error) {
	return s.entClient.InvoiceDelivery.Query().Where(invoicedelivery.InvoiceRequestIDEQ(requestID)).Order(dbent.Desc(invoicedelivery.FieldCreatedAt)).All(ctx)
}

// Approve 将待审批申请转为可开票状态。
func (s *InvoiceService) Approve(ctx context.Context, requestID, adminID int64) (*dbent.InvoiceRequest, error) {
	return s.updateReviewState(ctx, requestID, adminID, InvoiceStatusApproved, "")
}

// Reject 驳回申请并释放其订单占用。
func (s *InvoiceService) Reject(ctx context.Context, requestID, adminID int64, reason string) (*dbent.InvoiceRequest, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, infraerrors.BadRequest("INVOICE_REJECTION_REASON_REQUIRED", "rejection reason is required")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("start invoice rejection transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	updated, err := tx.InvoiceRequest.Update().Where(invoicerequest.IDEQ(requestID), invoicerequest.StatusEQ(InvoiceStatusSubmitted)).
		SetStatus(InvoiceStatusRejected).SetRejectionReason(reason).SetReviewedBy(adminID).SetReviewedAt(time.Now().UTC()).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("reject invoice request: %w", err)
	}
	if updated != 1 {
		return nil, infraerrors.Conflict("INVOICE_REQUEST_STATE_INVALID", "invoice request is not awaiting approval")
	}
	if _, err := tx.InvoiceRequestItem.Update().Where(invoicerequestitem.InvoiceRequestIDEQ(requestID), invoicerequestitem.ActiveEQ(true)).SetActive(false).Save(ctx); err != nil {
		return nil, fmt.Errorf("release invoice orders: %w", err)
	}
	request, err := tx.InvoiceRequest.Get(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("read rejected invoice request: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit invoice rejection: %w", err)
	}
	return request, nil
}

// Issue 保存管理员录入的开票信息，且要求申请已有附件。
func (s *InvoiceService) Issue(ctx context.Context, requestID, adminID int64, input InvoiceIssueInput) (*dbent.InvoiceRequest, error) {
	input.InvoiceNumber = strings.TrimSpace(input.InvoiceNumber)
	input.Remark = strings.TrimSpace(input.Remark)
	attachments, err := s.GetAttachments(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list invoice attachments: %w", err)
	}
	if len(attachments) == 0 {
		return nil, infraerrors.BadRequest("INVOICE_ATTACHMENT_REQUIRED", "upload at least one invoice attachment before issuing")
	}
	issuedAt := input.IssuedAt.UTC()
	if issuedAt.IsZero() {
		issuedAt = time.Now().UTC()
	}
	update := s.entClient.InvoiceRequest.Update().Where(invoicerequest.IDEQ(requestID), invoicerequest.StatusEQ(InvoiceStatusApproved)).
		SetStatus(InvoiceStatusIssued).SetIssuedAt(issuedAt).SetIssueRemark(input.Remark).SetReviewedBy(adminID)
	if input.InvoiceNumber != "" {
		update.SetInvoiceNumber(input.InvoiceNumber)
	} else {
		update.ClearInvoiceNumber()
	}
	updated, err := update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("issue invoice request: %w", err)
	}
	if updated != 1 {
		return nil, infraerrors.Conflict("INVOICE_REQUEST_STATE_INVALID", "invoice request must be approved before issuing")
	}
	return s.entClient.InvoiceRequest.Get(ctx, requestID)
}

// DeleteAttachment 删除尚未发送的管理员发票附件，并清理对应存储文件。
func (s *InvoiceService) DeleteAttachment(ctx context.Context, attachmentID, adminID int64) error {
	attachment, err := s.entClient.InvoiceAttachment.Get(ctx, attachmentID)
	if err != nil {
		return infraerrors.NotFound("INVOICE_ATTACHMENT_NOT_FOUND", "invoice attachment not found")
	}
	request, err := s.entClient.InvoiceRequest.Get(ctx, attachment.InvoiceRequestID)
	if err != nil {
		return infraerrors.NotFound("INVOICE_REQUEST_NOT_FOUND", "invoice request not found")
	}
	if request.Status != InvoiceStatusApproved && request.Status != InvoiceStatusIssued {
		return infraerrors.Conflict("INVOICE_ATTACHMENT_DELETE_INVALID", "attachments cannot be deleted after invoice delivery")
	}
	store, err := s.attachmentStore(ctx, attachment)
	if err != nil {
		return err
	}
	if err := store.Delete(ctx, attachment.StorageKey); err != nil {
		return fmt.Errorf("remove invoice attachment file: %w", err)
	}
	if err := s.entClient.InvoiceAttachment.DeleteOneID(attachmentID).Exec(ctx); err != nil {
		return fmt.Errorf("delete invoice attachment: %w", err)
	}
	return nil
}

// Send 将已开具发票和当前附件发送到申请时冻结的收件邮箱。
func (s *InvoiceService) Send(ctx context.Context, requestID, adminID int64) (*dbent.InvoiceRequest, error) {
	request, err := s.entClient.InvoiceRequest.Get(ctx, requestID)
	if err != nil {
		return nil, infraerrors.NotFound("INVOICE_REQUEST_NOT_FOUND", "invoice request not found")
	}
	if request.Status != InvoiceStatusIssued {
		return nil, infraerrors.Conflict("INVOICE_REQUEST_STATE_INVALID", "invoice request must be issued before sending")
	}
	attachments, err := s.GetAttachments(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list invoice attachments: %w", err)
	}
	if len(attachments) == 0 {
		return nil, infraerrors.BadRequest("INVOICE_ATTACHMENT_REQUIRED", "invoice attachment is required before sending")
	}
	mailAttachments := make([]EmailAttachment, 0, len(attachments))
	summaries := make([]string, 0, len(attachments))
	var attachmentBytes int64
	for _, attachment := range attachments {
		attachmentBytes += attachment.SizeBytes
		if attachmentBytes > invoiceEmailAttachmentsMaxBytes {
			return nil, infraerrors.BadRequest("INVOICE_EMAIL_ATTACHMENT_LIMIT", "invoice email attachments cannot exceed 20 MiB in total")
		}
		file, openErr := s.openAttachment(ctx, attachment)
		if openErr != nil {
			return nil, openErr
		}
		data, readErr := io.ReadAll(file)
		_ = file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read invoice attachment %d: %w", attachment.ID, readErr)
		}
		mailAttachments = append(mailAttachments, EmailAttachment{Name: attachment.FileName, ContentType: attachment.ContentType, Data: data})
		summaries = append(summaries, fmt.Sprintf("%s (%d bytes, %s)", attachment.FileName, attachment.SizeBytes, attachment.Sha256))
	}
	items, err := s.GetRequestItems(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("list invoice items: %w", err)
	}
	user, err := s.entClient.User.Get(ctx, request.UserID)
	if err != nil {
		return nil, fmt.Errorf("load invoice user: %w", err)
	}
	if s.notification == nil {
		return nil, fmt.Errorf("notification email service is not configured")
	}
	now := time.Now().UTC()
	messageID, err := s.notification.SendWithMessageID(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventInvoiceSent,
		RecipientEmail: request.RecipientEmail,
		RecipientName:  user.Username,
		UserID:         request.UserID,
		SourceType:     "invoice_request",
		SourceID:       request.RequestNo,
		ReminderKey:    now.Format(time.RFC3339Nano),
		Variables: map[string]string{
			"invoice_request_no": request.RequestNo,
			"invoice_number":     stringValue(request.InvoiceNumber),
			"invoice_title":      request.InvoiceTitle,
			"invoice_amount":     fmt.Sprintf("%.2f", request.TotalAmount),
			"currency":           request.Currency,
			"payment_items":      invoicePaymentItemsDisplay(items),
			"issued_at":          request.IssuedAt.UTC().Format("2006-01-02 15:04"),
		},
		Attachments: mailAttachments,
	})
	if err != nil {
		_ = s.recordDelivery(ctx, requestID, request.RecipientEmail, "FAILED", "", strings.Join(summaries, "; "), err.Error(), adminID)
		return nil, err
	}
	if err := s.recordDelivery(ctx, requestID, request.RecipientEmail, "SENT", messageID, strings.Join(summaries, "; "), "", adminID); err != nil {
		return nil, err
	}
	updated, err := s.entClient.InvoiceRequest.Update().Where(invoicerequest.IDEQ(requestID), invoicerequest.StatusEQ(InvoiceStatusIssued)).SetStatus(InvoiceStatusSent).SetSentAt(now).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("mark invoice request sent: %w", err)
	}
	if updated != 1 {
		return nil, infraerrors.Conflict("INVOICE_REQUEST_STATE_INVALID", "invoice request state changed before delivery was recorded")
	}
	return s.entClient.InvoiceRequest.Get(ctx, requestID)
}

// UploadAttachment 持久保存管理员上传的发票附件。
func (s *InvoiceService) UploadAttachment(ctx context.Context, requestID, adminID int64, fileName, declaredContentType string, body io.Reader) (*dbent.InvoiceAttachment, error) {
	request, err := s.entClient.InvoiceRequest.Get(ctx, requestID)
	if err != nil {
		return nil, infraerrors.NotFound("INVOICE_REQUEST_NOT_FOUND", "invoice request not found")
	}
	if request.Status != InvoiceStatusApproved && request.Status != InvoiceStatusIssued {
		return nil, infraerrors.Conflict("INVOICE_REQUEST_STATE_INVALID", "attachments can only be uploaded after approval")
	}
	count, err := s.entClient.InvoiceAttachment.Query().Where(invoiceattachment.InvoiceRequestIDEQ(requestID)).Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count invoice attachments: %w", err)
	}
	if count >= invoiceAttachmentsMaxPerRequest {
		return nil, infraerrors.BadRequest("INVOICE_ATTACHMENT_LIMIT", "too many invoice attachments")
	}
	data, err := io.ReadAll(io.LimitReader(body, invoiceAttachmentMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read invoice attachment: %w", err)
	}
	if len(data) == 0 || int64(len(data)) > invoiceAttachmentMaxBytes {
		return nil, infraerrors.BadRequest("INVOICE_ATTACHMENT_SIZE_INVALID", "invoice attachment must be between 1 byte and 10 MiB")
	}
	contentType, extension, ok := normalizeInvoiceAttachmentType(data, declaredContentType)
	if !ok {
		return nil, infraerrors.BadRequest("INVOICE_ATTACHMENT_TYPE_INVALID", "only PDF, PNG, and JPEG invoice attachments are allowed")
	}
	fileName = safeInvoiceFileName(fileName, extension)
	storageKey, err := newInvoiceStorageKey(extension)
	if err != nil {
		return nil, fmt.Errorf("generate invoice attachment key: %w", err)
	}
	profileID := s.fileStorage.CurrentInvoiceAttachmentProfileID()
	store, err := s.fileStorage.ResolveInvoiceAttachmentStore(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("resolve invoice attachment storage: %w", err)
	}
	if err := store.Put(ctx, storageKey, contentType, data); err != nil {
		return nil, fmt.Errorf("store invoice attachment: %w", err)
	}
	sum := sha256.Sum256(data)
	attachment, err := s.entClient.InvoiceAttachment.Create().SetInvoiceRequestID(requestID).SetFileName(fileName).
		SetContentType(contentType).SetSizeBytes(int64(len(data))).SetStorageKey(storageKey).SetStorageType(s.fileStorage.InvoiceAttachmentStorageType(profileID)).SetStorageProfileID(profileID).SetSha256(hex.EncodeToString(sum[:])).SetUploadedBy(adminID).Save(ctx)
	if err != nil {
		_ = store.Delete(ctx, storageKey)
		return nil, fmt.Errorf("create invoice attachment record: %w", err)
	}
	return attachment, nil
}

// OpenAttachmentForUser 打开用户拥有的发票附件，并由调用方负责关闭文件。
func (s *InvoiceService) OpenAttachmentForUser(ctx context.Context, attachmentID, userID int64) (*dbent.InvoiceAttachment, io.ReadCloser, error) {
	attachment, err := s.entClient.InvoiceAttachment.Get(ctx, attachmentID)
	if err != nil {
		return nil, nil, infraerrors.NotFound("INVOICE_ATTACHMENT_NOT_FOUND", "invoice attachment not found")
	}
	request, err := s.GetUserRequest(ctx, attachment.InvoiceRequestID, userID)
	if err != nil {
		return nil, nil, err
	}
	_ = request
	file, err := s.openAttachment(ctx, attachment)
	if err != nil {
		return nil, nil, err
	}
	return attachment, file, nil
}

// OpenAttachmentForAdmin 打开管理员有权读取的发票附件。
func (s *InvoiceService) OpenAttachmentForAdmin(ctx context.Context, attachmentID int64) (*dbent.InvoiceAttachment, io.ReadCloser, error) {
	attachment, err := s.entClient.InvoiceAttachment.Get(ctx, attachmentID)
	if err != nil {
		return nil, nil, infraerrors.NotFound("INVOICE_ATTACHMENT_NOT_FOUND", "invoice attachment not found")
	}
	file, err := s.openAttachment(ctx, attachment)
	if err != nil {
		return nil, nil, err
	}
	return attachment, file, nil
}

func (s *InvoiceService) updateReviewState(ctx context.Context, requestID, adminID int64, status, reason string) (*dbent.InvoiceRequest, error) {
	updated, err := s.entClient.InvoiceRequest.Update().Where(invoicerequest.IDEQ(requestID), invoicerequest.StatusEQ(InvoiceStatusSubmitted)).
		SetStatus(status).SetReviewedBy(adminID).SetReviewedAt(time.Now().UTC()).SetRejectionReason(reason).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("review invoice request: %w", err)
	}
	if updated != 1 {
		return nil, infraerrors.Conflict("INVOICE_REQUEST_STATE_INVALID", "invoice request is not awaiting approval")
	}
	return s.entClient.InvoiceRequest.Get(ctx, requestID)
}

func (s *InvoiceService) recordDelivery(ctx context.Context, requestID int64, recipient, status, messageID, summary, errorMessage string, adminID int64) error {
	create := s.entClient.InvoiceDelivery.Create().SetInvoiceRequestID(requestID).SetRecipientEmail(recipient).SetStatus(status).SetAttachmentSummary(summary).SetSentBy(adminID)
	if strings.TrimSpace(messageID) != "" {
		create.SetMessageID(messageID)
	}
	if strings.TrimSpace(errorMessage) != "" {
		create.SetErrorMessage(errorMessage)
	}
	_, err := create.Save(ctx)
	return err
}

func (s *InvoiceService) openAttachment(ctx context.Context, attachment *dbent.InvoiceAttachment) (io.ReadCloser, error) {
	if attachment == nil || strings.TrimSpace(attachment.StorageKey) == "" {
		return nil, infraerrors.NotFound("INVOICE_ATTACHMENT_NOT_FOUND", "invoice attachment not found")
	}
	store, err := s.attachmentStore(ctx, attachment)
	if err != nil {
		return nil, err
	}
	file, err := store.Open(ctx, attachment.StorageKey)
	if os.IsNotExist(err) {
		return nil, infraerrors.NotFound("INVOICE_ATTACHMENT_MISSING", "invoice attachment file is missing")
	}
	if err != nil {
		return nil, fmt.Errorf("open invoice attachment: %w", err)
	}
	return file, nil
}

// attachmentStore 通过附件的不可变档案解析存储，旧数据默认使用本地档案。
func (s *InvoiceService) attachmentStore(ctx context.Context, attachment *dbent.InvoiceAttachment) (FileObjectStore, error) {
	if s == nil || s.fileStorage == nil {
		return nil, fmt.Errorf("invoice attachment storage is not configured")
	}
	return s.fileStorage.ResolveInvoiceAttachmentStore(ctx, attachment.StorageProfileID)
}

func validateInvoiceOrders(orders []*dbent.PaymentOrder) (string, float64, error) {
	if len(orders) == 0 {
		return "", 0, infraerrors.BadRequest("INVOICE_ORDERS_REQUIRED", "select at least one completed order")
	}
	currency := ""
	total := 0.0
	for _, order := range orders {
		if order == nil || order.Status != OrderStatusCompleted || (order.OrderType != payment.OrderTypeBalance && order.OrderType != payment.OrderTypeSubscription) || order.PayAmount <= 0 || math.IsNaN(order.PayAmount) || math.IsInf(order.PayAmount, 0) {
			return "", 0, infraerrors.BadRequest("INVOICE_ORDER_INELIGIBLE", "selected orders must be completed balance or subscription purchases")
		}
		orderCurrency := PaymentOrderCurrency(order)
		if currency == "" {
			currency = orderCurrency
		} else if currency != orderCurrency {
			return "", 0, infraerrors.BadRequest("INVOICE_CURRENCY_MISMATCH", "invoice orders must use the same currency")
		}
		total += order.PayAmount
	}
	if total <= 0 || math.IsNaN(total) || math.IsInf(total, 0) {
		return "", 0, infraerrors.BadRequest("INVOICE_AMOUNT_INVALID", "invoice amount is invalid")
	}
	return currency, total, nil
}

func invoiceOrderSnapshot(order *dbent.PaymentOrder) map[string]any {
	if order.OrderType == payment.OrderTypeSubscription {
		return map[string]any{
			"plan_name":         order.PlanSnapshot.Name,
			"validity_days":     order.PlanSnapshot.ValidityDays,
			"daily_limit_usd":   order.PlanSnapshot.DailyLimitUSD,
			"weekly_limit_usd":  order.PlanSnapshot.WeeklyLimitUSD,
			"monthly_limit_usd": order.PlanSnapshot.MonthlyLimitUSD,
		}
	}
	return map[string]any{"credited_amount": order.Amount}
}

func uniquePositiveInvoiceIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func normalizeInvoiceType(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return InvoiceTypePersonal
	}
	if value == InvoiceTypePersonal || value == InvoiceTypeEnterprise {
		return value
	}
	return ""
}

func newInvoiceRequestNo() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "INV-" + time.Now().UTC().Format("20060102") + "-" + strings.ToUpper(hex.EncodeToString(buf)), nil
}

func newInvoiceStorageKey(extension string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf) + extension, nil
}

func normalizeInvoiceAttachmentType(data []byte, declaredContentType string) (string, string, bool) {
	detected := http.DetectContentType(data)
	if strings.HasPrefix(detected, "application/pdf") && strings.HasPrefix(string(data), "%PDF-") {
		return "application/pdf", ".pdf", true
	}
	if detected == "image/png" {
		return "image/png", ".png", true
	}
	if detected == "image/jpeg" {
		return "image/jpeg", ".jpg", true
	}
	_ = declaredContentType
	return "", "", false
}

func safeInvoiceFileName(fileName, extension string) string {
	base := strings.TrimSpace(filepath.Base(fileName))
	base = strings.ReplaceAll(base, "\x00", "")
	if base == "." || base == "" || len(base) > 200 {
		base = "invoice" + extension
	}
	if !strings.EqualFold(filepath.Ext(base), extension) {
		base += extension
	}
	return base
}

func invoicePaymentItemsDisplay(items []*dbent.InvoiceRequestItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("%s: %.2f %s", item.OrderNo, item.PayAmount, item.Currency))
	}
	return strings.Join(parts, "; ")
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
