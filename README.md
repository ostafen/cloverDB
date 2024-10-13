<p align="center">
<img alt="CloverDB Logo" src=".github/logo.png#gh-light-mode-only" width="300px">
<img alt="CloverDB Logo" src=".github/logo-white.png#gh-dark-mode-only" width="300px">
</p>
<h2 align="center">Lightweight document-oriented NoSQL Database</h2>

[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)  
[![Go Reference](https://pkg.go.dev/badge/badge/github.com/ostafen/clover.svg)](https://pkg.go.dev/github.com/ostafen/clover)
[![Go Report Card](https://goreportcard.com/badge/github.com/ostafen/clover)](https://goreportcard.com/report/github.com/ostafen/clover)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Coverage Status](https://coveralls.io/repos/github/ostafen/clover/badge.svg)](https://coveralls.io/github/ostafen/clover)
[![Join the chat at https://gitter.im/cloverDB/community](https://badges.gitter.im/cloverDB/community.svg)](https://gitter.im/cloverDB/community?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

> 🇬🇧 English | [🇨🇳 简体中文](README-CN.md) | [🇪🇸 Spanish](README-ES.md)

**CloverDB** is a lightweight NoSQL database designed for being simple and easily maintainable, thanks to its small code base. It has been inspired by [tinyDB](https://github.com/msiemens/tinydb).

## Features

- Document oriented
- Written in pure Golang
- Simple and intuitive api
- Easily maintainable

## Why CloverDB?

**CloverDB** has been written for being easily maintainable. As such, it trades performance with simplicity, and is not intended to be an alternative to more performant databases such as **MongoDB** or **MySQL**.
However, there are projects where running a separate database server may result overkilled, and, for simple queries, network delay may be the major performance bottleneck.
For such scenarios, **CloverDB** may be a more suitable alternative.

## Database Layout

Previously, **CloverDB** relied on the [Badger](https://github.com/dgraph-io/badger) key-value store as a storage layer. However, **Badger** is not suitable for every scenario (for example, when the database size is a constraint). This is why, the storage layer of **CloverDB** has been abstracted through a set of interface types to work with any key-value store. At the moment, **CloverDB** can work with both **Badger** and [Bolt](https://github.com/etcd-io/bbolt) (by default **Bolt** is used).


## Installation
Make sure you have a working Go environment (Go 1.18 or higher is required). 
```shell
  GO111MODULE=on go get github.com/ostafen/clover/v2
```

## Databases and Collections

CloverDB stores data records as JSON documents, which are grouped together in collections. A database is made up of one or more collections.

### Database

To store documents inside collections, you have to open a Clover database using the `Open()` function.

```go
import (
  "log"
  "github.com/dgraph-io/badger/v4"
  c "github.com/ostafen/clover/v2"
  badgerstore "github.com/ostafen/clover/v2/store/badger"
)

...

// by default, Bolt will be used internally
db, _ := c.Open("clover-db")

// use OpenWithStore() if you want to select a different storage backend
store, _ := badgerstore.Open(badger.DefaultOptions("").WithInMemory(true)) // opens a badger in memory database
db, _ := c.OpenWithStore(store)

defer db.Close() // remember to close the db when you have done
```

### Collections

CloverDB stores documents inside collections. Collections are the **schemaless** equivalent of tables in relational databases. A collection is created by calling the `CreateCollection()` function on a database instance. New documents can be inserted using the `Insert()` or `InsertOne()` methods. Each document is uniquely identified by a **Version 4 UUID** stored in the **_id** special field and generated during insertion.

```go
import (
  c "github.com/ostafen/clover/v2"
  // import document module
  d "github.com/ostafen/clover/v2/document"
)

db, _ := c.Open("clover-db")
db.CreateCollection("myCollection") // create a new collection named "myCollection"

// insert a new document inside the collection
doc := d.NewDocument()
doc.Set("hello", "clover!")

// InsertOne returns the id of the inserted document
docId, _ := db.InsertOne("myCollection", doc)
fmt.Println(docId)
```

### Importing and Exporting Collections

CloverDB is capable of easily importing and exporting collections to JSON format regardless of the storage engine used.

```go
import (
//import query module
  q "github.com/ostafen/clover/v2/query"
)

// dump the content of the "todos" collection in a "todos.json" file
db.ExportCollection("todos", "todos.json")

...

// recover the todos collection from the exported json file
db.DropCollection("todos")
db.ImportCollection("todos", "todos.json")

docs, _ := db.FindAll(q.NewQuery("todos"))
for _, doc := range docs {
  log.Println(doc)
}
```

## Queries

CloverDB is equipped with a fluent and elegant API to query your data. A query is represented by the **Query** object, which allows to retrieve documents matching a given **criterion**. A query can be created by passing a valid collection name to the `Query()` method.

### Select All Documents in a Collection

The `FindAll()` method is used to retrieve all documents satisfying a given query.

```go
docs, _ := db.FindAll(q.NewQuery("myCollection"))

todo := &struct {
    Completed bool   `clover:"completed"`
    Title     string `clover:"title"`
    UserId    int    `clover:"userId"`
}{}

for _, doc := range docs {
    doc.Unmarshal(todo)
    log.Println(todo)
}
```

### Filter Documents with Criteria

In order to filter the documents returned by `FindAll()`, you have to specify a query Criteria using the `Where()` method. A Criteria object simply represents a predicate on a document, evaluating to **true** only if the document satisfies all the query conditions. 


The following example shows how to build a simple Criteria, matching all the documents having the **completed** field equal to true.

```go
db.FindAll(q.NewQuery("todos").Where(q.Field("completed").Eq(true)))

// or equivalently
db.FindAll(q.NewQuery("todos").Where(q.Field("completed").IsTrue()))
```

In order to build very complex queries, we chain multiple Criteria objects by using the `And()` and `Or()` methods, each returning a new Criteria obtained by applying the corresponding logical operator.

```go
// find all completed todos belonging to users with id 5 and 8
db.FindAll(q.NewQuery("todos").Where(c.Field("completed").Eq(true).And(q.Field("userId").In(5, 8))))
```

Naturally, you can also create Criteria involving multiple fields. CloverDB provides you with two equivalent ways to accomplish this:

```go
db.FindAll(q.NewQuery("myCollection").Where(q.Field("myField1").Gt(q.Field("myField2"))))
// or, if you prefer
db.FindAll(q.NewQuery("myCollection").Where(q.Field("myField1").Gt("$myField2")))
```

### Sorting Documents

To sort documents in CloverDB, you need to use `Sort()`. It is a variadic function which accepts a sequence of SortOption, each allowing to specify a field and a sorting direction.
A sorting direction can be one of 1 or -1, respectively corresponding to ascending and descending order. If no SortOption is provided, `Sort()` uses the **_id** field by default.

```go
// Find any todo belonging to the most recent inserted user
db.FindFirst(q.NewQuery("todos").Sort(c.SortOption{"userId", -1}))
```

### Skip/Limit Documents

Sometimes, it can be useful to discard some documents from the output, or simply set a limit on the maximum number of results returned by a query. For this purpose, CloverDB provides the `Skip()` and `Limit()` functions, both accepting an integer $n$ as parameter.

```go
// discard the first 10 documents from the output,
// also limiting the maximum number of query results to 100
db.FindAll(q.NewQuery("todos").Skip(10).Limit(100))
```

### Update/Delete Documents

The `Update()` method is used to modify specific fields of documents in a collection. The `Delete()` method is used to delete documents. Both methods belong to the Query object, so that it is easy to update and delete documents matching a particular query.

```go
// mark all todos belonging to user with id 1 as completed
updates := make(map[string]interface{})
updates["completed"] = true

db.Update(q.NewQuery("todos").Where(q.Field("userId").Eq(1)), updates)

// delete all todos belonging to users with id 5 and 8
db.Delete(q.NewQuery("todos").Where(q.Field("userId").In(5,8)))
```

To update or delete a single document using a specific document id, use `UpdateById()` or `DeleteById()`, respectively:

```go
docId := "1dbce353-d3c6-43b3-b5a8-80d8d876389b"
// update the document with the specified id
db.UpdateById("todos", docId, map[string]interface{}{"completed": true})
// or delete it
db.DeleteById("todos", docId)
```

## Indexes

In CloverDB, indexes support the efficient execution of queries. Without indexes, a collection must be fully scanned to select those documents matching a given query. An index is a special data structure storing the values of a specific document field (or set of fields), sorted by the value of the field itself. This means that they can be exploited to supports efficient equality matches and range-based queries. 
Moreover, when documents are iterated through an index, results can be returned in sorted order without performing any additional sorting step.
Note however that using indexes is not completely for free. Apart from increasing disk space, indexes require additional cpu-time during each insert and update/delete operation. Moreover, when accessing a document through an index, two disk reads must be performed, since indexes only store a reference (the document id) to the actual document. As a consequence, the speed-up is sensitive only when the specified criteria is used to access a restricted set of documents.

### Creating an index

Currently, CloverDB only support single-field indexes. An index can be created simply by calling the `CreateIndex()` method, which takes both the names of the collection and the field to be indexed.

```go
db.CreateIndex("myCollection", "myField")
```

Assume you have the following query:

```go
criteria := q.Field("myField").Gt(a).And(q.Field("myField").Lt(b))
db.FindAll(q.NewQuery("myCollection").Where(criteria).Sort(q.SortOption{"myField", -1}))
```

where **a** and **b** are values of your choice. CloverDB will use the created index both to perform the range query and to return results in sorted order.

## Data Types

Internally, CloverDB supports the following primitive data types: **int64**, **uint64**, **float64**, **string**, **bool** and **time.Time**. When possible, values having different types are silently converted to one of the internal types: signed integer values get converted to int64, while unsigned ones to uint64. Float32 values are extended to float64.

For example, consider the following snippet, which sets an uint8 value on a given document field:

```go
doc := d.NewDocument()
doc.Set("myField", uint8(10)) // "myField" is automatically promoted to uint64

fmt.Println(doc.Get("myField").(uint64))
```

Pointer values are dereferenced until either **nil** or a **non-pointer** value is found:

``` go
var x int = 10
var ptr *int = &x
var ptr1 **int = &ptr

doc.Set("ptr", ptr)
doc.Set("ptr1", ptr1)

fmt.Println(doc.Get("ptr").(int64) == 10)
fmt.Println(doc.Get("ptr1").(int64) == 10)

ptr = nil

doc.Set("ptr1", ptr1)
// ptr1 is not nil, but it points to the nil "ptr" pointer, so the field is set to nil
fmt.Println(doc.Get("ptr1") == nil)
```

Invalid types leaves the document untouched:

```go
doc := d.NewDocument()
doc.Set("myField", make(chan struct{}))

log.Println(doc.Has("myField")) // will output false
```

## Contributing

**CloverDB** is actively developed. Any contribution, in the form of a suggestion, bug report or pull request, is well accepted :blush:

Contributions and suggestions have been gratefully received from the following users:

<a href="https://github.com/ostafen/clover/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=ostafen/clover" />
</a>

Made with [contrib.rocks](https://contrib.rocks).

