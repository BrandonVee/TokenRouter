package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// InvoiceAttachment 保存受保护的发票文件元数据。
type InvoiceAttachment struct {
	ent.Schema
}

func (InvoiceAttachment) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "invoice_attachments"}}
}

func (InvoiceAttachment) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("invoice_request_id"),
		field.String("file_name").MaxLen(255),
		field.String("content_type").MaxLen(100),
		field.Int64("size_bytes"),
		field.String("storage_key").MaxLen(255).Unique(),
		field.String("sha256").MaxLen(64),
		field.Int64("uploaded_by"),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (InvoiceAttachment) Indexes() []ent.Index {
	return []ent.Index{index.Fields("invoice_request_id", "created_at")}
}
