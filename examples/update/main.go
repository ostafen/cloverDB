package main

import (
	"fmt"
	"log"

	c "github.com/ostafen/clover/v2"
	d "github.com/ostafen/clover/v2/document"
	"github.com/ostafen/clover/v2/query"
)

func main() {
	db, _ := c.Open("clover-db")
	defer db.Close()

	db.CreateCollection("todos")

	// Create documents
	todo1 := d.NewDocument()
	todo2 := d.NewDocument()

	todo1.Set("title", "delectus aut autem")
	todo1.Set("completed", false)
	todo1.Set("userId", 1)

	todo2.Set("title", "quis ut nam facilis et officia qui")
	todo2.Set("completed", false)
	todo2.Set("userId", 2)

	// Insert documents to collection "todos"
	db.Insert("todos", todo1, todo2)

	updates := make(map[string]interface{})
	updates["completed"] = true

	// mark all incomplete todos as completed
	q := query.NewQuery("todos").Where(query.Field("completed").Eq(false))
	db.Update(q, updates)

	// Query all todos
	todos, _ := db.FindAll(query.NewQuery("todos"))
	for _, todo := range todos {
		fmt.Printf("title: %v, completed: %v\n", todo.Get("title"), todo.Get("completed"))
	}

	// Delete todos with userId of 2
	db.Delete(query.NewQuery("todos").Where(query.Field("userId").Eq(2)))

	todos, _ = db.FindAll(query.NewQuery("todos"))
	for _, todo := range todos {
		fmt.Printf("title: %v, userId: %v\n", todo.Get("title"), todo.Get("userId"))
	}

	exists, err := db.HasCollection("dualCollection")
	if err != nil {
		log.Fatal(err)
	}
	if exists {
		err = db.DropCollection("dualCollection")
		if err != nil {
			log.Fatal(err)
		}
	}

	err = db.CreateCollectionByQuery(
		"dualCollection",
		query.NewQuery("todos").Where(query.Field("completed").Eq(true)).Limit(1),
	)
	if err != nil {
		log.Fatal(err)
	} else {
		collections, err := db.ListCollections()
		if err != nil {
			log.Fatal(err)
		}
		for k, v := range collections {
			fmt.Printf("collection_%d: %s\n", k, v)
		}
	}

	dualTodos, _ := db.FindAll(query.NewQuery("dualCollection"))
	for _, v := range dualTodos {
		fmt.Printf("title: %v, completed: %v\n", v.Get("title"), v.Get("completed"))
	}
}
