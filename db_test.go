package clover_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/require"

	c "github.com/ostafen/clover/v2"
	d "github.com/ostafen/clover/v2/document"
	"github.com/ostafen/clover/v2/index"
	q "github.com/ostafen/clover/v2/query"
	badgerstore "github.com/ostafen/clover/v2/store/badger"
	"github.com/ostafen/clover/v2/store/bbolt"
)

const (
	airlinesPath = "test/data/airlines.json"
	todosPath    = "test/data/todos.json"
	earthquakes  = "test/data/earthquakes.json"
)

type TodoModel struct {
	Title         string     `json:"title" clover:"title"`
	Completed     bool       `json:"completed,omitempty" clover:"completed"`
	Id            uint       `json:"id" clover:"id"`
	UserId        int        `json:"userId" clover:"userId"`
	CompletedDate *time.Time `json:"completed_date,omitempty" clover:"completed_date,omitempty"`
	Notes         *string    `json:"notes,omitempty" clover:"notes,omitempty"`
}

type dbFactory func(string) (*c.DB, error)

func getBadgerDB(dir string) (*c.DB, error) {
	store, err := badgerstore.Open(dir)
	if err != nil {
		return nil, err
	}
	return c.OpenWithStore(store)
}

func getBBoltDB(dir string) (*c.DB, error) {
	store, err := bbolt.Open(dir)
	if err != nil {
		return nil, err
	}
	return c.OpenWithStore(store)
}

func getDBFactories() []dbFactory {
	return []dbFactory{getBadgerDB, getBBoltDB}
}

func runCloverTest(t *testing.T, test func(t *testing.T, db *c.DB)) {
	for _, createDB := range getDBFactories() {
		dir, err := os.MkdirTemp("", "clover-test")
		require.NoError(t, err)

		db, err := createDB(dir)
		require.NoError(t, err)

		test(t, db)

		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dir))
	}
}

func TestErrCollectionNotExist(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		query := q.NewQuery("myCollection")
		_, err := db.Count(query)
		require.Equal(t, c.ErrCollectionNotExist, err)

		_, err = db.FindById("myCollection", "objectId")
		require.Equal(t, c.ErrCollectionNotExist, err)

		_, err = db.FindAll(query)
		require.Equal(t, c.ErrCollectionNotExist, err)

		err = db.Update(query, nil)
		require.Equal(t, c.ErrCollectionNotExist, err)

		err = db.Delete(query)
		require.Equal(t, c.ErrCollectionNotExist, err)

		err = db.DeleteById("myCollection", "objectId")
		require.Equal(t, c.ErrCollectionNotExist, err)

		err = db.DropCollection("myCollection")
		require.Equal(t, c.ErrCollectionNotExist, err)
	})
}

func TestCreateCollectionAndDrop(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		err := db.CreateCollection("myCollection")
		require.NoError(t, err)
		has, err := db.HasCollection("myCollection")
		require.NoError(t, err)
		require.True(t, has)

		err = db.CreateCollection("myCollection")
		require.Equal(t, err, c.ErrCollectionExist)

		err = db.DropCollection("myCollection")
		require.NoError(t, err)

		has, err = db.HasCollection("myCollection")
		require.NoError(t, err)
		require.False(t, has)

		err = db.DropCollection("myOtherCollection")
		require.Equal(t, err, c.ErrCollectionNotExist)
	})
}

func TestInsertOneAndDelete(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		err := db.CreateCollection("myCollection")
		require.NoError(t, err)

		doc := d.NewDocument()
		doc.Set("hello", "clover")

		require.Empty(t, doc.ObjectId())
		docId, err := db.InsertOne("myCollection", doc)
		require.NoError(t, err)
		require.NotEmpty(t, docId)

		_, err = db.InsertOne("myCollection", doc)
		require.Equal(t, c.ErrDuplicateKey, err)

		doc, err = db.FindById("myCollection", docId)
		require.NoError(t, err)
		require.NotEmpty(t, doc.ObjectId(), docId)

		err = db.DeleteById("myCollection", docId)
		require.NoError(t, err)

		doc, err = db.FindById("myCollection", docId)
		require.NoError(t, err)
		require.Nil(t, doc)

		n, err := db.Count(q.NewQuery("myCollection"))
		require.NoError(t, err)
		require.Equal(t, n, 0)
	})
}

func TestInsert(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		err := db.CreateCollection("myCollection")
		require.NoError(t, err)

		doc := d.NewDocument()
		doc.Set("hello", "clover")

		require.NoError(t, db.Insert("myCollection", doc))
		require.Equal(t, db.Insert("myOtherCollection"), c.ErrCollectionNotExist)
	})
}

func TestSaveDocument(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		err := db.CreateCollection("myCollection")
		require.NoError(t, err)

		doc := d.NewDocument()
		doc.Set("hello", "clover")
		require.NoError(t, db.Save("myCollection", doc))

		savedDoc, err := db.FindFirst(q.NewQuery("myCollection"))
		require.NoError(t, err)
		require.Equal(t, savedDoc, doc)

		id := doc.ObjectId()
		doc.Set("_id", id)
		doc.Set("hello", "clover-updated!")
		require.NoError(t, db.Save("myCollection", doc))

		n, err := db.Count(q.NewQuery("myCollection"))
		require.NoError(t, err)
		require.Equal(t, 1, n)

		docUpdated, err := db.FindById("myCollection", id)
		require.NoError(t, err)
		require.Equal(t, doc, docUpdated)
	})
}

func TestSaveDocumentObject(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		err := db.CreateCollection("myCollection")
		require.NoError(t, err)

		data := struct {
			Id    string `clover:"_id"`
			Hello string `clover:"hello"`
		}{
			Hello: "clover",
		}
		require.NoError(t, db.Save("myCollection", data))

		savedDoc, err := db.FindFirst(q.NewQuery("myCollection"))
		require.NoError(t, err)
		require.Equal(t, savedDoc.Get("hello"), "clover")

		id := savedDoc.ObjectId()
		data.Id = id
		data.Hello = "clover-updated!"
		require.NoError(t, db.Save("myCollection", data))

		n, err := db.Count(q.NewQuery("myCollection"))
		require.NoError(t, err)
		require.Equal(t, 1, n)

		docUpdated, err := db.FindById("myCollection", id)
		require.NoError(t, err)
		require.Equal(t, docUpdated.Get("hello"), "clover-updated!")
	})
}

