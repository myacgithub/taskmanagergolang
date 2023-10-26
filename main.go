package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	taskMutex sync.Mutex
	client    *mongo.Client
	ctx       context.Context
)

func init() {
	// Initialize MongoDB client
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, _ = mongo.Connect(ctx, clientOptions)
}

type Task struct {
	ID          primitive.ObjectID `json:"id" bson:"_id"`
	Description string             `json:"description"`
	Completed   bool               `json:"completed"`
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/tasks", getTasksHandler).Methods("GET")
	r.HandleFunc("/tasks", addTaskHandler).Methods("POST")
	r.HandleFunc("/tasks/{id:[0-9a-f]+}", deleteTaskHandler).Methods("DELETE")
	r.HandleFunc("/tasks/{id:[0-9a-f]+}", completeTaskHandler).Methods("PUT")

	http.Handle("/", r)
	fmt.Println("Server is running on :8080")
	http.ListenAndServe(":8080", nil)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("index.html"))
	tmpl.Execute(w, nil)
}

func getTasksHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch tasks from MongoDB
	collection := client.Database("taskdb").Collection("tasks")
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error fetching tasks from MongoDB:", err)
		return
	}
	defer cursor.Close(ctx)

	var tasks []Task
	for cursor.Next(ctx) {
		var task Task
		if err := cursor.Decode(&task); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Println("Error decoding task:", err)
			return
		}
		tasks = append(tasks, task)
	}

	jsonTasks, err := json.Marshal(tasks)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error encoding tasks to JSON:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonTasks)
}

func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	description := r.FormValue("description")

	if description == "" {
		http.Error(w, "Task description is required", http.StatusBadRequest)
		return
	}

	// Create a new task with an ObjectID
	task := Task{
		ID:          primitive.NewObjectID(),
		Description: description,
		Completed:   false,
	}
	collection := client.Database("taskdb").Collection("tasks")
	_, err := collection.InsertOne(ctx, task)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error inserting task:", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func deleteTaskHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Delete a task from MongoDB using the ObjectID
	collection := client.Database("taskdb").Collection("tasks")
	result, err := collection.DeleteOne(ctx, bson.M{"_id": objectID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println("Error deleting task:", err)
		return
	}

	if result.DeletedCount == 0 {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}
func completeTaskHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update the task as completed in MongoDB
	collection := client.Database("taskdb").Collection("tasks")
	filter := bson.M{"_id": objectID}
	update := bson.M{"$set": bson.M{"completed": true}}

	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
