package clover

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	badger "github.com/dgraph-io/badger/v3"
)

type docConsumer func(doc *Document) error

// StorageEngine represents the persistance layer and abstracts how collections are stored.
type StorageEngine interface {
	Open(path string) error
	Close() error

	CreateCollection(name string) error
	ListCollections() ([]string, error)
	DropCollection(name string) error
	HasCollection(name string) (bool, error)
	FindAll(q *Query) ([]*Document, error)
	FindById(collectionName string, id string) (*Document, error)
	DeleteById(collectionName string, id string) error
	IterateDocs(q *Query, consumer docConsumer) error
	Insert(collection string, docs ...*Document) error
	Save(collection string, doc *Document) error
	Update(q *Query, updateMap map[string]interface{}) error
	Delete(q *Query) error
}

var errStopIteration = errors.New("iteration stop")

type storageImpl struct {
	db *badger.DB
}

func newDefaultStorageImpl() *storageImpl {
	return &storageImpl{}
}

func (s *storageImpl) Open(path string) error {
	db, err := badger.Open(badger.DefaultOptions(path).WithLoggingLevel(badger.ERROR))
	s.db = db
	return err
}

func getCollectionKey(name string) string {
	return "coll:" + name
}

func (s *storageImpl) CreateCollection(name string) error {
	txn := s.db.NewTransaction(true)
	defer txn.Discard()

	ok, err := s.hasCollection(name, txn)
	if err != nil {
		return err
	}

	if ok {
		return ErrCollectionExist
	}

	if err := txn.Set([]byte(getCollectionKey(name)), []byte{0}); err != nil {
		return err
	}
	return txn.Commit()
}

func (s *storageImpl) DropCollection(name string) error {
	txn := s.db.NewTransaction(true)
	defer txn.Discard()

	if err := s.deleteAll(txn, name); err != nil {
		return err
	}

	if err := txn.Delete([]byte(getCollectionKey(name))); err != nil {
		return err
	}

	return txn.Commit()
}

func (s *storageImpl) FindAll(q *Query) ([]*Document, error) {
	docs := make([]*Document, 0)

	err := s.IterateDocs(q, func(doc *Document) error {
		if q.satisfy(doc) {
			docs = append(docs, doc)
		}
		return nil
	})
	return docs, err
}

func readDoc(data []byte) (*Document, error) {
	doc := NewDocument()
	return doc, json.Unmarshal(data, &doc.fields)
}

func (s *storageImpl) FindById(collectionName string, id string) (*Document, error) {
	txn := s.db.NewTransaction(false)
	defer txn.Discard()

	ok, err := s.hasCollection(collectionName, txn)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, ErrCollectionNotExist
	}

	item, err := txn.Get([]byte(getDocumentKey(collectionName, id)))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, nil
	}

	var doc *Document
	err = item.Value(func(data []byte) error {
		d, err := readDoc(data)
		doc = d
		return err
	})
	return doc, err
}

func getDocumentKey(collection string, id string) string {
	return collection + ":" + id
}

func (s *storageImpl) Insert(collection string, docs ...*Document) error {
	txn := s.db.NewTransaction(true)
	defer txn.Discard()

	ok, err := s.hasCollection(collection, txn)
	if err != nil {
		return err
	}

	if !ok {
		return ErrCollectionNotExist
	}

	for _, doc := range docs {
		data, err := json.Marshal(doc.fields)
		if err != nil {
			return err
		}

		if err := txn.Set([]byte(getDocumentKey(collection, doc.ObjectId())), data); err != nil {
			return err
		}
	}
	return txn.Commit()
}

func (s *storageImpl) Save(collection string, doc *Document) error {
	txn := s.db.NewTransaction(true)
	defer txn.Discard()

	ok, err := s.hasCollection(collection, txn)
	if err != nil {
		return err
	}

	if !ok {
		return ErrCollectionNotExist
	}

	data, err := json.Marshal(doc.fields)
	if err != nil {
		return err
	}

	if err := txn.Set([]byte(getDocumentKey(collection, doc.ObjectId())), data); err != nil {
		return err
	}

	return txn.Commit()
}

type docUpdater func(doc *Document) *Document

func (s *storageImpl) replaceDocs(txn *badger.Txn, q *Query, updater docUpdater) error {
	if txn == nil {
		txn = s.db.NewTransaction(true)
		defer txn.Discard()
	}

	ok, err := s.hasCollection(q.collection, txn)
	if err != nil {
		return err
	}

	if !ok {
		return ErrCollectionNotExist
	}

	docs := make([]*Document, 0)
	s.iterateDocs(txn, q, func(doc *Document) error {
		if q.satisfy(doc) {
			docs = append(docs, doc)
		}
		return nil
	})

	for _, doc := range docs {
		key := []byte(getDocumentKey(q.collection, doc.ObjectId()))
		if q.satisfy(doc) {
			newDoc := updater(doc)
			if newDoc == nil {
				if err := txn.Delete(key); err != nil {
					return err
				}
			} else {
				data, err := json.Marshal(newDoc.fields)
				if err != nil {
					return err
				}
				if err := txn.Set([]byte(getDocumentKey(q.collection, doc.ObjectId())), data); err != nil {
					return err
				}
			}
		}
	}
	return txn.Commit()
}