func TestInsertAndGet(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		err := db.CreateCollection("myCollection")
		require.NoError(t, err)

		nInserts := 100
		docs := make([]*d.Document, 0, nInserts)
		for i := 0; i < nInserts; i++ {
			doc := d.NewDocument()
			doc.Set("myField", i)
			docs = append(docs, doc)
		}

		require.NoError(t, db.Insert("myCollection", docs...))
		n, err := db.Count(q.NewQuery("myCollection"))
		require.NoError(t, err)
		require.Equal(t, nInserts, n)

		query := q.NewQuery("myCollection").MatchFunc(func(doc *d.Document) bool {
			require.True(t, doc.Has("myField"))

			v, _ := doc.Get("myField").(int64)
			return int(v)%2 == 0
		})
		n, err = db.Count(query)

		require.NoError(t, err)

		require.Equal(t, nInserts/2, n)
	})
}

func loadFromJson(db *c.DB, filename string, model interface{}) error {
	var objects []interface{}

	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &objects); err != nil {
		return err
	}

	collectionName := strings.TrimSuffix(filepath.Base(filename), ".json")
	if err := db.CreateCollection(collectionName); err != nil {
		return err
	}

	docs := make([]*d.Document, 0)
	for _, obj := range objects {
		data, err := json.Marshal(obj)
		if err != nil {
			return err
		}

		var fields interface{}
		if model == nil {
			fields = make(map[string]interface{})
		} else {
			fields = reflect.New(reflect.TypeOf(model)).Interface()
		}

		if err := json.Unmarshal(data, &fields); err != nil {
			return err
		}

		doc := d.NewDocumentOf(fields)
		docs = append(docs, doc)
	}
	return db.Insert(collectionName, docs...)
}

func TestUpdateCollection(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		criteria := q.Field("completed").Eq(true)
		updates := make(map[string]interface{})
		updates["completed"] = false

		docs, err := db.FindAll(q.NewQuery("todos").Where(criteria))
		require.NoError(t, err)

		err = db.Update(q.NewQuery("todos").Where(criteria), updates)
		require.NoError(t, err)

		n, err := db.Count(q.NewQuery("todos").Where(criteria))
		require.NoError(t, err)
		require.Equal(t, n, 0)

		for _, doc := range docs {
			doc.Set("completed", false)
			updatedDoc, err := db.FindFirst(q.NewQuery("todos").Where(q.Field("id").Eq(doc.Get("id"))))
			require.NoError(t, err)
			require.Equal(t, doc, updatedDoc)
		}
	})
}

func TestUpdateById(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		doc, err := db.FindFirst(q.NewQuery("todos"))

		require.NoError(t, err)

		err = db.UpdateById("todos", "invalid-id", func(_ *d.Document) *d.Document {
			return d.NewDocument()
		})
		require.Equal(t, err, c.ErrDocumentNotExist)

		id := doc.ObjectId()
		completed := doc.Get("completed").(bool)

		err = db.UpdateById("todos", id, func(doc *d.Document) *d.Document {
			doc.Set("completed", !doc.Get("completed").(bool))
			return doc
		})
		require.NoError(t, err)

		doc, err = db.FindById("todos", id)
		require.NoError(t, err)

		require.Equal(t, !completed, doc.Get("completed").(bool))
	})
}

func TestReplaceById(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		doc, err := db.FindFirst(q.NewQuery("todos"))
		require.NoError(t, err)

		err = db.ReplaceById("todos", "invalid-id", doc)
		require.Error(t, err)

		id := doc.ObjectId()
		newDoc := d.NewDocument()
		newDoc.Set("hello", "clover")

		err = db.ReplaceById("todos", id, newDoc)
		require.Error(t, err)

		newDoc.Set("_id", id)
		err = db.ReplaceById("todos", id, newDoc)
		require.NoError(t, err)

		doc, err = db.FindById("todos", id)
		require.NoError(t, err)
		require.Equal(t, doc, newDoc)
	})
}

func TestInsertAndDelete(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		criteria := q.Field("completed").Eq(true)
		err := db.Delete(q.NewQuery("todos").Where(criteria))
		require.NoError(t, err)

		n, err := db.Count(q.NewQuery("todos"))
		require.NoError(t, err)

		m, err := db.Count(q.NewQuery("todos").Where(criteria.Not()))
		require.NoError(t, err)

		require.Equal(t, n, m)
	})
}

func TestOpenExisting(t *testing.T) {
	dir, err := os.MkdirTemp("", "clover-test")
	defer os.RemoveAll(dir)
	require.NoError(t, err)

	err = os.RemoveAll(dir)
	require.NoError(t, err)

	db, err := c.Open(dir)
	require.NoError(t, err)

	require.NoError(t, loadFromJson(db, todosPath, nil))
	require.NoError(t, db.Close())

	db, err = c.Open(dir)
	require.NoError(t, err)

	has, err := db.HasCollection("todos")
	require.NoError(t, err)
	require.True(t, has)

	rows, err := db.Count(q.NewQuery("todos"))
	require.NoError(t, err)
	require.Equal(t, 200, rows)
}

func TestReloadIndex(t *testing.T) {
	dir, err := os.MkdirTemp("", "clover-test")
	defer os.RemoveAll(dir)
	require.NoError(t, err)

	db, err := c.Open(dir)
	require.NoError(t, err)

	db.CreateCollection("myCollection")

	doc := d.NewDocument()
	doc.Set("hello", "clover!")

	docId, err := db.InsertOne("myCollection", doc)
	require.NoError(t, err)

	doc, err = db.FindById("myCollection", docId)
	require.NoError(t, err)
	require.Equal(t, docId, doc.ObjectId())

	require.NoError(t, db.Close())

	db, err = c.Open(dir)
	require.NoError(t, err)

	doc, err = db.FindById("myCollection", docId)
	require.NoError(t, err)
	require.Equal(t, docId, doc.ObjectId())

	require.NoError(t, db.Close())
}

func TestInvalidCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		_, err := db.FindAll(q.NewQuery("todos").Where(q.Field("completed").Eq(func() {})))
		require.Error(t, err)

		_, err = db.FindAll(q.NewQuery("todos").Where(q.Field("completed").Neq(func() {})))
		require.Error(t, err)

		_, err = db.FindAll(q.NewQuery("todos").Where(q.Field("completed").Lt(func() {})))
		require.Error(t, err)

		_, err = db.FindAll(q.NewQuery("todos").Where(q.Field("completed").LtEq(func() {})))
		require.Error(t, err)

		_, err = db.FindAll(q.NewQuery("todos").Where(q.Field("completed").Gt(func() {})))
		require.Error(t, err)

		_, err = db.FindAll(q.NewQuery("todos").Where(q.Field("completed").GtEq(func() {})))
		require.Error(t, err)
	})
}

func TestExistsCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		n, err := db.Count(q.NewQuery("todos").Where(q.Field("completed_date").Exists()))
		require.NoError(t, err)
		m, err := db.Count(q.NewQuery("todos").Where(q.Field("completed").IsTrue()))
		require.NoError(t, err)
		require.Equal(t, n, m)
	})
}

func TestNotExistsCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		n, err := db.Count(q.NewQuery("todos").Where(q.Field("completed_date").NotExists()))
		require.NoError(t, err)

		m, err := db.Count(q.NewQuery("todos").Where(q.Field("completed").IsFalse()))
		require.NoError(t, err)

		require.Equal(t, n, m)
	})
}

func TestIsNil(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, nil))

		n, err := db.Count(q.NewQuery("todos").Where(q.Field("notes").IsNil()))
		require.NoError(t, err)
		require.Equal(t, n, 1)
	})
}

func TestIsTrue(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		n, err := db.Count(q.NewQuery("todos").Where(q.Field("completed").Eq(true)))
		require.NoError(t, err)

		m, err := db.Count(q.NewQuery("todos").Where(q.Field("completed").IsTrue()))
		require.NoError(t, err)

		require.Equal(t, n, m)
	})
}

func TestIsFalse(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		n, err := db.Count(q.NewQuery("todos").Where(q.Field("completed").Eq(false)))
		require.NoError(t, err)

		m, err := db.Count(q.NewQuery("todos").Where(q.Field("completed").IsFalse()))
		require.NoError(t, err)

		require.Equal(t, n, m)
	})
}

func TestIsNilOrNotExist(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		n, err := db.Count(q.NewQuery("todos").Where(q.Field("completed_date").IsNilOrNotExists()))
		require.NoError(t, err)
		m, err := db.Count(q.NewQuery("todos").Where(q.Field("completed").IsFalse()))
		require.NoError(t, err)

		require.Equal(t, n, m)
	})
}

func TestEqCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		docs, err := db.FindAll(q.NewQuery("todos").Where(q.Field("completed").Eq(true)))
		require.NoError(t, err)
		require.Greater(t, len(docs), 0)

		for _, doc := range docs {
			require.NotNil(t, doc.Get("completed"))
			require.Equal(t, doc.Get("completed"), true)
		}
	})
}

func TestBoolCompare(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		n, err := db.Count(q.NewQuery("todos").Where(q.Field("completed").Eq(true)))
		require.NoError(t, err)
		m, err := db.Count(q.NewQuery("todos").Where(q.Field("completed").Gt(false)))
		require.NoError(t, err)

		require.Equal(t, n, m)
	})
}

func TestCompareWithWrongType(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		n, err := db.Count(q.NewQuery("todos").Where(q.Field("completed").Gt("true")))
		require.NoError(t, err)
		require.Equal(t, 200, n)

		n, err = db.Count(q.NewQuery("todos").Where(q.Field("completed").GtEq("true")))
		require.NoError(t, err)
		require.Equal(t, 200, n)

		n, err = db.Count(q.NewQuery("todos").Where(q.Field("completed").Lt("true")))
		require.NoError(t, err)
		require.Equal(t, 0, n)

		n, err = db.Count(q.NewQuery("todos").Where(q.Field("completed").LtEq("true")))
		require.NoError(t, err)
		require.Equal(t, 0, n)
	})
}

func TestCompareString(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, airlinesPath, nil))

		docs, err := db.FindAll(q.NewQuery("airlines").Where(q.Field("Airport.Code").Gt("CLT")))
		require.NoError(t, err)
		require.Greater(t, len(docs), 0)

		for _, doc := range docs {
			code := doc.Get("Airport.Code").(string)
			require.NotNil(t, code)
			require.Greater(t, code, "CLT")
		}
	})
}

func TestEqCriteriaWithDifferentTypes(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		count1, err := db.Count(q.NewQuery("todos").Where(q.Field("userId").Eq(1)))
		require.NoError(t, err)

		count2, err := db.Count(q.NewQuery("todos").Where(q.Field("userId").Eq(int8(1))))
		require.NoError(t, err)

		count3, err := db.Count(q.NewQuery("todos").Where(q.Field("userId").Eq(int16(1))))
		require.NoError(t, err)

		count4, err := db.Count(q.NewQuery("todos").Where(q.Field("userId").Eq(int32(1))))
		require.NoError(t, err)

		count5, err := db.Count(q.NewQuery("todos").Where(q.Field("userId").Eq(int64(1))))
		require.NoError(t, err)

		count6, err := db.Count(q.NewQuery("todos").Where(q.Field("userId").Eq(uint(1))))
		require.NoError(t, err)

		count7, err := db.Count(q.NewQuery("todos").Where(q.Field("userId").Eq(uint8(1))))
		require.NoError(t, err)

		count8, err := db.Count(q.NewQuery("todos").Where(q.Field("userId").Eq(uint16(1))))
		require.NoError(t, err)

		count9, err := db.Count(q.NewQuery("todos").Where(q.Field("userId").Eq(uint32(1))))
		require.NoError(t, err)

		count10, err := db.Count(q.NewQuery("todos").Where(q.Field("userId").Eq(uint64(1))))
		require.NoError(t, err)

		count11, err := db.Count(q.NewQuery("todos").Where(q.Field("userId").Eq(float32(1))))
		require.NoError(t, err)

		count12, err := db.Count(q.NewQuery("todos").Where(q.Field("userId").Eq(float64(1))))
		require.NoError(t, err)

		require.Greater(t, count1, 0)

		require.Equal(t, count1, count2)
		require.Equal(t, count1, count3)
		require.Equal(t, count1, count4)
		require.Equal(t, count1, count5)
		require.Equal(t, count1, count6)
		require.Equal(t, count1, count7)
		require.Equal(t, count1, count8)
		require.Equal(t, count1, count9)
		require.Equal(t, count1, count10)
		require.Equal(t, count1, count11)
		require.Equal(t, count1, count12)
	})
}

