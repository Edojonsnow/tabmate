package main

import (
	"context"
	"log"
	"os"
	tablecontrollers "tabmate/internals/controllers/table"
	tabmate "tabmate/internals/store/postgres"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	connectionString := os.Getenv("DB_SOURCE")
	if connectionString == "" {
		log.Fatal("DB_SOURCE environment variable is not set")
	}

	conn, err := pgx.Connect(context.Background(), connectionString)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close(context.Background())

	if err = conn.Ping(context.Background()); err != nil {
		log.Fatal(err)
	}
	log.Println("Successfully connected to the database!")

	queries := tabmate.New(conn)
	router := setupRouter(queries)

	err = tablecontrollers.InitializeActiveTables(context.Background(), queries)
	if err != nil {
		log.Fatalf("Failed to initialize active tables: %v", err)
	}
		
	log.Println("Server starting on http://localhost:8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}