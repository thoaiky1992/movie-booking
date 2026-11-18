# Application Architecture

MACALLAN

TECHNICAL DOCUMENT

Current Version: 1.0

Application Architecture



## **A. Glossary**

List of terms and their definitions, often found at the end of the document.

| **Term** | **Meaning** | **Note** |
| --- | --- | --- |
|  |  |  |

## **B. Change log**

| **Version** | **Author** | **Changelog** | **Date** |
| --- | --- | --- | --- |
| 1.0 | Duke | Init |  |

## **C. Overview**

**Go microservice** built on:

| Concern | Technology |
| --- | --- |
| HTTP API | Gin |
| Internal RPC | gRPC / Protocol Buffers |
| Dependency Injection | Uber Fx |
| ORM | GORM + PostgreSQL |
| Cache | Valkey |
| Event Bus | Apache Kafka |
| Identity | Keycloak |
| CLI | Cobra |
| Observability | OpenTelemetry + Prometheus + Zap |

The architectural style is **Clean Architecture** *(Mixed Hexagonal / Ports & Adapters)*, combined with **CQRS** at the application layer and **Event-Driven** integration across services. 

## **C. Project Structure**

Every service follows the same top-level layout:

```none
<service-name>/
├── main.go                  # Entry point — delegates to cmd/
├── cmd/                     # Cobra CLI commands
│   ├── root.go
│   ├── server.go            # Starts HTTP + gRPC server
│   ├── worker.go            # Starts background consumer worker
│   ├── migration.go
│   └── seed.go
│
├── internal/
│   ├── server/              # Bootstrap: Fx app, router, lifecycle
│   └── worker/              # Bootstrap: Kafka consumer worker
│
├── modules/                 # Feature modules (one per bounded context)
│   ├── module.go            # Top-level Fx module registry
│   └── <feature>/           # e.g., user/, auth/, restriction/
│       ├── module.go
│       ├── domain/
│       ├── application/
│       ├── infrastructure/
│       └── presentation/
│
├── shared/                  # Configs, constants shared across modules
├── proto/                   # Protobuf definitions + generated code
└── database/                # GORM models, migrations, seeds
```

#### Feature Module Layout

Within each feature module the four Clean Architecture layers map directly to folders:

```
modules/user/
├── module.go                       # Fx wiring for this bounded context
├── event_di.go                     # Event consumer Fx registrations
│
├── domain/                         # Layer 1 — Domain
│   ├── user.go
│   ├── player_info.go
│   ├── email.go                    # Value objects
│   ├── password.go
│   └── error.go
│
├── application/                    # Layer 2 — Application (CQRS)
│   ├── command_create_player_by_email.go
│   ├── command_change_password_user.go
│   ├── query_get_player_profile.go
│   ├── repository_player.go        # Repository port (interface only)
│   └── repository_user.go
│
├── infrastructure/                 # Layer 3 — Infrastructure
│   ├── persistence/                # Repository implementations (GORM)
│   ├── cache/                      # Redis cache proxies
│   ├── producer/                   # Kafka producers
│   ├── grpc_client/                # gRPC client adapters
│   └── uow/                        # Unit of Work implementation
│
└── presentation/                   # Layer 4 — Presentation
    ├── http_api/                   # Gin handlers + DTOs
    ├── grpc_api/                   # gRPC service implementations
    └── event/                      # Kafka consumer handlers
```

## D. Clean Architecture Layers

Dependencies always point **inward**. Outer layers depend on inner layers; inner layers never import outer layers.

### Domain Layer

- Contains **entities** (aggregate roots) and **value objects**.
- Zero dependencies on frameworks, ORMs, or external services.
- Encapsulates invariants and domain errors.

```
// modules/user/domain/user.go
type User struct {
    common.DomainModel
    ShortID         string
    Email           *Email        // value object — enforces format
    Phone           *Phone
    Password        *Password     // value object — hashed, never plain-text
    Status          UserStatus    // enum: new | active | suspended | closed
    Type            UserType      // player | operator | bo
    TenantID        *uuid.UUID
    IsMfaEnabled    bool
    LastLoginAt     *time.Time
}
```

Value objects encapsulate validation so the domain never holds invalid state:

```
type Email struct{ value string }

func NewEmail(raw string) (*Email, error) {
    if !isValidEmail(raw) { return nil, domain.ErrInvalidEmail }
    return &Email{value: raw}, nil
}
```

### Application Layer

- One file per **use-case** (command or query).
- Defines **repository and event port interfaces** — the application layer owns the contract; infrastructure implements it.
- No direct imports infrastructure components like `gorm`, `redis`, or `kafka`.

