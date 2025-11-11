# TabMate: Group Payment & Bill Splitting App

TabMate is a mobile application designed to simplify the process of splitting bills and managing payments when dining out with friends. It leverages real-time synchronization and smart menu parsing to create a seamless and transparent payment experience for everyone at the table.

This repository contains the Go (Golang) backend for the Pay-Up application.

## Table of Contents

- [Core Features](#core-features)
- [Architecture & Technology Stack](#architecture--technology-stack)
- [Project Status](#project-status)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation & Setup](#installation--setup)
- [Usage](#usage)
  - [Running the Server](#running-the-server)
  - [API Endpoints](#api-endpoints)
- [Database Migrations](#database-migrations)

## Core Features

- **Menu Scanning:** Scan a restaurant's menu QR code. The app uses AWS Textract (OCR) to parse the menu and extract dishes and prices.
- **Virtual Table:** Users at the same dining table can join a "virtual table" in the app using a unique, shareable code.
- **Real-time Order Sync:** When one person adds an item to their order, it appears instantly for everyone in the virtual table, and personal/group totals are updated in real-time via WebSockets.
- **Individual Cost Tracking:** Each user can see exactly what they've ordered and what their personal subtotal is at any given moment.
- **Simplified Bill Splitting:** At the end of the meal, the app provides a clear breakdown of who owes what, including tax and tip calculations.
- **P2P Payment Facilitation:** [Future Feature] Streamline peer-to-peer payments between users to settle the bill.

## Architecture & Technology Stack

The backend is built with a focus on scalability, performance, and leveraging modern cloud services.

### Backend

- **Language:** Go (Golang)
- **Web Framework:** Standard Library `net/http` with `gin` router for routing.
- **Real-time Communication:** WebSockets (using `gorilla/websocket`).
- **Database Interaction:** `sqlc` for generating type-safe Go code from raw SQL.
- **Database Driver:** `pgx/v5` for PostgreSQL.

### Database

- **Type:** PostgreSQL
- **Migrations:** `pressly/goose` for managing database schema evolution.

### Cloud & DevOps (AWS)

- **Authentication:** AWS Cognito
- **Compute:** AWS Fargate on ECS for running the Go application container.
- **Serverless:** AWS Lambda for asynchronous tasks (e.g., menu processing).
- **Storage:** AWS S3 for storing scanned menu images.
- **Database Hosting:** AWS RDS (PostgreSQL).
- **AI/ML:** AWS Textract for Optical Character Recognition (OCR).
- **API Management:** AWS API Gateway (handling both HTTP and WebSocket traffic).
- **Containerization:** Docker
- **Container Registry:** AWS ECR

## Project Status

**Current Phase:** [e.g., Initial Development, Alpha, Beta]

This project is currently under active development. The foundational backend services for user authentication, table creation, and real-time synchronization are being built.

## Getting Started

Follow these instructions to get the backend running on your local machine for development and testing purposes.

### Prerequisites

- Go (version 1.21 or newer)
- PostgreSQL (version 12 or newer)
- Docker & Docker Compose (for local database setup)
- `pressly/goose` migration tool (`go install github.com/pressly/goose/v3/cmd/goose@latest`)
- `sqlc` CLI tool (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)
- `air` for live-reloading (optional, recommended for development) (`go install github.com/cosmtrek/air@latest`)
- An AWS account with credentials configured for development.

### Installation & Setup

1.  **Clone the repository:**

    ```bash
    git clone https://github.com/Edojonsnow/tabmate.git
    ```

2.  **Set up environment variables:**

    - Copy the example environment file:
      ```bash
      cp .env.example .env
      ```
    - Edit the `.env` file and provide your local PostgreSQL connection string, AWS Cognito details, and any other required secrets.
      ```env
      # .env
      DB_SOURCE="postgresql://YOUR_USER:YOUR_PASSWORD@localhost:5432/tabmates_dev?sslmode=disable"
      # ... other variables
      ```

3.  **Start the local database (if using Docker):**

    ```bash
    docker-compose up -d
    ```

    _This will start a PostgreSQL container. You may need to create the initial database (`tabmate_dev`) manually._

4.  **Run database migrations:**

    - First, ensure your environment variables are loaded. If you are not using a tool like `direnv`, you can source them:
      ```bash
      source .env # (Ensure your .env file uses 'export' for this to work)
      ```
    - Apply all migrations:
      ```bash
      goose up postgres "$DB_SOURCE" -dir ./migrations
      ```

5.  **Generate Go code from SQL:**

    ```bash
    sqlc generate
    ```

6.  **Install Go dependencies:**
    ```bash
    go mod tidy
    ```

## Usage

### Running the Server

- **For Development (with live-reloading using `air`):**

  ```bash
  # Ensure .env variables are loaded (direnv is great for this)
  air
  ```

- **Standard Run (without live-reloading):**
  ```bash
  # Ensure .env variables are loaded
  go run ./cmd/api/main.go
  ```

The server will start, typically on port `8080` (or as configured in your `.env` file).

### API Endpoints

- **Authentication:**
  - `POST /api/create-user` - Create a new user in db.
- **Tables:**
  - `POST /api/create-table` - Create a new dining table.
  - `POST /api/tables/:code` - Join an existing table.
- **WebSockets:**
  - `GET /ws/table/:code` - Establish a WebSocket connection to a table.

## Database Migrations

This project uses `pressly/goose` for database schema migrations. Migration files are located in the `/migrations` directory.

- **Check Status:**
  ```bash
  goose -dir "./migrations" postgres "$DB_SOURCE" status
  ```
- **Apply Migrations:**
  ```bash
  goose -dir "./migrations" postgres "$DB_SOURCE" up
  ```
- **Roll Back Last Migration:**
  ```bash
  goose -dir "./migrations" postgres "$DB_SOURCE" down
  ```
- **Create a New Migration:**
  ```bash
  goose -dir "./migrations" create a_descriptive_name sql
  ```
