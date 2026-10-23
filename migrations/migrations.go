package migrations

import (
	_ "embed"
)

//go:embed 000001_init_schema.up.sql
var SchemaV1 string

//go:embed 000002_auth_refresh_tokens.up.sql
var SchemaV2 string
