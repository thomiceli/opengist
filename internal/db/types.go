package db

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type binaryData []byte

func (b *binaryData) Value() (driver.Value, error) {
	return []byte(*b), nil
}

func (b *binaryData) Scan(value interface{}) error {
	valBytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal BinaryData: %v", value)
	}
	*b = valBytes
	return nil
}

func (*binaryData) GormDataType() string {
	return "binary_data"
}

func (*binaryData) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "sqlite":
		return "BLOB"
	case "mysql":
		return "VARBINARY(1024)"
	case "postgres":
		return "BYTEA"
	default:
		return "BLOB"
	}
}

type jsonData json.RawMessage

func (j *jsonData) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	result := json.RawMessage{}
	err := json.Unmarshal(bytes, &result)
	*j = jsonData(result)
	return err
}

func (j *jsonData) Value() (driver.Value, error) {
	if len(*j) == 0 {
		return nil, nil
	}
	return json.RawMessage(*j).MarshalJSON()
}

func (*jsonData) GormDataType() string {
	return "json"
}

func (*jsonData) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	switch db.Dialector.Name() {
	case "mysql", "sqlite":
		return "JSON"
	case "postgres":
		return "JSONB"
	}
	return ""
}