### Infrastructure Layer

- Implements every interface the application layer defines.
- Three sub-layers:
    - **Persistence** — GORM repositories, SQL model mappers.
    - **Cache** — Redis read-through proxy wrappers around repository implementations.
    - **Messaging** — Kafka producers that publish domain events.

### Presentation Layer

- Thin adapters that translate wire formats into application calls.
-  Sub-layers:
    - **http\_api** — Gin route handlers and request/response DTOs.
    - **grpc\_api** — gRPC service implementations wrapping application commands/queries.
    - **event** — Kafka consumer handlers that deserialize messages and invoke commands

## E. CQRS Pattern

The application layer separates every operation into either a **Command** (write) or a **Query** (read).

### Commands

Commands change state. They return minimal data (IDs, tokens) and produce side effects (DB writes, events).

```
// Interface defined in application/
type ICreatePlayerByEmailCommand interface {
    Execute(ctx context.Context, dto *domain.CreatePlayerByEmailDto,
            meta PlayerSignUpPublishMeta) (uuid.UUID, *auth_domain.TokenResponse, error)
}

// Implementation (same package, no separate handler file needed)
type commandCreatePlayerByEmail struct {
    uow               IUowCreatePlayer      // transaction boundary
    playerRepo        IPlayerRepository
    sessionService    session_app.ISessionService
    playerSyncProducer IPlayerSyncProducer  // event publication
    consentCmd        consent_application.ICommandSubmitTnCAndPrivacyPolicy
}

func (c *commandCreatePlayerByEmail) Execute(ctx context.Context, ...) (uuid.UUID, ..., error) {
    // 1. Validate business rules (TnC accepted, email not taken)
    // 2. Build aggregate
    player := domain.NewPlayer(dto)
    // 3. Persist inside a Unit of Work transaction
    if err := c.uow.Execute(ctx, func(tx Tx) error {
        return tx.PlayerRepo.Create(ctx, player)
    }); err != nil { return uuid.Nil, nil, err }
    // 4. Publish integration event
    c.playerSyncProducer.PublishSignUp(ctx, player)
    return player.UserID, token, nil
}
```

**Naming convention:** `command_<verb>_<subject>.go`

### Queries

Queries read state. They have no side effects and may fan out across repositories, caches, and downstream gRPC services.

```
// Interface defined in application/
type IGetUserProfileQuery interface {
    Execute(ctx context.Context, tenantID uuid.UUID, userID string) (*domain.Player, error)
}

type queryGetUserProfile struct {
    repository  IPlayerRepository          // DB or cache-backed
    restriction IRestrictionQueryService   // remote gRPC
}

func (q *queryGetUserProfile) Execute(ctx context.Context, ...) (*domain.Player, error) {
    player, err := q.repository.GetByUserID(ctx, userID)
    if err != nil { return nil, err }

    restrictions, _ := q.restriction.GetUserRestrictions(ctx, tenantID, userID)
    player.Restrictions = restrictions   // enrich read model

    return player, nil
}
```

**Naming convention:** `query_<verb>_<subject>.go`

### Handler Wiring (no command bus)

Rather than a central command bus, handlers are wired via Fx DI and injected directly into the presentation layer. This keeps the call path explicit and type-safe:

```
// modules/user/presentation/http_api/http_api.go
type UserHTTPContainer struct {
    // Commands
    CreatePlayerByEmailCmd application.ICreatePlayerByEmailCommand
    ChangePasswordCmd      application.IChangePasswordPlayerCommand

    // Queries
    GetPlayerProfileQuery  application.IGetUserProfileQuery
    ListPlayersQuery       application.IListPlayersQuery
}

func (c *UserHTTPContainer) GetPlayerProfile() gin.HandlerFunc {
    return func(ctx *gin.Context) {
        userID := middleware.UserIDFromContext(ctx)
        player, err := c.GetPlayerProfileQuery.Execute(ctx, tenantID, userID)
        // map domain model → response DTO → JSON
    }
}
```

## F. Data Flow

### HTTP Write (Command)

```
Client
  │  POST /v1/account/register
  ▼
Gin Middleware (Auth · TenantID · TraceID)
  │
  ▼
Presentation: HTTP Handler
  │  bind & validate request DTO
  ▼
Application: Command.Execute()
  │  build domain aggregate
  │  call UoW → DB transaction
  │  call Kafka Producer
  ▼
Infrastructure: Persistence (GORM) ──► PostgreSQL
Infrastructure: Kafka Producer      ──► Kafka Topic
  │
  ▼ (return IDs / token)
Presentation: HTTP Handler
  │  map to response DTO
  ▼
Client  (201 Created + body)
```

