package db

import (
	"database/sql/driver"
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
