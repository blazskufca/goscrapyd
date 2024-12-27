package database

import (
	"database/sql"
	"github.com/blazskufca/goscrapyd/internal/validator"
	"time"
)

func CreateSqlNullInt64FromInt(data *int) sql.NullInt64 {
	if data == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Valid: true, Int64: int64(*data)}
}

func CreateSqlNullString(strPtr *string) sql.NullString {
	if strPtr == nil {
		return sql.NullString{}
	}
	data := *strPtr
	return sql.NullString{String: data, Valid: validator.NotBlank(data)}
}

func CreateSqlNullTimePtr(data *time.Time) sql.NullTime {
	if data == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *data, Valid: true}
}

func CreateCreateSqlNullTimeNonPtr(data time.Time) sql.NullTime {
	return CreateSqlNullTimePtr(&data)
}

func ReadSqlNullString(data sql.NullString) *string {
	if data.Valid {
		return &data.String
	}
	return nil
}
