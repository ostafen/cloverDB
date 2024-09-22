// Package bitcask ...
package bitcask

import (
	"git.mills.io/prologic/bitcask"
	"github.com/ostafen/clover/v2/store"
)

type bitcaskStore struct {
	db bitcask.DB
}

// Open ...
func Open(dir string) (store.Store, error) {
	db, err := bitcask.Open(dir, bitcask.WithMaxKeySize(256), bitcask.WithMaxValueSize(1<<20))
	if err != nil {
		return nil, err
	}
	return &bitcaskStore{db: db}, nil
}

func (store *bitcaskStore) Begin(update bool) (store.Tx, error) {
	return &bitcaskTx{store.db}, nil
}

func (store *bitcaskStore) Close() error {
	return store.db.Close()
}

type bitcaskTx struct {
	bitcask.DB
}

func (tx *bitcaskTx) Set(key, value []byte) error {
	return tx.DB.Put(key, value)
}

func (tx *bitcaskTx) Get(key []byte) ([]byte, error) {
	value, err := tx.DB.Get(key)
	// XXX: Clover assumes non-nil errors even for "Key Not Found" (which Bitcask considers an error)
	if err == bitcask.ErrKeyExpired {
		return nil, nil
	}
	return value, nil
}

func (tx *bitcaskTx) Delete(key []byte) error {
	return tx.DB.Delete(key)
}

func (tx *bitcaskTx) Cursor(forward bool) (store.Cursor, error) {
	var opts []bitcask.IteratorOption
	if !forward {
		opts = append(opts, bitcask.Reverse())
	}
	return &bitcaskCursor{Iterator: tx.DB.Iterator(opts...)}, nil
}

func (tx *bitcaskTx) Commit() error {
	return nil
}

func (tx *bitcaskTx) Rollback() error {
	return nil
}

type bitcaskCursor struct {
	bitcask.Iterator
	currItem *store.Item
}

func (c *bitcaskCursor) Seek(seek []byte) error {
	item, err := c.Iterator.SeekPrefix(seek)
	if err != nil || err == bitcask.ErrStopIteration {
		c.currItem = nil
		return nil
	}

	c.currItem = &store.Item{
		Key:   item.Key(),
		Value: item.Value(),
	}

	return nil
}

func (c *bitcaskCursor) Next() {
	item, err := c.Iterator.Next()
	if err != nil || err == bitcask.ErrStopIteration {
		c.currItem = nil
		return
	}

	c.currItem = &store.Item{
		Key:   item.Key(),
		Value: item.Value(),
	}
}

func (c *bitcaskCursor) Valid() bool {
	return c.currItem != nil && c.currItem.Key != nil && c.currItem.Value != nil
}

func (c *bitcaskCursor) Item() (store.Item, error) {
	item := c.currItem
	return *item, nil
}

func (c *bitcaskCursor) Close() error {
	return c.Iterator.Close()
}