### HTTP Read (Query)

```
Client
  │  GET /v1/account/profile
  ▼
Gin Middleware
  │
  ▼
Presentation: HTTP Handler
  │
  ▼
Application: Query.Execute()
  │
  ├──► Infrastructure: Cache Proxy (Redis)
  │       hit  ──► return cached model
  │       miss ──► Infrastructure: Persistence (GORM) ──► PostgreSQL
  │                  ──► write to cache ──► return model
  │
  ├──► Infrastructure: gRPC Client ──► Restriction Service
  │       enrich player model with restriction data
  │
  ▼
Presentation: map domain model → response DTO
  │
  ▼
Client  (200 OK + body)
```

### Event Consumer Flow

```
Kafka Topic (e.g., wallet.deposit.credit.processed.v1)
  │
  ▼
Worker: ConsumerRegistry (registered at startup)
  │
  ▼
Presentation: Event Handler (presentation/event/)
  │  deserialize Kafka message → domain DTO
  ▼
Application: Command.Execute()
  │  (same command objects used by HTTP handlers)
  ▼
Infrastructure: Persistence / Cache / Producers
```

## G. Event-Driven Architecture

### Integration Events (Kafka)

Services communicate asynchronously through Kafka topics. Events are named by domain action and versioned:

```
TopicUserLoginV1                       = "user.login.v1"
TopicPlayerAccountClosedV1             = "player.account.closed.v1"
TopicWalletDepositCreditProcessedV1    = "wallet.deposit.credit.processed.v1"
```

**Publishing** (infrastructure/producer):

```
// modules/user/infrastructure/producer/player_sync_producer.go
type playerSyncProducer struct {
    broker core_msg_broker.IProducer
}

func (p *playerSyncProducer) PublishSignUp(ctx context.Context, player *domain.Player) error {
    msg := mapToSignUpEvent(player)
    return p.broker.Publish(ctx, TopicPlayerSignedUpV1, msg)
}
```

**Consuming** (presentation/event):

```
// modules/user/presentation/event/register.go
func RegisterUserEvents(registry core_msg_broker.IConsumerRegistry, handler *UserEvent) {
    registry.RegisterNormal(TopicUserLoginV1,        handler.HandleLoginEvent)
    registry.RegisterNormal(TopicPlayerAccountClosedV1, handler.HandleAccountClosed)
}
```

Kafka consumers run in the **worker** process (separate from the HTTP server), started via `cmd/worker.go`.

### Local / In-Process Events

For side effects within the same process (e.g., triggering audit log after a write), local events are dispatched synchronously without Kafka:

`presentation/local_event/ — handlers for in-process domain events`

### Event Flow Across Services

```
User Service                     Wallet Service
    │                                │
    │  user.signed_up.v1 ──────────► │  (ignored or handled)
    │                                │
    │ ◄─── wallet.deposit.credit.processed.v1
    │      HandleDepositCreditProcessed()
    │      → incrementDepositCountCmd.Execute()
```

## H. Dependency Injection

All wiring is handled by **Uber Fx**. There are no manually constructed dependency graphs.

### Module Registration

```
// modules/user/module.go
var UserModule = fx.Module("User",
    // Infrastructure providers
    fx.Provide(persistence.NewPlayerRepository),
    fx.Provide(cache.NewPlayerRepositoryCacheProxy),   // wraps persistence repo
    fx.Provide(producer.NewPlayerSyncProducer),

    // Application providers
    fx.Provide(application.NewCreatePlayerByEmailCommand),
    fx.Provide(application.NewGetUserProfileQuery),

    // Presentation providers
    fx.Provide(http_api.NewUserHTTPContainer),
    fx.Provide(grpc_api.NewUserGRPCService),
)
```

### Cache Proxy Pattern

The cache proxy wraps the persistence repository and is registered under the **same interface**. The application layer sees one `IPlayerRepository` — it doesn't know whether it's talking to Postgres directly or to a Redis-backed proxy:

```
// infrastructure/cache/player_repository_cache.go
type playerRepositoryCacheProxy struct {
    cache    redis.Client
    fallback application.IPlayerRepository  // the real DB repository
}

func (p *playerRepositoryCacheProxy) GetByUserID(ctx context.Context, id domain.UserID) (*domain.Player, error) {
    if cached, ok := p.cache.Get(ctx, cacheKey(id)); ok {
        return cached, nil
    }
    player, err := p.fallback.GetByUserID(ctx, id)
    if err == nil { p.cache.Set(ctx, cacheKey(id), player, ttl) }
    return player, err
}
```

