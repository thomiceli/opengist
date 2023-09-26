package memdb

import "github.com/hashicorp/go-memdb"
import ogdb "github.com/thomiceli/opengist/internal/db"

var db *memdb.MemDB

type GistInit struct {
	UserID uint
	Gist   *ogdb.Gist
}

func Setup() error {
	var err error
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"gist_init": {
				Name: "gist_init",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.UintFieldIndex{Field: "UserID"},
					},
				},
			},
		},
	}

	db, err = memdb.NewMemDB(schema)
	if err != nil {
		return err
	}

	return nil
}

func InsertGistInit(userId uint, gist *ogdb.Gist) error {
	txn := db.Txn(true)
	if err := txn.Insert("gist_init", &GistInit{
		UserID: userId,
		Gist:   gist,
	}); err != nil {
		txn.Abort()
		return err
	}

	txn.Commit()
	return nil
}

func GetGistInitAndDelete(userId uint) (*GistInit, error) {
	txn := db.Txn(true)
	defer txn.Abort()

	raw, err := txn.First("gist_init", "id", userId)
	if err != nil {
		return nil, err
	}

	if raw == nil {
		return nil, nil
	}

	gistInit := raw.(*GistInit)
	if err := txn.Delete("gist_init", gistInit); err != nil {
		return nil, err
	}

	txn.Commit()
	return gistInit, nil
}
