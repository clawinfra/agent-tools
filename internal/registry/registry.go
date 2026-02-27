package registry

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/clawinfra/agent-tools/internal/store"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ErrNotFound is returned when a resource is not found.
var ErrNotFound = errors.New("not found")

// ErrDuplicate is returned when a tool with the same name+version already exists.
var ErrDuplicate = errors.New("duplicate tool")

// Registry manages tool registration and discovery.
type Registry struct {
	db  *store.DB
	log *zap.Logger
}

// New creates a new Registry.
func New(db *store.DB, log *zap.Logger) *Registry {
	return &Registry{db: db, log: log}
}

// RegisterTool registers a new tool and returns it.
func (r *Registry) RegisterTool(ctx context.Context, req *RegisterToolRequest) (*Tool, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	schemaJSON, err := json.Marshal(req.Schema)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}
	pricingJSON, err := json.Marshal(req.Pricing)
	if err != nil {
		return nil, fmt.Errorf("marshal pricing: %w", err)
	}

	id := makeToolDID(req.Name, req.Version, req.ProviderID)
	now := time.Now().Unix()
	tags := strings.Join(req.Tags, ",")

	// Auto-upsert the provider if not already registered (v0.1: no strict auth yet).
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO providers (id, name, endpoint, pubkey, stake_claw, reputation, created_at, last_seen)
		VALUES (?, '', '', '', '0', 0, ?, ?)
		ON CONFLICT(id) DO UPDATE SET last_seen=excluded.last_seen
	`, req.ProviderID, now, now)
	if err != nil {
		return nil, fmt.Errorf("upsert provider: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO tools (id, name, version, description, schema_json, pricing, provider_id, endpoint, timeout_ms, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, req.Name, req.Version, req.Description, string(schemaJSON), string(pricingJSON),
		req.ProviderID, req.Endpoint, req.TimeoutMS, tags, now, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, fmt.Errorf("%w: %s@%s", ErrDuplicate, req.Name, req.Version)
		}
		return nil, fmt.Errorf("insert tool: %w", err)
	}

	r.log.Info("tool registered",
		zap.String("id", id),
		zap.String("name", req.Name),
		zap.String("version", req.Version),
		zap.String("provider", req.ProviderID),
	)

	return r.GetTool(ctx, id)
}

// GetTool returns a tool by ID.
func (r *Registry) GetTool(ctx context.Context, id string) (*Tool, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, version, description, schema_json, pricing, provider_id, endpoint, timeout_ms, tags, created_at, updated_at, is_active
		FROM tools WHERE id = ?
	`, id)
	return scanTool(row)
}

// ListTools returns paginated tools.
func (r *Registry) ListTools(ctx context.Context, page, limit int) (*SearchResult, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, version, description, schema_json, pricing, provider_id, endpoint, timeout_ms, tags, created_at, updated_at, is_active
		FROM tools WHERE is_active = 1
		ORDER BY created_at DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tools, err := scanTools(rows)
	if err != nil {
		return nil, err
	}

	var total int
	err = r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM tools WHERE is_active = 1").Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("count tools: %w", err)
	}

	return &SearchResult{
		Tools: tools,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}

// SearchTools performs full-text search on the tool registry.
func (r *Registry) SearchTools(ctx context.Context, q *SearchQuery) (*SearchResult, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}
	offset := (q.Page - 1) * q.Limit

	var (
		rows *sql.Rows
		err  error
	)

	if q.Query != "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT t.id, t.name, t.version, t.description, t.schema_json, t.pricing, 
			       t.provider_id, t.endpoint, t.timeout_ms, t.tags, t.created_at, t.updated_at, t.is_active
			FROM tools t
			WHERE t.is_active = 1
			  AND t.rowid IN (SELECT rowid FROM tools_fts WHERE tools_fts MATCH ?)
			ORDER BY t.created_at DESC LIMIT ? OFFSET ?
		`, q.Query+"*", q.Limit, offset)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, name, version, description, schema_json, pricing, provider_id, endpoint, timeout_ms, tags, created_at, updated_at, is_active
			FROM tools WHERE is_active = 1
			ORDER BY created_at DESC LIMIT ? OFFSET ?
		`, q.Limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("search tools: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tools, err := scanTools(rows)
	if err != nil {
		return nil, err
	}

	return &SearchResult{
		Tools: tools,
		Total: len(tools), // simplified; full count query would be separate
		Page:  q.Page,
		Limit: q.Limit,
		Query: q.Query,
	}, nil
}