func (s *storageImpl) Update(q *Query, updateMap map[string]interface{}) error {
	return s.replaceDocs(nil, q, func(doc *Document) *Document {
		updateDoc := doc.Copy()
		for updateField, updateValue := range updateMap {
			updateDoc.Set(updateField, updateValue)
		}
		return updateDoc
	})
}

func (s *storageImpl) deleteAll(txn *badger.Txn, collName string) error {
	return s.replaceDocs(txn, &Query{collection: collName}, func(_ *Document) *Document {
		return nil
	})
}

func (s *storageImpl) Delete(q *Query) error {
	return s.replaceDocs(nil, q, func(_ *Document) *Document {
		return nil
	})
}

func (s *storageImpl) DeleteById(collName string, id string) error {
	txn := s.db.NewTransaction(true)
	defer txn.Discard()

	ok, err := s.hasCollection(collName, txn)
	if err != nil {
		return err
	}

	if !ok {
		return ErrCollectionNotExist
	}

	if err := txn.Delete([]byte(getDocumentKey(collName, id))); err != nil {
		return err
	}
	return txn.Commit()
}

func (s *storageImpl) Close() error {
	return s.db.Close()
}

func (s *storageImpl) hasCollection(name string, txn *badger.Txn) (bool, error) {
	_, err := txn.Get([]byte(getCollectionKey(name)))
	if errors.Is(err, badger.ErrKeyNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (s *storageImpl) HasCollection(name string) (bool, error) {
	txn := s.db.NewTransaction(false)
	defer txn.Discard()
	return s.hasCollection(name, txn)
}

func (s *storageImpl) iterateDocs(txn *badger.Txn, q *Query, consumer docConsumer) error {
	if txn == nil {
		txn = s.db.NewTransaction(false)
		defer txn.Discard()
	}

	ok, err := s.hasCollection(q.collection, txn)
	if err != nil {
		return err
	}

	if !ok {
		return ErrCollectionNotExist
	}

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()
	prefix := []byte(q.collection + ":")

	it.Seek(prefix)
	for i := 0; i < q.skip && it.ValidForPrefix(prefix); i++ { // skip the first q.skip documents
		it.Next()
	}

	for n := 0; (q.limit < 0 || n < q.limit) && it.ValidForPrefix(prefix); it.Next() {
		err := it.Item().Value(func(data []byte) error {
			doc, err := readDoc(data)
			if err != nil {
				return err
			}

			if q.satisfy(doc) {
				n++
				return consumer(doc)
			}
			return nil
		})

		// do not propagate iteration stop error
		if err == errStopIteration {
			return nil
		}

		if err != nil {
			return err
		}
	}
	return nil
}

func (s *storageImpl) iterateDocsSlice(q *Query, consumer docConsumer) error {
	allDocs := make([]*Document, 0)
	err := s.iterateDocs(nil, q.Skip(0).Limit(-1), func(doc *Document) error {
		allDocs = append(allDocs, doc)
		return nil
	})

	if err != nil {
		return err
	}

	sort.Slice(allDocs, func(i, j int) bool {
		return compareDocuments(allDocs[i], allDocs[j], q.sortOpts) < 0
	})

	docsToSkip := q.skip
	if len(allDocs) < q.skip {
		docsToSkip = len(allDocs)
	}
	allDocs = allDocs[docsToSkip:]

	if q.limit >= 0 && len(allDocs) > q.limit {
		allDocs = allDocs[:q.limit]
	}

	for _, doc := range allDocs {
		err = consumer(doc)
		if err == errStopIteration {
			return nil
		}

		if err != nil {
			return err
		}
	}
	return nil
}

func (s *storageImpl) IterateDocs(q *Query, consumer docConsumer) error {
	sortDocs := len(q.sortOpts) > 0
	if !sortDocs {
		return s.iterateDocs(nil, q, consumer)
	}
	return s.iterateDocsSlice(q, consumer)
}

func (s *storageImpl) ListCollections() ([]string, error) {
	txn := s.db.NewTransaction(true)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	collections := make([]string, 0)
	prefix := []byte("coll:")
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		key := string(item.Key())
		collectionName := strings.TrimPrefix(key, "coll:")
		collections = append(collections, collectionName)
	}

	return collections, nil
}
