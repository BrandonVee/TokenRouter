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

// InvoiceDelivery 保存每一次开票邮件投递的结果。
type InvoiceDelivery struct {
	ent.Schema
}

func (InvoiceDelivery) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "invoice_deliveries"}}
}

func (InvoiceDelivery) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("invoice_request_id"),
		field.String("recipient_email").MaxLen(255),
		field.String("status").MaxLen(20),
		field.String("message_id").Optional().Nillable().MaxLen(255),
		field.String("attachment_summary").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("error_message").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("sent_by").Optional().Nillable(),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (InvoiceDelivery) Indexes() []ent.Index {
	return []ent.Index{index.Fields("invoice_request_id", "created_at")}
}
