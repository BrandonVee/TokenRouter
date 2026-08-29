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

// DashboardAd 保存仪表盘广告内容及展示顺序。
type DashboardAd struct {
	ent.Schema
}

func (DashboardAd) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "dashboard_ads"},
	}
}

func (DashboardAd) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").MaxLen(100),
		field.String("image_url").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("link_url").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Enum("fit_mode").Values("adaptive", "cover", "fill").Default("adaptive"),
		field.Time("starts_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("ends_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Bool("enabled").Default(true),
		field.Int("sort_order").Default(0),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (DashboardAd) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sort_order"),
		index.Fields("enabled"),
	}
}
