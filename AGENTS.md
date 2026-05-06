# TabMate Backend

Go REST API for splitting bills and managing group dining. Users create or join virtual "tables" using 8-character codes, sync items in real-time, and split costs.

## Related Repos

- **Mobile frontend**: `../tabmate-mobile` — React Native + Expo + NativeWind
- Default dev port: `:8080`
- Mobile connects via `EXPO_PUBLIC_API_BASE` and `EXPO_PUBLIC_WS_BASE`

## Tech Stack

- **Language**: Go 1.24
- **Framework**: Gin (HTTP routing)
- **Database**: PostgreSQL via `pgx/v5`
- **Migrations**: Goose
- **Codegen**: sqlc (type-safe SQL → Go)
- **Auth**: AWS Cognito + JWT (`golang-jwt/jwt/v4`) + OIDC (`coreos/go-oidc`)
- **Real-time**: Gorilla WebSocket
- **Cloud**: AWS (Cognito, S3, Textract, RDS, Lambda, ECS/Fargate)

## Project Structure

```
cmd/api/
  main.go               # Server entry point
  routes.go             # All route definitions
internals/
  auth/                 # Cognito + OIDC token verification
  middleware/           # Auth middleware for protected routes
  controllers/
    auth/               # Login, signup, confirm, forgot password, callback
    table/              # Table CRUD, item management, WebSocket, sync
    user/               # User creation and retrieval
    fixedbills/         # Fixed bill CRUD, split calculation, settlement
  config/               # DB connection config
  store/postgres/       # sqlc-generated queries and model types
migrations/             # Goose SQL migration files (8 total)
templates/              # HTML templates for web views
db/                     # DB initialisation helpers
```

## Database Schema

### `users`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| name | text | |
| email | text | unique |
| cognito_sub | text | unique |
| profile_picture_url | text | nullable |
| created_at / updated_at | timestamptz | |

### `tables`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| created_by | UUID | FK → users |
| table_code | text | unique, 8 chars |
| name | text | |
| restaurant_name | text | |
| status | text | open / closed |
| menu_url | text | nullable |
| vat | numeric | default 0 |
| created_at / updated_at / closed_at | timestamptz | |

### `table_members`
| Column | Type | Notes |
|--------|------|-------|
| table_id | UUID | FK → tables |
| user_id | UUID | FK → users |
| joined_at | timestamptz | |
| role | text | host / guest |
| is_settled | bool | |

### `items`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| table_code | text | FK → tables.table_code |
| added_by_user_id | UUID | FK → users |
| name | text | |
| price | numeric | |
| quantity | int | |
| description | text | nullable |
| source | text | nullable |
| original_parsed_text | text | nullable |
| created_at / updated_at | timestamptz | |

### `fixedbills`
| Column | Type | Notes |
|--------|------|-------|
| id | UUID | PK |
| created_by | UUID | FK → users |
| bill_code | text | unique, 8 chars |
| name | text | |
| description | text | nullable |
| total_amount | numeric | |
| status | text | open / locked / settled |
| created_at / updated_at / settled_at | timestamptz | |

### `bill_members`
| Column | Type | Notes |
|--------|------|-------|
| bill_id | UUID | FK → fixedbills |
| user_id | UUID | FK → users |
| amount_owed | numeric | auto-calculated equal split |
| is_settled | bool | |
| settled_at | timestamptz | nullable |
| role | text | host / guest |
| joined_at | timestamptz | |

## API Routes

Defined in `cmd/api/routes.go`. Protected routes require `Authorization: Bearer <JWT>`.

### Public
| Method | Path | Controller |
|--------|------|------------|
| POST | `/login` | auth |
| POST | `/signup` | auth |
| GET | `/confirm-signup` | auth |
| POST | `/forgot-password` | auth |
| GET | `/logout` | auth |
| GET | `/callback` | auth |

### Protected (`/api/...`)

**Users**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/create-user` | Create user record in DB after Cognito signup |
| GET | `/api/get-user` | Get current user info |

**Tables**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/create-table` | Create table → `{ code, id, name, restaurant, created_by }` |
| POST | `/api/join-table/:code` | Join table by 8-char code |
| GET | `/api/tables/:code` | Get table details |
| GET | `/api/tables/:code/members` | List members with roles |
| GET | `/api/tables/:code/table-items` | All items with user details |
| GET | `/api/get-user-tables` | All tables for current user |
| POST | `/api/tables/:code/sync` | Delta sync pending changes (see below) |
| PATCH | `/api/tables/:code` | Update table VAT |

**Items**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/items` | Add item to table |
| PATCH | `/api/items/:id` | Update item quantity |
| DELETE | `/api/items/:id` | Delete item |

**Fixed Bills**
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/create-bill` | Create bill → `{ code, id, name, totalAmount }` |
| GET | `/api/bills/:code` | Get bill details |
| POST | `/api/join-bill/:code` | Join bill (recalculates equal split for all members) |
| GET | `/api/bills/:code/members` | List bill members with amounts owed |
| DELETE | `/api/bills/:code/leave` | Leave bill (recalculates split) |
| GET | `/api/bills/:code/split` | Per-member split breakdown |
| POST | `/api/bills/:code/settle` | Mark bill as settled |
| GET | `/api/get-user-bills` | All bills for current user |

**WebSocket**
| Method | Path | Description |
|--------|------|-------------|
| GET | `/ws/table/:code` | Real-time table updates (auth via `?token=<JWT>`) |

## Key Request/Response Shapes

### Create Table
```json
// POST /api/create-table
{ "tablename": "Dinner", "restaurant": "Fiora" }

// Response
{ "code": "a1b2c3d4", "id": "<uuid>", "name": "Dinner", "restaurant": "Fiora", "created_by": "<uuid>" }
```

### Create Fixed Bill
```json
// POST /api/create-bill
{ "billname": "Restaurant Bill", "description": "Dinner", "totalAmount": "150.50" }

// Response
{ "code": "x1y2z3w4", "id": "<uuid>", "name": "Restaurant Bill", "totalAmount": "150.50" }
```

### Sync Items (delta-based)
```json
// POST /api/tables/:code/sync
{
  "updates": [
    {
      "itemName": "Pizza",
      "price": 15.99,
      "quantityDelta": 1,
      "username": "john",
      "addedByUserId": "<uuid>"
    }
  ]
}

// Response
{ "status": "ok" }
```

## Authentication Flow

1. Mobile POSTs credentials to `/login` → Cognito verifies → returns JWT ID token + access token
2. Mobile sends `Authorization: Bearer <id-token>` on all protected requests
3. Auth middleware extracts and verifies JWT via OIDC / Cognito public keys
4. User identity pulled from token claims and stored in Gin context
5. WebSocket auth uses query param: `?token=<id-token>`

## Code Generation

sqlc generates type-safe Go from SQL queries. After changing SQL in `store/postgres/`:

```bash
sqlc generate
```

After changing schema, create a new migration:

```bash
goose -dir migrations create <name> sql
goose -dir migrations up
```

## Running Locally

```bash
go run cmd/api/main.go
```

Ensure PostgreSQL is running and environment variables are set (DB connection string, AWS Cognito pool/client IDs, AWS region/credentials).

## Current Feature State

- Table creation, joining, item management — complete
- WebSocket real-time broadcast — complete
- Delta-based item sync — complete
- Fixed bill (equal split) — actively being built
  - DB tables (`fixedbills`, `bill_members`) — complete
  - All CRUD endpoints — complete
  - Mobile integration — in progress
