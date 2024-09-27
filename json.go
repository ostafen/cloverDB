package clover

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	d "github.com/ostafen/clover/v2/document"
	"github.com/ostafen/clover/v2/query"
)

// ExportCollection exports an existing collection to a JSON file.
func (db *DB) ExportCollection(collectionName string, exportPath string) (err error) {
	exists, err := db.HasCollection(collectionName)
	if err != nil {
		return err
	}
	if !exists {
		return ErrCollectionNotExist
	}
	q := query.NewQuery(collectionName)
	f, err := os.Create(exportPath)
	if err != nil {
		return err
	}
	defer f.Close()

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("internal error: %v", p)
		}
	}()
	isFirst := true
	err = db.ForEach(q, func(doc *d.Document) bool {
		jsonByte, err := json.Marshal(doc.AsMap())
		if err != nil {
			panic(err)
		}
		jsonString := string(jsonByte)
		if isFirst {
			isFirst = false
			jsonString = "[" + jsonString
		} else {
			jsonString = "," + jsonString
		}
		if _, err := f.WriteString(jsonString); err != nil {
			panic(err)
		}
		return true
	})
	if err == nil {
		_, err = f.WriteString("]")
	}
	return
}

// ImportCollection imports a collection from a JSON file.
func (db *DB) ImportCollection(collectionName string, importPath string) error {
	file, err := os.Open(importPath)
	if err != nil {
		return err
	}

	if err := db.CreateCollection(collectionName); err != nil {
		return err
	}

	reader := bufio.NewReader(file)
	jsonObjects := make([]*map[string]interface{}, 0)
	err = json.NewDecoder(reader).Decode(&jsonObjects)
	if err != nil {
		return err
	}

	docs := make([]*d.Document, 0)
	for _, doc := range jsonObjects {
		docs = append(docs, d.NewDocumentOf(*doc))
	}
	return db.Insert(collectionName, docs...)
}
