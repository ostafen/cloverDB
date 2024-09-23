package badger

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/ostafen/clover/v2/store"
)

type badgerStore struct {
	db     *badger.DB
	chWg   sync.WaitGroup
	chQuit chan struct{}

	gcInterval     time.Duration
	gcDiscardRatio float64
}

func (store *badgerStore) Begin(update bool) (store.Tx, error) {
	tx := store.db.NewTransaction(update)
	return &badgerTx{Txn: tx}, nil
}

func (store *badgerStore) Close() error {
	store.stopGC()
	return store.db.Close()
}

type badgerTx struct {
	*badger.Txn
}

func (tx *badgerTx) Set(key, value []byte) error {
	return tx.Txn.Set(key, value)
}

func getItemValue(item *badger.Item) ([]byte, error) {
	var value []byte
	err := item.Value(func(val []byte) error {
		value = val
		return nil
	})
	return value, err
}

func (tx *badgerTx) Get(key []byte) ([]byte, error) {
	item, err := tx.Txn.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}
	return getItemValue(item)
}

func (tx *badgerTx) Commit() error {
	return tx.Txn.Commit()
}

func (tx *badgerTx) Rollback() error {
	tx.Txn.Discard()
	return nil
}

func (tx *badgerTx) Cursor(forward bool) (store.Cursor, error) {
	opts := badger.DefaultIteratorOptions
	opts.Reverse = !forward
	return &badgerCursor{it: tx.NewIterator(opts)}, nil
}

type badgerCursor struct {
	it *badger.Iterator
}

func (cursor *badgerCursor) Seek(key []byte) error {
	cursor.it.Seek(key)
	return nil
}

func (cursor *badgerCursor) Next() {
	cursor.it.Next()
}

func (cursor *badgerCursor) Valid() bool {
	return cursor.it.Valid()
}

func (cursor *badgerCursor) Item() (store.Item, error) {
	item := cursor.it.Item()

	value, err := getItemValue(item)
	return store.Item{Key: item.Key(), Value: value}, err
}

func (cursor *badgerCursor) Close() error {
	cursor.it.Close()
	return nil
}

func Open(dir string) (store.Store, error) {
	return OpenWithOptions(badger.DefaultOptions(dir))
}

func OpenWithOptions(opts badger.Options) (store.Store, error) {
	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	dataStore := &badgerStore{
		db:             db,
		chQuit:         make(chan struct{}, 1),
		gcInterval:     time.Minute * 5,
		gcDiscardRatio: 0.5,
	}
	dataStore.startGC()
	return dataStore, nil
}

func (store *badgerStore) SetGCReclaimInterval(duration time.Duration) { store.gcInterval = duration }
func (store *badgerStore) SetGCDiscardRatio(ratio float64)             { store.gcDiscardRatio = ratio }

func (store *badgerStore) startGC() {
	store.chWg.Add(1)

	go func() {
		defer store.chWg.Done()

		ticker := time.NewTicker(store.gcInterval)
		defer ticker.Stop()

		for {
			select {
			case <-store.chQuit:
				return

			case <-ticker.C:
				err := store.db.RunValueLogGC(store.gcDiscardRatio)
				if err != nil && errors.Is(err, badger.ErrNoRewrite) {
					log.Printf("RunValueLogGC(): %s\n", err.Error())
				}
			}
		}
	}()
}

func (store *badgerStore) stopGC() {
	store.chQuit <- struct{}{}
	store.chWg.Wait()
	close(store.chQuit)
}
