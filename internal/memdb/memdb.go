package memdb

import "github.com/hashicorp/go-memdb"
import ogdb "github.com/thomiceli/opengist/internal/db"

var db *memdb.MemDB

type GistPush struct {
	UserID         uint
	GistUUID       string
	RepositoryPath string
	Gist           *ogdb.Gist
}

func Setup() error {
	var err error
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"gist_push": {
				Name: "gist_push",
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

func InsertGistPush(userId uint, gistUuid string, repositoryPath string, gist *ogdb.Gist) error {
	txn := db.Txn(true)
	if err := txn.Insert("gist_push", &GistPush{
		UserID:         userId,
		GistUUID:       gistUuid,
		RepositoryPath: repositoryPath,
		Gist:           gist,
	}); err != nil {
		txn.Abort()
		return err
	}

	txn.Commit()
	return nil
}

func GetGistPushAndDelete(userId uint) (*GistPush, error) {
	txn := db.Txn(true)
	defer txn.Abort()

	raw, err := txn.First("gist_push", "id", userId)
	if err != nil {
		return nil, err
	}

	if raw == nil {
		return nil, nil
	}

	gistPush := raw.(*GistPush)
	if err := txn.Delete("gist_push", gistPush); err != nil {
		return nil, err
	}

	txn.Commit()
	return gistPush, nil
}
