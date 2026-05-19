package postgres

import (
	"context"
	"encoding/json"
	stdlib_errors "errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

type IdentityProviderRepository struct {
	db DB
}

func NewIdentityProviderRepository(db DB) *IdentityProviderRepository {
	return &IdentityProviderRepository{db: db}
}

func (r *IdentityProviderRepository) Create(ctx context.Context, idp *domain.IdentityProvider) error {
	groupMappingsJSON, err := json.Marshal(idp.GroupMapping)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to marshal group mappings", err)
	}

	query := `
		INSERT INTO identity_providers (id, name, type, scope, tenant_id, client_id, client_secret,
			issuer_url, discovery_url, entity_id, sso_url, metadata_url, certificate,
			scopes, redirect_uris, enabled, default_role, group_mappings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW(), NOW())
	`
	_, err = r.db.Exec(ctx, query,
		idp.ID, idp.Name, idp.Type, idp.Scope, idp.TenantID,
		idp.ClientID, idp.ClientSecret, idp.IssuerURL, idp.DiscoveryURL,
		idp.EntityID, idp.SSOURL, idp.MetadataURL, idp.Certificate,
		idp.Scopes, idp.RedirectURIs, idp.Enabled, idp.DefaultRole, groupMappingsJSON,
	)
	return err
}

func (r *IdentityProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.IdentityProvider, error) {
	query := `
		SELECT id, name, type, scope, tenant_id, client_id, client_secret,
			issuer_url, discovery_url, entity_id, sso_url, metadata_url, certificate,
			scopes, redirect_uris, enabled, default_role, group_mappings, created_at, updated_at
		FROM identity_providers
		WHERE id = $1
	`
	var idp domain.IdentityProvider
	var groupMappingsJSON []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&idp.ID, &idp.Name, &idp.Type, &idp.Scope, &idp.TenantID,
		&idp.ClientID, &idp.ClientSecret, &idp.IssuerURL, &idp.DiscoveryURL,
		&idp.EntityID, &idp.SSOURL, &idp.MetadataURL, &idp.Certificate,
		&idp.Scopes, &idp.RedirectURIs, &idp.Enabled, &idp.DefaultRole, &groupMappingsJSON,
		&idp.CreatedAt, &idp.UpdatedAt,
	)
	if err != nil {
		if stdlib_errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New(errors.NotFound, "identity provider not found")
		}
		return nil, err
	}

	if err := json.Unmarshal(groupMappingsJSON, &idp.GroupMapping); err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to unmarshal group mappings", err)
	}

	return &idp, nil
}

func (r *IdentityProviderRepository) List(ctx context.Context, scope domain.IdentityProviderScope, tenantID *uuid.UUID) ([]*domain.IdentityProvider, error) {
	var query string
	var args []interface{}

	if scope != "" && tenantID != nil {
		query = `
			SELECT id, name, type, scope, tenant_id, client_id, client_secret,
				issuer_url, discovery_url, entity_id, sso_url, metadata_url, certificate,
				scopes, redirect_uris, enabled, default_role, group_mappings, created_at, updated_at
			FROM identity_providers
			WHERE scope = $1 AND tenant_id = $2
			ORDER BY name
		`
		args = []interface{}{scope, *tenantID}
	} else if scope != "" {
		query = `
			SELECT id, name, type, scope, tenant_id, client_id, client_secret,
				issuer_url, discovery_url, entity_id, sso_url, metadata_url, certificate,
				scopes, redirect_uris, enabled, default_role, group_mappings, created_at, updated_at
			FROM identity_providers
			WHERE scope = $1
			ORDER BY name
		`
		args = []interface{}{scope}
	} else if tenantID != nil {
		query = `
			SELECT id, name, type, scope, tenant_id, client_id, client_secret,
				issuer_url, discovery_url, entity_id, sso_url, metadata_url, certificate,
				scopes, redirect_uris, enabled, default_role, group_mappings, created_at, updated_at
			FROM identity_providers
			WHERE tenant_id = $1 OR tenant_id IS NULL
			ORDER BY name
		`
		args = []interface{}{*tenantID}
	} else {
		query = `
			SELECT id, name, type, scope, tenant_id, client_id, client_secret,
				issuer_url, discovery_url, entity_id, sso_url, metadata_url, certificate,
				scopes, redirect_uris, enabled, default_role, group_mappings, created_at, updated_at
			FROM identity_providers
			ORDER BY name
		`
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanIdentityProviders(rows)
}

func (r *IdentityProviderRepository) Update(ctx context.Context, idp *domain.IdentityProvider) error {
	groupMappingsJSON, err := json.Marshal(idp.GroupMapping)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to marshal group mappings", err)
	}

	query := `
		UPDATE identity_providers
		SET name = $1, type = $2, scope = $3, tenant_id = $4,
			client_id = $5, client_secret = $6, issuer_url = $7, discovery_url = $8,
			entity_id = $9, sso_url = $10, metadata_url = $11, certificate = $12,
			scopes = $13, redirect_uris = $14, enabled = $15, default_role = $16,
			group_mappings = $17, updated_at = NOW()
		WHERE id = $18
	`
	result, err := r.db.Exec(ctx, query,
		idp.Name, idp.Type, idp.Scope, idp.TenantID,
		idp.ClientID, idp.ClientSecret, idp.IssuerURL, idp.DiscoveryURL,
		idp.EntityID, idp.SSOURL, idp.MetadataURL, idp.Certificate,
		idp.Scopes, idp.RedirectURIs, idp.Enabled, idp.DefaultRole, groupMappingsJSON,
		idp.ID,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "identity provider not found")
	}
	return nil
}

func (r *IdentityProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, "DELETE FROM identity_providers WHERE id = $1", id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return errors.New(errors.NotFound, "identity provider not found")
	}
	return nil
}

func (r *IdentityProviderRepository) scanIdentityProviders(rows pgx.Rows) ([]*domain.IdentityProvider, error) {
	var idps []*domain.IdentityProvider
	for rows.Next() {
		var idp domain.IdentityProvider
		var groupMappingsJSON []byte

		err := rows.Scan(
			&idp.ID, &idp.Name, &idp.Type, &idp.Scope, &idp.TenantID,
			&idp.ClientID, &idp.ClientSecret, &idp.IssuerURL, &idp.DiscoveryURL,
			&idp.EntityID, &idp.SSOURL, &idp.MetadataURL, &idp.Certificate,
			&idp.Scopes, &idp.RedirectURIs, &idp.Enabled, &idp.DefaultRole, &groupMappingsJSON,
			&idp.CreatedAt, &idp.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(groupMappingsJSON, &idp.GroupMapping); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to unmarshal group mappings", err)
		}

		idps = append(idps, &idp)
	}
	return idps, rows.Err()
}