func TestCompareUint64(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		docs, err := db.FindAll(q.NewQuery("todos").Where(q.Field("id").Gt(uint64(4))))
		require.NoError(t, err)

		for _, doc := range docs {
			require.Greater(t, doc.Get("id"), uint64(4))
		}
	})
}

func TestNeqCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		docs, err := db.FindAll(q.NewQuery("todos").Where(q.Field("userId").Neq(7)))
		require.NoError(t, err)
		require.Greater(t, len(docs), 0)

		for _, doc := range docs {
			require.NotNil(t, doc.Get("userId"))
			require.NotEqual(t, doc.Get("userId"), float64(7))
		}
	})
}

func TestGtCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		docs, err := db.FindAll(q.NewQuery("todos").Where(q.Field("userId").Gt(4)))
		require.NoError(t, err)
		require.Greater(t, len(docs), 0)

		for _, doc := range docs {
			require.NotNil(t, doc.Get("userId"))
			require.Greater(t, doc.Get("userId"), int64(4))
		}
	})
}

func TestGtEqCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		docs, err := db.FindAll(q.NewQuery("todos").Where(q.Field("userId").GtEq(4)))
		require.NoError(t, err)
		require.Greater(t, len(docs), 0)

		for _, doc := range docs {
			require.NotNil(t, doc.Get("userId"))
			require.GreaterOrEqual(t, doc.Get("userId"), int64(4))
		}
	})
}

func TestLtCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		docs, err := db.FindAll(q.NewQuery("todos").Where(q.Field("userId").Lt(4)))
		require.NoError(t, err)

		require.Greater(t, len(docs), 0)
		for _, doc := range docs {
			require.NotNil(t, doc.Get("userId"))
			require.Less(t, doc.Get("userId"), int64(4))
		}
	})
}

func TestLtEqCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		docs, err := db.FindAll(q.NewQuery("todos").Where(q.Field("userId").LtEq(4)))
		require.NoError(t, err)

		require.Greater(t, len(docs), 0)

		for _, doc := range docs {
			require.NotNil(t, doc.Get("userId"))
			require.LessOrEqual(t, doc.Get("userId"), int64(4))
		}
	})
}

func TestInCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		docs, err := db.FindAll(q.NewQuery("todos").Where(q.Field("userId").In(5, 8)))
		require.NoError(t, err)

		require.Greater(t, len(docs), 0)

		for _, doc := range docs {
			userId := doc.Get("userId")
			require.NotNil(t, userId)

			if userId != int64(5) && userId != int64(8) {
				require.Fail(t, "userId is not in the correct range")
			}
		}

		criteria := q.Field("userId").In(q.Field("id"), 6)
		docs, err = db.FindAll(q.NewQuery("todos").Where(criteria))
		require.NoError(t, err)

		require.Greater(t, len(docs), 0)
		for _, doc := range docs {
			userId := doc.Get("userId").(int64)
			id := doc.Get("id").(uint64)
			require.True(t, uint64(userId) == id || userId == 6)
		}
	})
}

func TestContainsCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		err := db.CreateCollection("myCollection")
		require.NoError(t, err)

		vals := [][]int{
			{
				1, 2, 4,
			},
			{
				5, 6, 7,
			},
			{
				4, 10, 20,
			},
		}
		docs := make([]*d.Document, 0, 3)
		for _, val := range vals {
			doc := d.NewDocument()
			doc.Set("myField", val)
			docs = append(docs, doc)
		}
		require.NoError(t, db.Insert("myCollection", docs...))

		testElement := 4
		docs, err = db.FindAll(q.NewQuery("myCollection").Where(q.Field("myField").Contains(testElement)))
		require.NoError(t, err)

		require.Equal(t, 2, len(docs))

		for _, doc := range docs {
			myField := doc.Get("myField").([]interface{})
			require.NotNil(t, myField)

			found := false
			for _, elem := range myField {
				if elem.(int64) == 4 {
					found = true
					break
				}
			}

			require.True(t, found, fmt.Sprintf("myField does not contain element %d\n", testElement))

		}
	})
}

func TestAndCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		criteria := q.Field("completed").Eq(true).And(q.Field("userId").Gt(2))
		docs, err := db.FindAll(q.NewQuery("todos").Where(criteria))
		require.NoError(t, err)

		require.Greater(t, len(docs), 0)
		for _, doc := range docs {
			require.NotNil(t, doc.Get("completed"))
			require.NotNil(t, doc.Get("userId"))
			require.Equal(t, doc.Get("completed"), true)
			require.Greater(t, doc.Get("userId"), int64(2))
		}
	})
}

func TestOrCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, airlinesPath, nil))

		criteria := q.Field("Statistics.Flights.Cancelled").Gt(100).Or(q.Field("Statistics.Flights.Total").GtEq(1000))
		docs, err := db.FindAll(q.NewQuery("airlines").Where(criteria))
		require.NoError(t, err)

		require.Greater(t, len(docs), 0)

		for _, doc := range docs {
			cancelled := doc.Get("Statistics.Flights.Cancelled").(float64)
			total := doc.Get("Statistics.Flights.Total").(float64)

			if cancelled <= 100 && total < 1000 {
				require.Fail(t, "or criteria not satisfied")
			}
		}
	})
}

