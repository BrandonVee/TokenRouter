package admin

import (
	"mime"
	"net/http"
	"strconv"
	"time"

	dbent "github.com/BrandonVee/TokenRouter/ent"
	"github.com/BrandonVee/TokenRouter/internal/handler/dto"
	"github.com/BrandonVee/TokenRouter/internal/pkg/response"
	"github.com/BrandonVee/TokenRouter/internal/server/middleware"
	"github.com/BrandonVee/TokenRouter/internal/service"

	"github.com/gin-gonic/gin"
)

type invoiceRejectBody struct {
	Reason string `json:"reason" binding:"required"`
}
type invoiceIssueBody struct {
	InvoiceNumber string     `json:"invoice_number" binding:"required"`
	IssuedAt      *time.Time `json:"issued_at"`
	Remark        string     `json:"remark"`
}

// ListInvoices 返回管理员可筛选的人工发票申请。
func (h *PaymentHandler) ListInvoices(c *gin.Context) {
	if !h.requireInvoiceService(c) {
		return
	}
	page, pageSize := response.ParsePagination(c)
	requests, total, err := h.invoiceService.ListAdminRequests(c.Request.Context(), c.Query("status"), c.Query("keyword"), page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, adminInvoiceRequestsToDTO(requests), int64(total), page, pageSize)
}

// GetInvoice 返回审批、开票和投递所需的完整申请详情。
func (h *PaymentHandler) GetInvoice(c *gin.Context) {
	if !h.requireInvoiceService(c) {
		return
	}
	id, ok := parseInvoiceID(c, "id")
	if !ok {
		return
	}
	request, err := h.invoiceService.GetAdminRequest(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items, err := h.invoiceService.GetRequestItems(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	attachments, err := h.invoiceService.GetAttachments(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	deliveries, err := h.invoiceService.ListDeliveries(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"request": adminInvoiceRequestToDTO(request), "items": adminInvoiceItemsToDTO(items), "attachments": adminInvoiceAttachmentsToDTO(attachments), "deliveries": adminInvoiceDeliveriesToDTO(deliveries)})
}

// ApproveInvoice 审批通过用户提交的申请。
func (h *PaymentHandler) ApproveInvoice(c *gin.Context) {
	if !h.requireInvoiceService(c) {
		return
	}
	id, ok := parseInvoiceID(c, "id")
	if !ok {
		return
	}
	request, err := h.invoiceService.Approve(c.Request.Context(), id, adminUserID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, adminInvoiceRequestToDTO(request))
}

// RejectInvoice 驳回申请并释放订单再次申请。
func (h *PaymentHandler) RejectInvoice(c *gin.Context) {
	if !h.requireInvoiceService(c) {
		return
	}
	id, ok := parseInvoiceID(c, "id")
	if !ok {
		return
	}
	var body invoiceRejectBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid invoice rejection: "+err.Error())
		return
	}
	request, err := h.invoiceService.Reject(c.Request.Context(), id, adminUserID(c), body.Reason)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, adminInvoiceRequestToDTO(request))
}

// UploadInvoiceAttachment 保存管理员开具后的受保护附件。
func (h *PaymentHandler) UploadInvoiceAttachment(c *gin.Context) {
	if !h.requireInvoiceService(c) {
		return
	}
	id, ok := parseInvoiceID(c, "id")
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 11*1024*1024)
	fileHeader, err := c.FormFile("attachment")
	if err != nil {
		response.BadRequest(c, "invoice attachment is required")
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.BadRequest(c, "cannot read invoice attachment")
		return
	}
	defer func() { _ = file.Close() }()
	attachment, err := h.invoiceService.UploadAttachment(c.Request.Context(), id, adminUserID(c), fileHeader.Filename, fileHeader.Header.Get("Content-Type"), file)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, adminInvoiceAttachmentsToDTO([]*dbent.InvoiceAttachment{attachment})[0])
}

// IssueInvoice 录入发票号码和开票日期。
func (h *PaymentHandler) IssueInvoice(c *gin.Context) {
	if !h.requireInvoiceService(c) {
		return
	}
	id, ok := parseInvoiceID(c, "id")
	if !ok {
		return
	}
	var body invoiceIssueBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid invoice issue request: "+err.Error())
		return
	}
	issuedAt := time.Time{}
	if body.IssuedAt != nil {
		issuedAt = *body.IssuedAt
	}
	request, err := h.invoiceService.Issue(c.Request.Context(), id, adminUserID(c), service.InvoiceIssueInput{InvoiceNumber: body.InvoiceNumber, IssuedAt: issuedAt, Remark: body.Remark})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, adminInvoiceRequestToDTO(request))
}

