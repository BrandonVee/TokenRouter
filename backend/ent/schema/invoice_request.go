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

// InvoiceRequest 保存用户在支付完成后提交的人工发票申请。
type InvoiceRequest struct {
	ent.Schema
}

func (InvoiceRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "invoice_requests"}}
}

func (InvoiceRequest) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("request_no").MaxLen(64).Unique(),
		field.String("status").MaxLen(20).Default("SUBMITTED"),
		field.String("currency").MaxLen(16),
		field.Float("total_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),
		field.String("invoice_type").MaxLen(20).Default("PERSONAL"),
		field.String("invoice_title").MaxLen(255),
		field.String("tax_id").Optional().Nillable().MaxLen(128),
		field.String("bank_name").Optional().Nillable().MaxLen(255),
		field.String("bank_account").Optional().Nillable().MaxLen(128),
		field.String("recipient_email").MaxLen(255),
		field.String("account_email").MaxLen(255),
		field.String("remark").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("rejection_reason").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("reviewed_by").Optional().Nillable(),
		field.Time("reviewed_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("invoice_number").Optional().Nillable().MaxLen(128),
		field.Time("issued_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("issue_remark").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("sent_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (InvoiceRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("status", "created_at"),
		index.Fields("invoice_number"),
	}
}