func TestLikeCriteria(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		likeCriteria := q.Field("title").Like(".*est.*")
		docs, err := db.FindAll(q.NewQuery("todos").Where(likeCriteria))

		require.NoError(t, err)

		n := len(docs)
		for _, doc := range docs {
			s, isString := doc.Get("title").(string)
			require.True(t, isString)
			require.True(t, strings.Contains(s, "est"))
		}

		docs, err = db.FindAll(q.NewQuery("todos").Where(likeCriteria.Not()))
		require.NoError(t, err)

		m := len(docs)
		for _, doc := range docs {
			s, isString := doc.Get("title").(string)
			require.True(t, isString)
			require.False(t, strings.Contains(s, "est"))
		}

		total, err := db.Count(q.NewQuery("todos"))
		require.NoError(t, err)
		require.Equal(t, total, n+m)
	})
}

func TestTimeRangeQuery(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		start := time.Date(2020, 06, 10, 0, 0, 0, 0, time.UTC)
		end := time.Date(2021, 03, 20, 0, 0, 0, 0, time.UTC)

		allDocs, err := db.FindAll(q.NewQuery("todos"))
		require.NoError(t, err)

		n := 0
		for _, doc := range allDocs {
			date := doc.Get("completed_date")
			if date == nil {
				continue
			}
			require.IsType(t, time.Time{}, date)

			completedDate := date.(time.Time)
			if completedDate.Unix() >= start.Unix() && completedDate.Unix() <= end.Unix() {
				n++
			}
		}

		docs, err := db.FindAll(q.NewQuery("todos").Where(q.Field("completed_date").GtEq(start).And(q.Field("completed_date").Lt(end))))
		require.NoError(t, err)
		require.Len(t, docs, n)

		for _, doc := range docs {
			date := doc.Get("completed_date").(time.Time)
			require.Positive(t, date.Sub(start))
			require.Negative(t, date.Sub(end))
		}
	})
}

func TestLimit(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		n, err := db.Count(q.NewQuery("todos"))
		require.NoError(t, err)

		for m := n / 2; m >= 1; m = m / 2 {
			k, err := db.Count(q.NewQuery("todos").Limit(m))
			require.NoError(t, err)
			require.Equal(t, m, k)
		}
	})
}

func TestSkip(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		allDocs, err := db.FindAll(q.NewQuery("todos"))
		require.NoError(t, err)
		require.Len(t, allDocs, 200)

		skipDocs, err := db.FindAll(q.NewQuery("todos").Skip(100))
		require.NoError(t, err)

		require.Len(t, skipDocs, 100)
		require.Equal(t, allDocs[100:], skipDocs)
	})
}

func TestSkipWithSort(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		sortOption := q.SortOption{
			Field:     "id",
			Direction: 1,
		}
		allDocs, err := db.FindAll(q.NewQuery("todos").Sort(sortOption))
		require.NoError(t, err)
		require.Len(t, allDocs, 200)

		skipDocs, err := db.FindAll(q.NewQuery("todos").Sort(sortOption).Skip(100))
		require.NoError(t, err)

		require.Len(t, skipDocs, 100)
		require.Equal(t, allDocs[100:], skipDocs)
	})
}

func TestLimitAndSkip(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		sortOption := q.SortOption{
			Field:     "id",
			Direction: 1,
		}
		allDocs, err := db.FindAll(q.NewQuery("todos").Sort(sortOption))
		require.NoError(t, err)
		require.Len(t, allDocs, 200)

		docs, err := db.FindAll(q.NewQuery("todos").Sort(sortOption).Skip(100).Limit(50))
		require.NoError(t, err)

		require.Len(t, docs, 50)
		require.Equal(t, allDocs[100:150], docs)
	})
}

func TestFindFirst(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		doc, err := db.FindFirst(q.NewQuery("todos").Where(q.Field("completed").Eq(true)))
		require.NoError(t, err)
		require.NotNil(t, doc)

		require.Equal(t, doc.Get("completed"), true)
	})
}

func TestExists(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		exists, err := db.Exists(q.NewQuery("todos").Where(q.Field("completed").IsTrue()))
		require.NoError(t, err)
		require.True(t, exists)

		exists, err = db.Exists(q.NewQuery("todos").Where(q.Field("userId").Eq(100)))
		require.NoError(t, err)
		require.False(t, exists)
	})
}

func TestForEach(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		n, err := db.Count(q.NewQuery("todos").Where(q.Field("completed").IsTrue()))
		require.NoError(t, err)

		m := 0
		err = db.ForEach(q.NewQuery("todos").Where(q.Field("completed").IsTrue()), func(doc *d.Document) bool {
			m++
			return true
		})
		require.NoError(t, err)
		require.Equal(t, n, m)
	})
}

func TestSort(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, airlinesPath, nil))

		sortOpts := []q.SortOption{
			{Field: "Statistics.Flights.Total", Direction: 1},
			{Field: "Statistics.Flights.Cancelled", Direction: -1},
		}

		docs, err := db.FindAll(q.NewQuery("airlines").Sort(sortOpts...))
		require.NoError(t, err)

		totals := make([]int, 0, len(docs))
		cancelled := make([]int, 0, len(docs))
		for _, doc := range docs {
			if doc.Has("Statistics.Flights.Total") {
				total := int(doc.Get("Statistics.Flights.Total").(float64))
				deleted := int(doc.Get("Statistics.Flights.Cancelled").(float64))

				totals = append(totals, total)
				cancelled = append(cancelled, deleted)
			}
		}

		sorted := sort.SliceIsSorted(docs, func(i, j int) bool {
			if totals[i] != totals[j] {
				return totals[i] < totals[j]
			}
			return cancelled[i] > cancelled[j]
		})
		require.True(t, sorted)
	})
}

