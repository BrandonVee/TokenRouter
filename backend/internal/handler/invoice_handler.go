package handler

import (
	"mime"
	"net/http"
	"strconv"

	dbent "github.com/BrandonVee/TokenRouter/ent"
	"github.com/BrandonVee/TokenRouter/internal/handler/dto"
	"github.com/BrandonVee/TokenRouter/internal/pkg/response"
	"github.com/BrandonVee/TokenRouter/internal/service"

	"github.com/gin-gonic/gin"
)

// createInvoiceRequestBody 是用户提交发票申请的请求体。
type createInvoiceRequestBody struct {
	OrderIDs       []int64 `json:"order_ids" binding:"required"`
	InvoiceType    string  `json:"invoice_type"`
	InvoiceTitle   string  `json:"invoice_title" binding:"required"`
	TaxID          string  `json:"tax_id"`
	BankName       string  `json:"bank_name"`
	BankAccount    string  `json:"bank_account"`
	RecipientEmail string  `json:"recipient_email"`
	Remark         string  `json:"remark"`
}

// GetEligibleInvoiceOrders 返回用户可合并申请开票的订单。
func (h *PaymentHandler) GetEligibleInvoiceOrders(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok || !h.requireInvoiceService(c) {
		return
	}
	page, pageSize := response.ParsePagination(c)
	orders, total, err := h.invoiceService.ListEligibleOrders(c.Request.Context(), subject.UserID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, sanitizePaymentOrdersForResponse(orders), int64(total), page, pageSize)
}

// ListInvoiceRequests 返回当前用户的发票申请列表。
func (h *PaymentHandler) ListInvoiceRequests(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok || !h.requireInvoiceService(c) {
		return
	}
	page, pageSize := response.ParsePagination(c)
	requests, total, err := h.invoiceService.ListUserRequests(c.Request.Context(), subject.UserID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, invoiceRequestsToDTO(requests, false), int64(total), page, pageSize)
}

// CreateInvoiceRequest 提交用户填写的开票申请。
func (h *PaymentHandler) CreateInvoiceRequest(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok || !h.requireInvoiceService(c) {
		return
	}
	var body createInvoiceRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid invoice request: "+err.Error())
		return
	}
	request, err := h.invoiceService.Create(c.Request.Context(), subject.UserID, service.InvoiceCreateInput{
		OrderIDs: body.OrderIDs, InvoiceType: body.InvoiceType, InvoiceTitle: body.InvoiceTitle, TaxID: body.TaxID, BankName: body.BankName, BankAccount: body.BankAccount, RecipientEmail: body.RecipientEmail, Remark: body.Remark,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, invoiceRequestToDTO(request, false))
}

// GetInvoiceRequest 返回当前用户的一份发票申请及其安全附件元数据。
func (h *PaymentHandler) GetInvoiceRequest(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok || !h.requireInvoiceService(c) {
		return
	}
	requestID, ok := parseInvoiceID(c, "id")
	if !ok {
		return
	}
	request, err := h.invoiceService.GetUserRequest(c.Request.Context(), requestID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items, err := h.invoiceService.GetRequestItems(c.Request.Context(), requestID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	attachments, err := h.invoiceService.GetAttachments(c.Request.Context(), requestID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"request": invoiceRequestToDTO(request, false), "items": invoiceItemsToDTO(items), "attachments": invoiceAttachmentsToDTO(attachments)})
}

// DownloadInvoiceAttachment 下载当前用户有权读取的附件。
func (h *PaymentHandler) DownloadInvoiceAttachment(c *gin.Context) {
	subject, ok := requireAuth(c)
	if !ok || !h.requireInvoiceService(c) {
		return
	}
	attachmentID, ok := parseInvoiceID(c, "attachment_id")
	if !ok {
		return
	}
	attachment, file, err := h.invoiceService.OpenAttachmentForUser(c.Request.Context(), attachmentID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = file.Close() }()
	c.Header("Content-Type", attachment.ContentType)
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment.FileName}))
	http.ServeContent(c.Writer, c.Request, attachment.FileName, attachment.CreatedAt, file)
}

func (h *PaymentHandler) requireInvoiceService(c *gin.Context) bool {
	if h.invoiceService != nil {
		return true
	}
	response.InternalError(c, "invoice service is not configured")
	return false
}

func parseInvoiceID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid "+name)
		return 0, false
	}
	return id, true
}

func invoiceRequestToDTO(request *dbent.InvoiceRequest, includeAccountEmail bool) dto.InvoiceRequestResponse {
	result := dto.InvoiceRequestResponse{
		ID: request.ID, UserID: request.UserID, RequestNo: request.RequestNo, Status: request.Status, Currency: request.Currency,
		TotalAmount: request.TotalAmount, InvoiceType: request.InvoiceType, InvoiceTitle: request.InvoiceTitle, TaxID: request.TaxID, BankName: request.BankName, BankAccount: request.BankAccount, RecipientEmail: request.RecipientEmail,
		Remark: request.Remark, RejectionReason: request.RejectionReason, ReviewedBy: request.ReviewedBy, ReviewedAt: request.ReviewedAt,
		InvoiceNumber: request.InvoiceNumber, IssuedAt: request.IssuedAt, IssueRemark: request.IssueRemark, SentAt: request.SentAt,
		CreatedAt: request.CreatedAt, UpdatedAt: request.UpdatedAt,
	}
	if includeAccountEmail {
		result.AccountEmail = request.AccountEmail
	}
	return result
}

func invoiceRequestsToDTO(requests []*dbent.InvoiceRequest, includeAccountEmail bool) []dto.InvoiceRequestResponse {
	result := make([]dto.InvoiceRequestResponse, 0, len(requests))
	for _, request := range requests {
		result = append(result, invoiceRequestToDTO(request, includeAccountEmail))
	}
	return result
}

func invoiceItemsToDTO(items []*dbent.InvoiceRequestItem) []dto.InvoiceRequestItemResponse {
	result := make([]dto.InvoiceRequestItemResponse, 0, len(items))
	for _, item := range items {
		result = append(result, dto.InvoiceRequestItemResponse{ID: item.ID, PaymentOrderID: item.PaymentOrderID, OrderNo: item.OrderNo, OrderType: item.OrderType,
			Currency: item.Currency, PayAmount: item.PayAmount, CreditedAmount: item.CreditedAmount, RechargeCode: item.RechargeCode, ProductSnapshot: item.ProductSnapshot, PaidAt: item.PaidAt})
	}
	return result
}

func invoiceAttachmentsToDTO(attachments []*dbent.InvoiceAttachment) []dto.InvoiceAttachmentResponse {
	result := make([]dto.InvoiceAttachmentResponse, 0, len(attachments))
	for _, attachment := range attachments {
		result = append(result, dto.InvoiceAttachmentResponse{ID: attachment.ID, FileName: attachment.FileName, ContentType: attachment.ContentType, SizeBytes: attachment.SizeBytes, UploadedBy: attachment.UploadedBy, CreatedAt: attachment.CreatedAt})
	}
	return result
}