// DeactivateTool soft-deletes a tool.
func (r *Registry) DeactivateTool(ctx context.Context, id, providerID string) error {
	res, err := r.db.ExecContext(ctx,
		"UPDATE tools SET is_active = 0, updated_at = ? WHERE id = ? AND provider_id = ?",
		time.Now().Unix(), id, providerID)
	if err != nil {
		return fmt.Errorf("deactivate: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%w or not authorized", ErrNotFound)
	}
	return nil
}

// RegisterProvider registers or upserts a provider.
func (r *Registry) RegisterProvider(ctx context.Context, p *Provider) (*Provider, error) {
	if p.ID == "" {
		return nil, fmt.Errorf("provider id is required")
	}
	if p.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if p.PubKey == "" {
		return nil, fmt.Errorf("pubkey is required")
	}
	now := time.Now().Unix()
	if p.StakeCLAW == "" {
		p.StakeCLAW = "0"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO providers (id, name, endpoint, pubkey, stake_claw, reputation, created_at, last_seen)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			endpoint=excluded.endpoint,
			pubkey=excluded.pubkey,
			stake_claw=excluded.stake_claw,
			last_seen=excluded.last_seen
	`, p.ID, p.Name, p.Endpoint, p.PubKey, p.StakeCLAW, now, now)
	if err != nil {
		return nil, fmt.Errorf("upsert provider: %w", err)
	}
	r.log.Info("provider registered", zap.String("id", p.ID))
	return r.GetProvider(ctx, p.ID)
}

// GetProvider returns a provider by ID.
func (r *Registry) GetProvider(ctx context.Context, id string) (*Provider, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, name, endpoint, pubkey, stake_claw, reputation, created_at, last_seen
		FROM providers WHERE id = ?
	`, id)
	return scanProvider(row)
}