// SendInvoice 将已开具的发票附件发送到冻结的收件邮箱。
func (h *PaymentHandler) SendInvoice(c *gin.Context) {
	if !h.requireInvoiceService(c) {
		return
	}
	id, ok := parseInvoiceID(c, "id")
	if !ok {
		return
	}
	request, err := h.invoiceService.Send(c.Request.Context(), id, adminUserID(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, adminInvoiceRequestToDTO(request))
}

// DownloadInvoiceAttachment 下载管理员有权读取的附件。
func (h *PaymentHandler) DownloadInvoiceAttachment(c *gin.Context) {
	if !h.requireInvoiceService(c) {
		return
	}
	id, ok := parseInvoiceID(c, "attachment_id")
	if !ok {
		return
	}
	attachment, file, err := h.invoiceService.OpenAttachmentForAdmin(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = file.Close() }()
	c.Header("Content-Type", attachment.ContentType)
	// 预览请求以内联方式返回，普通下载仍保持附件响应。
	disposition := "attachment"
	if c.Query("preview") == "1" {
		disposition = "inline"
	}
	c.Header("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": attachment.FileName}))
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
func adminUserID(c *gin.Context) int64 {
	if subject, ok := middleware.GetAuthSubjectFromContext(c); ok {
		return subject.UserID
	}
	return 0
}

func adminInvoiceRequestToDTO(request *dbent.InvoiceRequest) dto.InvoiceRequestResponse {
	return dto.InvoiceRequestResponse{ID: request.ID, UserID: request.UserID, UserEmail: request.AccountEmail, RequestNo: request.RequestNo, Status: request.Status, Currency: request.Currency, TotalAmount: request.TotalAmount, InvoiceType: request.InvoiceType, InvoiceTitle: request.InvoiceTitle, TaxID: request.TaxID, BankName: request.BankName, BankAccount: request.BankAccount, RecipientEmail: request.RecipientEmail, AccountEmail: request.AccountEmail, Remark: request.Remark, RejectionReason: request.RejectionReason, ReviewedBy: request.ReviewedBy, ReviewedAt: request.ReviewedAt, InvoiceNumber: request.InvoiceNumber, IssuedAt: request.IssuedAt, IssueRemark: request.IssueRemark, SentAt: request.SentAt, CreatedAt: request.CreatedAt, UpdatedAt: request.UpdatedAt}
}
func adminInvoiceRequestsToDTO(requests []*dbent.InvoiceRequest) []dto.InvoiceRequestResponse {
	result := make([]dto.InvoiceRequestResponse, 0, len(requests))
	for _, request := range requests {
		result = append(result, adminInvoiceRequestToDTO(request))
	}
	return result
}
func adminInvoiceItemsToDTO(items []*dbent.InvoiceRequestItem) []dto.InvoiceRequestItemResponse {
	result := make([]dto.InvoiceRequestItemResponse, 0, len(items))
	for _, item := range items {
		result = append(result, dto.InvoiceRequestItemResponse{ID: item.ID, PaymentOrderID: item.PaymentOrderID, OrderNo: item.OrderNo, OrderType: item.OrderType, Currency: item.Currency, PayAmount: item.PayAmount, CreditedAmount: item.CreditedAmount, RechargeCode: item.RechargeCode, ProductSnapshot: item.ProductSnapshot, PaidAt: item.PaidAt})
	}
	return result
}
func adminInvoiceAttachmentsToDTO(items []*dbent.InvoiceAttachment) []dto.InvoiceAttachmentResponse {
	result := make([]dto.InvoiceAttachmentResponse, 0, len(items))
	for _, item := range items {
		result = append(result, dto.InvoiceAttachmentResponse{ID: item.ID, FileName: item.FileName, ContentType: item.ContentType, SizeBytes: item.SizeBytes, UploadedBy: item.UploadedBy, CreatedAt: item.CreatedAt})
	}
	return result
}
func adminInvoiceDeliveriesToDTO(items []*dbent.InvoiceDelivery) []dto.InvoiceDeliveryResponse {
	result := make([]dto.InvoiceDeliveryResponse, 0, len(items))
	for _, item := range items {
		result = append(result, dto.InvoiceDeliveryResponse{ID: item.ID, RecipientEmail: item.RecipientEmail, Status: item.Status, MessageID: item.MessageID, ErrorMessage: item.ErrorMessage, SentBy: item.SentBy, CreatedAt: item.CreatedAt})
	}
	return result
}