func TestSortWithIndex(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, airlinesPath, nil))

		err := db.CreateIndex("airlines", "Statistics.Flights.Total")
		require.NoError(t, err)

		n, err := db.Count(q.NewQuery("airlines"))
		require.NoError(t, err)

		docs, err := db.FindAll(q.NewQuery("airlines").Sort(q.SortOption{Field: "Statistics.Flights.Total", Direction: -1}))
		require.NoError(t, err)
		require.Equal(t, n, len(docs))

		sorted := sort.SliceIsSorted(docs, func(i, j int) bool {
			return docs[j].Get("Statistics.Flights.Total").(float64) < docs[i].Get("Statistics.Flights.Total").(float64)
		})
		require.True(t, sorted)
	})
}

func TestForEachStop(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		n := 0
		err := db.ForEach(q.NewQuery("todos"), func(doc *d.Document) bool {
			if n < 100 {
				n++
				return true
			}
			return false
		})
		require.NoError(t, err)
		require.Equal(t, n, 100)
	})
}

func TestListCollections(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		collections, err := db.ListCollections()
		require.NoError(t, err)
		require.Equal(t, 1, len(collections))

		err = db.CreateCollection("test1")
		require.NoError(t, err)

		collections, err = db.ListCollections()
		require.NoError(t, err)
		require.Equal(t, 2, len(collections))

		err = db.DropCollection("test1")
		require.NoError(t, err)

		err = db.CreateCollection("c1")
		require.NoError(t, err)
		err = db.CreateCollection("c2")
		require.NoError(t, err)
		err = db.CreateCollection("c3")
		require.NoError(t, err)

		collections, err = db.ListCollections()
		require.NoError(t, err)
		require.Equal(t, 4, len(collections))

		err = db.DropCollection("c1")
		require.NoError(t, err)
		err = db.DropCollection("c2")
		require.NoError(t, err)
		err = db.DropCollection("c3")
		require.NoError(t, err)
		err = db.DropCollection("todos")
		require.NoError(t, err)

		collections, err = db.ListCollections()
		require.NoError(t, err)
		require.Equal(t, 0, len(collections))
	})
}

func TestExportAndImportCollection(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		exportPath, err := os.MkdirTemp("", "export-dir")
		require.NoError(t, err)
		defer os.RemoveAll(exportPath)

		exportFilePath := exportPath + "todos.json"
		err = db.ExportCollection("todos", exportFilePath)
		require.NoError(t, err)

		err = db.ImportCollection("todos-copy", exportFilePath)
		require.NoError(t, err)

		docs, err := db.FindAll(q.NewQuery("todos").Sort())
		require.NoError(t, err)

		importDocs, err := db.FindAll(q.NewQuery("todos-copy").Sort())
		require.NoError(t, err)

		require.Equal(t, len(docs), len(importDocs))

		for i := 0; i < len(docs); i++ {
			todo1 := &TodoModel{}
			todo2 := &TodoModel{}

			require.NoError(t, docs[i].Unmarshal(todo1))
			require.NoError(t, importDocs[i].Unmarshal(todo2))

			require.Equal(t, todo1, todo2)
		}
	})
}

func TestSliceCompare(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, nil))

		allDocs, err := db.FindAll(q.NewQuery("todos"))
		require.NoError(t, err)

		require.NoError(t, db.CreateCollection("todos.copy"))

		for _, doc := range allDocs {
			title, _ := doc.Get("title").(string)
			if title != "" {
				s := make([]int, len(title))
				for i := 0; i < len(title); i++ {
					s[i] = int(title[i])
				}
				doc.Set("title", s)
			}
		}
		err = db.Insert("todos.copy", allDocs...)
		require.NoError(t, err)

		sort1, err := db.FindAll(q.NewQuery("todos").Sort(q.SortOption{Field: "title"}))
		require.NoError(t, err)

		sort2, err := db.FindAll(q.NewQuery("todos.copy").Sort(q.SortOption{Field: "title"}))
		require.NoError(t, err)

		require.Equal(t, len(sort1), len(sort2))

		for i := 0; i < len(sort1); i++ {
			doc1 := sort1[i]
			doc2 := sort2[i]

			title := ""
			sTitle := doc2.Get("title").([]interface{})
			for j := 0; j < len(sTitle); j++ {
				title += string(byte(sTitle[j].(int64)))
			}
			require.Equal(t, title, doc1.Get("title"))
		}
	})
}

func TestCompareDocumentFields(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, airlinesPath, nil))

		criteria := q.Field("Statistics.Flights.Diverted").Gt(q.Field("Statistics.Flights.Cancelled"))
		docs, err := db.FindAll(q.NewQuery("airlines").Where(criteria))
		require.NoError(t, err)

		require.Greater(t, len(docs), 0)
		for _, doc := range docs {
			diverted := doc.Get("Statistics.Flights.Diverted").(float64)
			cancelled := doc.Get("Statistics.Flights.Cancelled").(float64)
			require.Greater(t, diverted, cancelled)
		}

		//alternative syntax using $
		criteria = q.Field("Statistics.Flights.Diverted").Gt("$Statistics.Flights.Cancelled")
		docs, err = db.FindAll(q.NewQuery("airlines").Where(criteria))
		require.NoError(t, err)

		require.Greater(t, len(docs), 0)
		for _, doc := range docs {
			diverted := doc.Get("Statistics.Flights.Diverted").(float64)
			cancelled := doc.Get("Statistics.Flights.Cancelled").(float64)
			require.Greater(t, diverted, cancelled)
		}
	})
}

func testIndexedQuery(t *testing.T, db *c.DB, criteria q.Criteria, collection, field string) {
	allDocs, err := db.FindAll(q.NewQuery(collection).Where(criteria).Sort())
	require.NoError(t, err)

	err = db.DropIndex(collection, field)
	if !errors.Is(err, c.ErrIndexNotExist) {
		require.NoError(t, err)
	}

	err = db.CreateIndex(collection, field)
	require.NoError(t, err)

	indexAllDocs, err := db.FindAll(q.NewQuery(collection).Where(criteria).Sort())
	require.NoError(t, err)
	require.Len(t, indexAllDocs, len(allDocs))

	for i := 0; i < len(indexAllDocs); i++ {
		require.Equal(t, allDocs[i], indexAllDocs[i])
	}
}