// ListProviders returns all providers.
func (r *Registry) ListProviders(ctx context.Context) ([]*Provider, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, endpoint, pubkey, stake_claw, reputation, created_at, last_seen
		FROM providers ORDER BY reputation DESC, created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var providers []*Provider
	for rows.Next() {
		p, err := scanProviderRow(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

func scanProvider(row *sql.Row) (*Provider, error) {
	var (
		p         Provider
		createdAt int64
		lastSeen  int64
	)
	err := row.Scan(&p.ID, &p.Name, &p.Endpoint, &p.PubKey, &p.StakeCLAW, &p.Reputation, &createdAt, &lastSeen)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.CreatedAt = time.Unix(createdAt, 0)
	p.LastSeen = time.Unix(lastSeen, 0)
	return &p, nil
}

func scanProviderRow(rows *sql.Rows) (*Provider, error) {
	var (
		p         Provider
		createdAt int64
		lastSeen  int64
	)
	if err := rows.Scan(&p.ID, &p.Name, &p.Endpoint, &p.PubKey, &p.StakeCLAW, &p.Reputation, &createdAt, &lastSeen); err != nil {
		return nil, err
	}
	p.CreatedAt = time.Unix(createdAt, 0)
	p.LastSeen = time.Unix(lastSeen, 0)
	return &p, nil
}

// RecordInvocation creates a new invocation record.
// input is the raw input map; the hash is computed automatically.
func (r *Registry) RecordInvocation(ctx context.Context, toolID, consumerID string, input map[string]any) (string, error) {
	h, err := hashInput(input)
	if err != nil {
		return "", fmt.Errorf("hash input: %w", err)
	}
	id := "inv_" + uuid.NewString()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO invocations (id, tool_id, consumer_id, input_hash, started_at, status)
		VALUES (?, ?, ?, ?, ?, 'pending')
	`, id, toolID, consumerID, h, time.Now().Unix())
	if err != nil {
		return "", fmt.Errorf("record invocation: %w", err)
	}
	return id, nil
}

// CompleteInvocation updates an invocation with its result.
func (r *Registry) CompleteInvocation(ctx context.Context, id, outputHash, receiptSig, costCLAW string) error {
	now := time.Now().Unix()
	_, err := r.db.ExecContext(ctx, `
		UPDATE invocations SET
			status = 'completed', output_hash = ?, receipt_sig = ?, cost_claw = ?, completed_at = ?
		WHERE id = ?
	`, outputHash, receiptSig, costCLAW, now, id)
	return err
}

// FailInvocation marks an invocation as failed.
func (r *Registry) FailInvocation(ctx context.Context, id, reason string) error {
	now := time.Now().Unix()
	_, err := r.db.ExecContext(ctx, `
		UPDATE invocations SET status = 'failed', error = ?, completed_at = ? WHERE id = ?
	`, reason, now, id)
	return err
}

// hashInput computes the SHA-256 of a JSON-serialized input map.
func hashInput(input map[string]any) (string, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}

// makeToolDID generates a deterministic DID for a tool.
func makeToolDID(name, version, providerID string) string {
	h := sha256.Sum256([]byte(name + "@" + version + "#" + providerID))
	return "did:claw:tool:" + hex.EncodeToString(h[:16])
}

func scanTool(row *sql.Row) (*Tool, error) {
	var (
		t           Tool
		schemaJSON  string
		pricingJSON string
		tags        string
		createdAt   int64
		updatedAt   int64
		isActive    int
	)
	err := row.Scan(
		&t.ID, &t.Name, &t.Version, &t.Description,
		&schemaJSON, &pricingJSON, &t.ProviderID, &t.Endpoint,
		&t.TimeoutMS, &tags, &createdAt, &updatedAt, &isActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return assembleTool(&t, schemaJSON, pricingJSON, tags, createdAt, updatedAt, isActive)
}

func scanTools(rows *sql.Rows) ([]*Tool, error) {
	var tools []*Tool
	for rows.Next() {
		var (
			t           Tool
			schemaJSON  string
			pricingJSON string
			tags        string
			createdAt   int64
			updatedAt   int64
			isActive    int
		)
		if err := rows.Scan(
			&t.ID, &t.Name, &t.Version, &t.Description,
			&schemaJSON, &pricingJSON, &t.ProviderID, &t.Endpoint,
			&t.TimeoutMS, &tags, &createdAt, &updatedAt, &isActive,
		); err != nil {
			return nil, err
		}
		tool, err := assembleTool(&t, schemaJSON, pricingJSON, tags, createdAt, updatedAt, isActive)
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, rows.Err()
}

func assembleTool(t *Tool, schemaJSON, pricingJSON, tags string, createdAt, updatedAt int64, isActive int) (*Tool, error) {
	if err := json.Unmarshal([]byte(schemaJSON), &t.Schema); err != nil {
		return nil, fmt.Errorf("unmarshal schema: %w", err)
	}
	t.Pricing = &Pricing{}
	if err := json.Unmarshal([]byte(pricingJSON), t.Pricing); err != nil {
		return nil, fmt.Errorf("unmarshal pricing: %w", err)
	}
	if tags != "" {
		t.Tags = strings.Split(tags, ",")
	}
	t.CreatedAt = time.Unix(createdAt, 0)
	t.UpdatedAt = time.Unix(updatedAt, 0)
	t.IsActive = isActive == 1
	return t, nil
}
