package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
)

type SecurityGroupRepository struct {
	db *pgxpool.Pool
}

// NewSecurityGroupRepository creates a SecurityGroupRepository that uses the provided pgxpool.Pool for database operations.
func NewSecurityGroupRepository(db *pgxpool.Pool) *SecurityGroupRepository {
	return &SecurityGroupRepository{db: db}
}

func (r *SecurityGroupRepository) Create(ctx context.Context, sg *domain.SecurityGroup) error {
	query := `
		INSERT INTO security_groups (id, user_id, vpc_id, name, description, arn, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query, sg.ID, sg.UserID, sg.VPCID, sg.Name, sg.Description, sg.ARN, sg.CreatedAt)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to create security group", err)
	}
	return nil
}

func (r *SecurityGroupRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.SecurityGroup, error) {
	userID := appcontext.UserIDFromContext(ctx)
	query := `SELECT id, user_id, vpc_id, name, description, arn, created_at FROM security_groups WHERE id = $1 AND user_id = $2`

	var sg domain.SecurityGroup
	err := r.db.QueryRow(ctx, query, id, userID).Scan(&sg.ID, &sg.UserID, &sg.VPCID, &sg.Name, &sg.Description, &sg.ARN, &sg.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New(errors.NotFound, "security group not found")
		}
		return nil, errors.Wrap(errors.Internal, "failed to get security group", err)
	}

	rules, err := r.getRulesForGroup(ctx, sg.ID)
	if err != nil {
		return nil, err
	}
	sg.Rules = rules

	return &sg, nil
}

func (r *SecurityGroupRepository) GetByName(ctx context.Context, vpcID uuid.UUID, name string) (*domain.SecurityGroup, error) {
	userID := appcontext.UserIDFromContext(ctx)
	query := `SELECT id, user_id, vpc_id, name, description, arn, created_at FROM security_groups WHERE vpc_id = $1 AND name = $2 AND user_id = $3`

	var sg domain.SecurityGroup
	err := r.db.QueryRow(ctx, query, vpcID, name, userID).Scan(&sg.ID, &sg.UserID, &sg.VPCID, &sg.Name, &sg.Description, &sg.ARN, &sg.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.New(errors.NotFound, "security group not found")
		}
		return nil, errors.Wrap(errors.Internal, "failed to get security group by name", err)
	}

	rules, err := r.getRulesForGroup(ctx, sg.ID)
	if err != nil {
		return nil, err
	}
	sg.Rules = rules

	return &sg, nil
}

func (r *SecurityGroupRepository) ListByVPC(ctx context.Context, vpcID uuid.UUID) ([]*domain.SecurityGroup, error) {
	userID := appcontext.UserIDFromContext(ctx)
	query := `SELECT id, user_id, vpc_id, name, description, arn, created_at FROM security_groups WHERE vpc_id = $1 AND user_id = $2 ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, vpcID, userID)
	if err != nil {
		return nil, errors.Wrap(errors.Internal, "failed to list security groups", err)
	}
	defer rows.Close()

	var groups []*domain.SecurityGroup
	for rows.Next() {
		var sg domain.SecurityGroup
		if err := rows.Scan(&sg.ID, &sg.UserID, &sg.VPCID, &sg.Name, &sg.Description, &sg.ARN, &sg.CreatedAt); err != nil {
			return nil, errors.Wrap(errors.Internal, "failed to scan security group", err)
		}
		groups = append(groups, &sg)
	}
	return groups, nil
}

func (r *SecurityGroupRepository) AddRule(ctx context.Context, rule *domain.SecurityRule) error {
	query := `
		INSERT INTO security_rules (id, group_id, direction, protocol, port_min, port_max, cidr, priority, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query, rule.ID, rule.GroupID, rule.Direction, rule.Protocol, rule.PortMin, rule.PortMax, rule.CIDR, rule.Priority, rule.CreatedAt)
	if err != nil {
		return errors.Wrap(errors.Internal, "failed to add security rule", err)
	}
	return nil
}

func (r *SecurityGroupRepository) DeleteRule(ctx context.Context, ruleID uuid.UUID) error {
	query := `DELETE FROM security_rules WHERE id = $1`
	_, err := r.db.Exec(ctx, query, ruleID)
	return err
}

func (r *SecurityGroupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	userID := appcontext.UserIDFromContext(ctx)
	query := `DELETE FROM security_groups WHERE id = $1 AND user_id = $2`
	_, err := r.db.Exec(ctx, query, id, userID)
	return err
}

func (r *SecurityGroupRepository) AddInstanceToGroup(ctx context.Context, instanceID, groupID uuid.UUID) error {
	query := `INSERT INTO instance_security_groups (instance_id, group_id) VALUES ($1, $2)`
	_, err := r.db.Exec(ctx, query, instanceID, groupID)
	return err
}

func (r *SecurityGroupRepository) RemoveInstanceFromGroup(ctx context.Context, instanceID, groupID uuid.UUID) error {
	query := `DELETE FROM instance_security_groups WHERE instance_id = $1 AND group_id = $2`
	_, err := r.db.Exec(ctx, query, instanceID, groupID)
	return err
}

func (r *SecurityGroupRepository) ListInstanceGroups(ctx context.Context, instanceID uuid.UUID) ([]*domain.SecurityGroup, error) {
	query := `
		SELECT sg.id, sg.user_id, sg.vpc_id, sg.name, sg.description, sg.arn, sg.created_at 
		FROM security_groups sg
		JOIN instance_security_groups isg ON sg.id = isg.group_id
		WHERE isg.instance_id = $1
	`
	rows, err := r.db.Query(ctx, query, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*domain.SecurityGroup
	for rows.Next() {
		var sg domain.SecurityGroup
		if err := rows.Scan(&sg.ID, &sg.UserID, &sg.VPCID, &sg.Name, &sg.Description, &sg.ARN, &sg.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, &sg)
	}
	return groups, nil
}

func (r *SecurityGroupRepository) getRulesForGroup(ctx context.Context, groupID uuid.UUID) ([]domain.SecurityRule, error) {
	query := `SELECT id, group_id, direction, protocol, port_min, port_max, cidr, priority, created_at FROM security_rules WHERE group_id = $1 ORDER BY priority DESC`
	rows, err := r.db.Query(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []domain.SecurityRule
	for rows.Next() {
		var rule domain.SecurityRule
		if err := rows.Scan(&rule.ID, &rule.GroupID, &rule.Direction, &rule.Protocol, &rule.PortMin, &rule.PortMax, &rule.CIDR, &rule.Priority, &rule.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}