func TestCreateIndex(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.Equal(t, c.ErrCollectionNotExist, db.CreateIndex("collection", "field"))
		require.NoError(t, db.CreateCollection("collection"))

		require.Equal(t, c.ErrCollectionNotExist, db.CreateIndex("coll", "field"))

		has, err := db.HasIndex("collection", "field")
		require.NoError(t, err)
		require.False(t, has)

		require.NoError(t, db.CreateIndex("collection", "field"))

		has, err = db.HasIndex("collection", "field")
		require.NoError(t, err)
		require.True(t, has)

		indexes, err := db.ListIndexes("collection")
		require.NoError(t, err)

		require.Equal(t, []index.Info{{Field: "field", Type: index.SingleField}}, indexes)
	})
}

func TestIndex(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		criteria := q.Field("userId").Gt(5).And(q.Field("userId").LtEq(10))
		testIndexedQuery(t, db, criteria, "todos", "userId")

		criteria = q.Field("userId").GtEq(5).And(q.Field("userId").LtEq(8))
		testIndexedQuery(t, db, criteria, "todos", "userId")

		criteria = q.Field("userId").Gt(5)
		testIndexedQuery(t, db, criteria, "todos", "userId")

		criteria = q.Field("userId").GtEq(5)
		testIndexedQuery(t, db, criteria, "todos", "userId")
	})
}

func TestIndexNested(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, airlinesPath, nil))

		criteria := q.Field("Statistics.Flights.Cancelled").Gt(100).And(q.Field("Statistics.Flights.Cancelled").Lt(200))
		testIndexedQuery(t, db, criteria, "airlines", "Statistics.Flights.Cancelled")
	})
}

func TestIndexObjectField(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, airlinesPath, nil))

		criteria := q.Field("Statistics.Flights").Gt(map[string]interface{}{
			"Cancelled": float64(106),
		}).And(q.Field("Statistics.Flights").LtEq(map[string]interface{}{
			"Cancelled": float64(250),
		}))
		testIndexedQuery(t, db, criteria, "airlines", "Statistics.Flights")
	})
}

func TestIndexWithMixedTypes(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		err := db.CreateCollection("test")
		require.NoError(t, err)

		for i := 0; i < 1000; i++ {
			doc := d.NewDocument()

			var value interface{}
			typeId := rand.Intn(6)
			switch typeId {
			case 0:
				value = rand.Intn(2) == 1
			case 1:
				value = rand.Intn(1000)
			case 2:
				value = rand.Float64() * 1000
			case 3:
				value = time.Now()
			case 4:
				value = nil
			case 5:
				value = gofakeit.Map()
			}

			doc.Set("myField", value)
			require.NoError(t, db.Insert("test", doc))
		}

		criteria := q.Field("myField").Lt(true)
		testIndexedQuery(t, db, criteria, "test", "myField")

		criteria = q.Field("myField").Gt(100.10)
		testIndexedQuery(t, db, criteria, "test", "myField")

		criteria = q.Field("myField").Eq(nil)
		testIndexedQuery(t, db, criteria, "test", "myField")
	})
}

func TestIndexUpdate(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, airlinesPath, nil))

		criteria := q.Field("Statistics.Flights.Cancelled").Gt(100).And(q.Field("Statistics.Flights.Cancelled").Lt(200))

		err := db.CreateIndex("airlines", "Statistics.Flights.Cancelled")
		require.NoError(t, err)

		n, err := db.Count(q.NewQuery("airlines").Where(criteria))
		require.NoError(t, err)

		err = db.Update(q.NewQuery("airlines").Where(criteria), map[string]interface{}{
			"Statistics.Flights.Cancelled": 99999999,
		})
		require.NoError(t, err)

		m, err := db.Count(q.NewQuery("airlines").Where(q.Field("Statistics.Flights.Cancelled").Eq(99999999)))
		require.NoError(t, err)

		require.Equal(t, n, m)
	})
}

func TestIndexDelete(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, airlinesPath, nil))

		criteria := q.Field("Statistics.Flights.Cancelled").Gt(100).And(q.Field("Statistics.Flights.Cancelled").Lt(200))

		n, err := db.Count(q.NewQuery("airlines").Where(criteria))
		require.NoError(t, err)

		require.Greater(t, n, 0)

		err = db.CreateIndex("airlines", "Statistics.Flights.Cancelled")
		require.NoError(t, err)

		err = db.Delete(q.NewQuery("airlines").Where(criteria))
		require.NoError(t, err)

		n, err = db.Count(q.NewQuery("airlines").Where(criteria))
		require.NoError(t, err)

		require.Equal(t, n, 0)
	})
}

func TestIndexQueryWithSort(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, airlinesPath, nil))

		criteria := q.Field("Statistics.Flights.Cancelled").Gt(100).And(q.Field("Statistics.Flights.Cancelled").Lt(200))
		query := q.NewQuery("airlines").Where(criteria).Sort(q.SortOption{Field: "Statistics.Flights.Cancelled", Direction: -1})
		docs, err := db.FindAll(query)
		require.NoError(t, err)

		require.NoError(t, db.CreateIndex("airlines", "Statistics.Flights.Cancelled"))

		indexDocs, err := db.FindAll(query)
		require.NoError(t, err)

		require.Equal(t, len(docs), len(indexDocs))

		for i := 0; i < len(docs); i++ {
			require.Equal(t, docs[i].Get("Statistics.Flights.Cancelled"), indexDocs[i].Get("Statistics.Flights.Cancelled"))
		}
	})
}

