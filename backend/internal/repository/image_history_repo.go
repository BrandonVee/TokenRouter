package repository

import (
	"context"
	"strings"

	dbent "github.com/BrandonVee/TokenRouter/ent"
	"github.com/BrandonVee/TokenRouter/ent/imagehistory"
	"github.com/BrandonVee/TokenRouter/ent/user"
	"github.com/BrandonVee/TokenRouter/internal/service"
)

type imageHistoryRepository struct {
	client *dbent.Client
}

// NewImageHistoryRepository 创建用户生图历史元数据仓储。
func NewImageHistoryRepository(client *dbent.Client) service.ImageHistoryRepository {
	return &imageHistoryRepository{client: client}
}

func (r *imageHistoryRepository) GetSavingEnabled(ctx context.Context, userID int64) (bool, error) {
	item, err := clientFromContext(ctx, r.client).User.Query().
		Where(user.IDEQ(userID)).
		Select(user.FieldSaveImageHistory).
		Only(ctx)
	if err != nil {
		return false, err
	}
	return item.SaveImageHistory, nil
}

func (r *imageHistoryRepository) SetSavingEnabled(ctx context.Context, userID int64, enabled bool) error {
	return clientFromContext(ctx, r.client).User.UpdateOneID(userID).
		SetSaveImageHistory(enabled).
		Exec(ctx)
}

func (r *imageHistoryRepository) Create(ctx context.Context, record service.ImageHistoryRecord) error {
	return clientFromContext(ctx, r.client).ImageHistory.Create().
		SetID(record.ID).
		SetUserID(record.UserID).
		SetNillableAPIKeyID(record.APIKeyID).
		SetRequestID(record.RequestID).
		SetSource(record.Source).
		SetEndpoint(record.Endpoint).
		SetModel(record.Model).
		SetPrompt(record.Prompt).
		SetRevisedPrompt(record.RevisedPrompt).
		SetParameters(record.Parameters).
		SetObjectKey(record.ObjectKey).
		SetMimeType(record.MimeType).
		SetSizeBytes(record.SizeBytes).
		SetWidth(record.Width).
		SetHeight(record.Height).
		SetSha256(record.SHA256).
		SetCreatedAt(record.CreatedAt).
		Exec(ctx)
}

func (r *imageHistoryRepository) List(ctx context.Context, userID int64, params service.ImageHistoryListParams) ([]service.ImageHistoryRecord, int64, error) {
	client := clientFromContext(ctx, r.client)
	filter := imagehistory.UserIDEQ(userID)
	if search := strings.TrimSpace(params.Search); search != "" {
		// 搜索只作用于当前用户，并覆盖列表中可见的主要元数据字段。
		filter = imagehistory.And(filter, imagehistory.Or(
			imagehistory.IDContainsFold(search),
			imagehistory.RequestIDContainsFold(search),
			imagehistory.SourceContainsFold(search),
			imagehistory.EndpointContainsFold(search),
			imagehistory.ModelContainsFold(search),
			imagehistory.PromptContainsFold(search),
			imagehistory.RevisedPromptContainsFold(search),
		))
	}
	total, err := client.ImageHistory.Query().Where(filter).Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	items, err := client.ImageHistory.Query().
		Where(filter).
		Order(dbent.Desc(imagehistory.FieldCreatedAt), dbent.Desc(imagehistory.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	records := make([]service.ImageHistoryRecord, 0, len(items))
	for _, item := range items {
		records = append(records, imageHistoryEntityToService(item))
	}
	return records, int64(total), nil
}

func (r *imageHistoryRepository) Get(ctx context.Context, userID int64, id string) (*service.ImageHistoryRecord, error) {
	item, err := clientFromContext(ctx, r.client).ImageHistory.Query().
		Where(imagehistory.IDEQ(id), imagehistory.UserIDEQ(userID)).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return nil, service.ErrImageHistoryNotFound
	}
	if err != nil {
		return nil, err
	}
	record := imageHistoryEntityToService(item)
	return &record, nil
}

func (r *imageHistoryRepository) Delete(ctx context.Context, userID int64, id string) error {
	deleted, err := clientFromContext(ctx, r.client).ImageHistory.Delete().
		Where(imagehistory.IDEQ(id), imagehistory.UserIDEQ(userID)).
		Exec(ctx)
	if err != nil {
		return err
	}
	if deleted == 0 {
		return service.ErrImageHistoryNotFound
	}
	return nil
}

func imageHistoryEntityToService(item *dbent.ImageHistory) service.ImageHistoryRecord {
	return service.ImageHistoryRecord{
		ID:            item.ID,
		UserID:        item.UserID,
		APIKeyID:      item.APIKeyID,
		RequestID:     item.RequestID,
		Source:        item.Source,
		Endpoint:      item.Endpoint,
		Model:         item.Model,
		Prompt:        item.Prompt,
		RevisedPrompt: item.RevisedPrompt,
		Parameters:    item.Parameters,
		ObjectKey:     item.ObjectKey,
		MimeType:      item.MimeType,
		SizeBytes:     item.SizeBytes,
		Width:         item.Width,
		Height:        item.Height,
		SHA256:        item.Sha256,
		CreatedAt:     item.CreatedAt,
	}
}
