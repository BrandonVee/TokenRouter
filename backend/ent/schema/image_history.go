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

// ImageHistory 保存用户主动留存的同步生图元数据，对象字节始终位于私有 S3。
type ImageHistory struct {
	ent.Schema
}

func (ImageHistory) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "image_histories"},
	}
}

func (ImageHistory) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").MaxLen(36).Immutable(),
		field.Int64("user_id").Immutable(),
		field.Int64("api_key_id").Optional().Nillable().Immutable(),
		field.String("request_id").MaxLen(255).Default(""),
		field.String("source").MaxLen(32),
		field.String("endpoint").MaxLen(64),
		field.String("model").MaxLen(255).Default(""),
		field.String("prompt").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.String("revised_prompt").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.String("parameters").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.String("object_key").SchemaType(map[string]string{dialect.Postgres: "text"}).Immutable(),
		field.String("mime_type").MaxLen(100),
		field.Int64("size_bytes").Positive(),
		field.Int("width").NonNegative().Default(0),
		field.Int("height").NonNegative().Default(0),
		field.String("sha256").MaxLen(64),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (ImageHistory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("request_id"),
		index.Fields("object_key").Unique(),
	}
}