## I. Cross-Cutting Concerns

### Transaction Management (Unit of Work)

Multi-aggregate writes are wrapped in a **Unit of Work** to ensure atomicity:

```
// application/repository_uow.go  — port definition
type IUowCreatePlayer interface {
    Execute(ctx context.Context, fn func(tx UowTx) error) error
}

// infrastructure/uow/uow_create_player.go  — adapter implementation
type uowCreatePlayer struct{ db *gorm.DB }

func (u *uowCreatePlayer) Execute(ctx context.Context, fn func(UowTx) error) error {
    return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        return fn(newUowTx(tx))
    })
}
```

### Authentication Middleware

Every protected route passes through Gin middleware that:

1. Validates the JWT (Keycloak-issued token).
2. Extracts `UserID`, `TenantID`, `Roles` and injects them into the request context.
3. Handlers retrieve these via context helpers — they never parse tokens themselves.

### Multi-Tenancy

`TenantID` is present on every aggregate root. The middleware extracts it from the JWT; repositories always filter by it. This is enforced at the type level — queries without a `TenantID` do not compile.

### Observability

- **Tracing:** OpenTelemetry spans are attached to every incoming HTTP request and propagated through context into gRPC calls and Kafka messages.
- **Metrics:** Prometheus counters/histograms exposed at `/metrics`.
- **Logging:** Structured JSON logs via Zap; logger is retrieved from context so trace IDs are automatically included.

## K. Cross-Cutting Concerns

### Synchronous (gRPC)

Used when the caller needs a response before it can proceed (e.g., user profile query enriched with live restriction data):

```
User Service ──gRPC──► Restriction Service
             ◄──────── restriction list
```

gRPC clients live in `infrastructure/grpc_client/`. They implement an application-layer interface, keeping the application layer decoupled from the transport:

```
// application port
type IRestrictionQueryService interface {
    GetUserRestrictions(ctx context.Context, tenantID uuid.UUID, userID uuid.UUID) ([]*domain.Restriction, error)
}

// infrastructure adapter
type restrictionGRPCClient struct{ conn proto.RestrictionServiceClient }

func (c *restrictionGRPCClient) GetUserRestrictions(...) ([]*domain.Restriction, error) {
    resp, err := c.conn.GetRestrictions(ctx, &proto.GetRestrictionsRequest{...})
    return mapProtoToDomain(resp), err
}
```

### Asynchronous (Kafka)

Used for integration events where the publisher does not need the consumer's response:

| Pattern | Example |
| --- | --- |
| Fan-out | `user.signed_up.v1` consumed by CRM, Notification, and Report services |
| Saga step | `wallet.deposit.credit.processed.v1` triggers deposit count increment in User Service |
| Outbox (eventual) | Account closure triggers cascading cancellations in Restriction, Limit, Document services |

## L. Module Anatomy

Adding a new bounded context follows the same pattern every time:

```
Step 1 — Domain
  modules/<feature>/domain/
    entity.go           — aggregate root (no framework imports)
    value_object.go     — immutable, validated type
    error.go            — sentinel errors

Step 2 — Application
  modules/<feature>/application/
    command_<verb>_<subject>.go  — one use-case per file
    query_<verb>_<subject>.go
    repository_<entity>.go       — interface, NOT implementation
    event_<entity>.go            — event port interfaces

Step 3 — Infrastructure
  modules/<feature>/infrastructure/
    persistence/repository_<entity>.go   — GORM implementation
    cache/<entity>_cache_proxy.go        — Redis cache proxy
    producer/<entity>_producer.go        — Kafka publisher

Step 4 — Presentation
  modules/<feature>/presentation/
    http_api/http_api.go          — route setup + handler methods
    http_api/dto/                 — request/response structs
    grpc_api/<feature>_service.go — gRPC service impl
    event/register.go             — Kafka consumer registrations

Step 5 — Wire up
  modules/<feature>/module.go    — Fx providers for everything above
  modules/module.go              — add new module to FeaturesModules
```

### Dependency Rules Checklist

| Layer | May import | Must NOT import |
| --- | --- | --- |
| `domain/` | standard library, `common.DomainModel` | `gorm`, `gin`, `kafka`, `redis`, other modules |
| `application/` | `domain/`, standard library | `gorm`, `gin`, `kafka`, infrastructure packages |
| `infrastructure/` | `application/`, `domain/`, external libs | `presentation/` |
| `presentation/` | `application/`, `domain/`, `gin`/`grpc` | `infrastructure/` (directly) |