func TestPagedQueryUsingIndex(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		err := db.CreateCollection("test")
		require.NoError(t, err)
		err = db.CreateIndex("test", "timestamp")
		require.NoError(t, err)

		docs := make([]*d.Document, 0, 1024)

		var is time.Time
		n := 10003
		for i := 0; i < n; i++ {
			doc := d.NewDocument()

			// Ensure unique timestamps by adding a small time offset (in nanoseconds)
			doc.Set("timestamp", time.Now().Add(time.Duration(i)*time.Nanosecond))

			if len(docs) == 1024 {
				err := db.Insert("test", docs...)
				require.NoError(t, err)

				docs = make([]*d.Document, 0, 1024)
			}
			docs = append(docs, doc)

			if i == 100 {
				is = doc.Get("timestamp").(time.Time)
			}
		}

		if len(docs) > 0 {
			err := db.Insert("test", docs...)
			require.NoError(t, err)
		}

		sortOpt := q.SortOption{Field: "timestamp", Direction: -1}

		count := 0
		var lastDoc *d.Document = nil
		for {
			var instant time.Time
			if lastDoc == nil {
				instant = time.Now()
			} else {
				instant = lastDoc.Get("timestamp").(time.Time)
			}

			all, err := db.FindAll(q.NewQuery("test").Where(q.Field("timestamp").Lt(instant)).Sort(sortOpt).Limit(25))
			require.NoError(t, err)

			sorted := sort.SliceIsSorted(all, func(i, j int) bool {
				return all[j].Get("timestamp").(time.Time).Before(all[i].Get("timestamp").(time.Time))
			})
			require.True(t, sorted)

			if len(all) > 0 {
				lastDoc = all[len(all)-1]
			} else {
				break
			}
			count += len(all)
		}

		require.Equal(t, n, count)

		m, err := db.Count(q.NewQuery("test").Where(q.Field("timestamp").Gt(is)).Sort(sortOpt))
		require.NoError(t, err)

		require.Equal(t, m, n-101)
	})
}

func TestDeleteByIdWithIndex(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, airlinesPath, nil))

		criteria := q.Field("Statistics.Flights.Cancelled").Gt(100).And(q.Field("Statistics.Flights.Cancelled").Lt(200))
		n, err := db.Count(q.NewQuery("airlines").Where(criteria))
		require.NoError(t, err)

		require.Greater(t, n, 0)

		err = db.CreateIndex("airlines", "Statistics.Flights.Cancelled")
		require.NoError(t, err)

		err = db.ForEach(q.NewQuery("airlines").Where(criteria), func(doc *d.Document) bool {
			err := db.DeleteById("airlines", doc.ObjectId())
			require.NoError(t, err)
			return true
		})
		require.NoError(t, err)

		n, err = db.Count(q.NewQuery("airlines").Where(criteria))
		require.NoError(t, err)

		require.Equal(t, n, 0)
	})
}

func TestListIndexes(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, db.CreateCollection("test"))

		indexes, err := db.ListIndexes("test")
		require.NoError(t, err)
		require.Empty(t, indexes)

		require.NoError(t, db.CreateIndex("test", "index"))

		indexes, err = db.ListIndexes("test")
		require.NoError(t, err)
		require.Equal(t, []index.Info{{Field: "index", Type: index.SingleField}}, indexes)

		require.NoError(t, db.DropIndex("test", "index"))

		indexes, err = db.ListIndexes("test")
		require.NoError(t, err)
		require.Empty(t, indexes)
	})
}

func TestCreateCollectionByQuery(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, loadFromJson(db, todosPath, &TodoModel{}))

		newCollection := "todos_copy"
		criteria := q.Field("completed").Eq(true)
		err := db.CreateCollectionByQuery(newCollection, q.NewQuery("todos").Where(criteria))
		require.NoError(t, err)

		hasNewCollection, err := db.HasCollection(newCollection)
		require.True(t, hasNewCollection)
		require.NoError(t, err)

		docs, err := db.FindAll(q.NewQuery(newCollection))
		require.NoError(t, err)

		for _, doc := range docs {
			completed, _ := doc.Get("completed").(bool)
			require.True(t, completed)
		}
	})
}

/*
func TestInMemoryMode(t *testing.T) {
	db, err := c.Open("clover-db", c.InMemoryMode(true))
	require.NoError(t, err)

	require.NoError(t, db.CreateCollection("test"))
	has, err := db.HasCollection("test")
	require.NoError(t, err)
	require.True(t, has)

	require.NoError(t, db.Close())

	db, err = c.Open("clover-db", c.InMemoryMode(true))
	require.NoError(t, err)

	has, err = db.HasCollection("test")
	require.NoError(t, err)
	require.False(t, has)
}*/

/*
func TestExpiration(t *testing.T) {
	runCloverTest(t, func(t *testing.T, db *c.DB) {
		require.NoError(t, db.CreateCollection("test"))

		require.NoError(t, db.CreateIndex("test", "HasExpiration"))

		nInserts := 1000

		expiredDocuments := 0

		docs := make([]*d.Document, 0)
		expiresAt := time.Now().Add(time.Second * 5)
		for i := 0; i < nInserts; i++ {
			doc := d.NewDocument()
			if rand.Intn(2) == 0 {
				doc.SetExpiresAt(expiresAt)
				expiredDocuments++
				doc.Set("HasExpiration", true)
			} else {
				doc.Set("HasExpiration", false)
			}

			docs = append(docs, doc)
		}

		require.NoError(t, db.Insert("test", docs...))

		time.Sleep(time.Second * 2)

		n, err := db.Count(q.NewQuery("test"))
		require.NoError(t, err)

		require.Equal(t, nInserts, n)

		time.Sleep(time.Second * 3)

		n, err = db.Count(q.NewQuery("test").Where(q.Field("HasExpiration").Eq(true)))
		require.NoError(t, err)

		require.Equal(t, 0, n)

		// run an insert with already expired documents
		expired := make([]*d.Document, 0)
		for _, doc := range docs {
			if doc.Get("HasExpiration").(bool) {
				expired = append(expired, doc)
			}
		}
		require.NoError(t, db.Insert("test", expired...))

		n, err = db.Count(q.NewQuery("test").Where(q.Field("HasExpiration").Eq(true)))
		require.NoError(t, err)

		require.Equal(t, 0, n)

		n, err = db.Count(q.NewQuery("test").Where(q.Field("HasExpiration").Eq(false)))
		require.NoError(t, err)

		require.Equal(t, nInserts-expiredDocuments, n)
	})
}
*/
