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

// InvoiceRequestItem 保存一笔被冻结的订单开票快照。
type InvoiceRequestItem struct {
	ent.Schema
}

func (InvoiceRequestItem) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "invoice_request_items"}}
}

func (InvoiceRequestItem) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("invoice_request_id"),
		field.Int64("payment_order_id"),
		field.Bool("active").Default(true),
		field.String("order_no").MaxLen(64),
		field.String("order_type").MaxLen(20),
		field.String("currency").MaxLen(16),
		field.Float("pay_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),
		field.Float("credited_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).Default(0),
		field.String("recharge_code").Optional().Nillable().MaxLen(64),
		field.JSON("product_snapshot", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("paid_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (InvoiceRequestItem) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("invoice_request_id"),
		index.Fields("payment_order_id"),
	}
}
