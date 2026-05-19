package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	theclouderrors "github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSgName         = "test-sg"
	selectSg           = "SELECT id, user_id, tenant_id, vpc_id, name, description, arn, created_at FROM security_groups"
	selectRule         = "SELECT id, group_id, direction, protocol, port_min, port_max, cidr, priority, created_at FROM security_rules"
	selectInstanceUser = "SELECT tenant_id FROM instances WHERE id = \\$1"
	selectSgUser       = "SELECT tenant_id FROM security_groups WHERE id = \\$1"
)

func TestSecurityGroupRepositoryCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		sg := &domain.SecurityGroup{
			ID:          uuid.New(),
			UserID:      uuid.New(),
			TenantID:    uuid.New(),
			VPCID:       uuid.New(),
			Name:        testSgName,
			Description: "desc",
			ARN:         "arn",
			CreatedAt:   time.Now(),
		}

		mock.ExpectExec("INSERT INTO security_groups").
			WithArgs(sg.ID, sg.UserID, sg.TenantID, sg.VPCID, sg.Name, sg.Description, sg.ARN, sg.CreatedAt).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err = repo.Create(context.Background(), sg)
		require.NoError(t, err)
	})

	t.Run(testDBError, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		sg := &domain.SecurityGroup{
			ID: uuid.New(),
		}

		mock.ExpectExec("INSERT INTO security_groups").
			WillReturnError(errors.New(testDBError))

		err = repo.Create(context.Background(), sg)
		require.Error(t, err)
	})
}

func TestSecurityGroupRepositoryGetByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()

		mock.ExpectQuery(selectSg).
			WithArgs(id, tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "vpc_id", "name", "description", "arn", "created_at"}).
				AddRow(id, userID, tenantID, uuid.New(), testSgName, "desc", "arn", now))

		mock.ExpectQuery(selectRule).
			WithArgs(id).
			WillReturnRows(pgxmock.NewRows([]string{"id", "group_id", "direction", "protocol", "port_min", "port_max", "cidr", "priority", "created_at"}).
				AddRow(uuid.New(), id, string(domain.RuleIngress), "tcp", 80, 80, testutil.TestAnyCIDR, 100, now))

		sg, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		if sg != nil {
			assert.Len(t, sg.Rules, 1)
		}
	})

	t.Run(testNotFound, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectQuery(selectSg).
			WithArgs(id, tenantID).
			WillReturnError(pgx.ErrNoRows)

		sg, err := repo.GetByID(ctx, id)
		require.Error(t, err)
		assert.Nil(t, sg)
		var target *theclouderrors.Error
		if errors.As(err, &target) {
			assert.Equal(t, theclouderrors.NotFound, target.Type)
		}
	})
}

func TestSecurityGroupRepositoryGetByName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		vpcID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()
		name := testSgName

		mock.ExpectQuery(selectSg).
			WithArgs(vpcID, name, tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "vpc_id", "name", "description", "arn", "created_at"}).
				AddRow(id, userID, tenantID, vpcID, name, "desc", "arn", now))

		mock.ExpectQuery(selectRule).
			WithArgs(id).
			WillReturnRows(pgxmock.NewRows([]string{"id", "group_id", "direction", "protocol", "port_min", "port_max", "cidr", "priority", "created_at"}).
				AddRow(uuid.New(), id, string(domain.RuleIngress), "tcp", 80, 80, testutil.TestAnyCIDR, 100, now))

		sg, err := repo.GetByName(ctx, vpcID, name)
		require.NoError(t, err)
		assert.NotNil(t, sg)
		assert.Equal(t, id, sg.ID)
	})

	t.Run(testNotFound, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		userID := uuid.New()
		tenantID := uuid.New()
		vpcID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		name := testSgName

		mock.ExpectQuery(selectSg).
			WithArgs(vpcID, name, tenantID).
			WillReturnError(pgx.ErrNoRows)

		sg, err := repo.GetByName(ctx, vpcID, name)
		require.Error(t, err)
		assert.Nil(t, sg)
		var target *theclouderrors.Error
		ok := errors.As(err, &target)
		if ok {
			assert.Equal(t, theclouderrors.NotFound, target.Type)
		}
	})
}

func TestSecurityGroupRepositoryGetByNameAcrossVPCs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		vpcID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()
		name := testSgName

		mock.ExpectQuery(selectSg).
			WithArgs(name, tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "vpc_id", "name", "description", "arn", "created_at"}).
				AddRow(id, userID, tenantID, vpcID, name, "desc", "arn", now))

		mock.ExpectQuery(selectRule).
			WithArgs(id).
			WillReturnRows(pgxmock.NewRows([]string{"id", "group_id", "direction", "protocol", "port_min", "port_max", "cidr", "priority", "created_at"}).
				AddRow(uuid.New(), id, string(domain.RuleIngress), "tcp", 80, 80, testutil.TestAnyCIDR, 100, now))

		sg, err := repo.GetByNameAcrossVPCs(ctx, name)
		require.NoError(t, err)
		assert.NotNil(t, sg)
		assert.Equal(t, id, sg.ID)
	})

	t.Run(testNotFound, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		name := testSgName

		mock.ExpectQuery(selectSg).
			WithArgs(name, tenantID).
			WillReturnError(pgx.ErrNoRows)

		sg, err := repo.GetByNameAcrossVPCs(ctx, name)
		require.Error(t, err)
		assert.Nil(t, sg)
		var target *theclouderrors.Error
		ok := errors.As(err, &target)
		if ok {
			assert.Equal(t, theclouderrors.NotFound, target.Type)
		}
	})
}

func TestSecurityGroupRepositoryListByVPC(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		vpcID := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()

		mock.ExpectQuery(selectSg).
			WithArgs(vpcID, tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "vpc_id", "name", "description", "arn", "created_at"}).
				AddRow(uuid.New(), userID, tenantID, vpcID, testSgName, "desc", "arn", now))

		groups, err := repo.ListByVPC(ctx, vpcID)
		require.NoError(t, err)
		assert.Len(t, groups, 1)
	})

	t.Run(testDBError, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		vpcID := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectQuery(selectSg).
			WithArgs(vpcID, tenantID).
			WillReturnError(errors.New(testDBError))

		groups, err := repo.ListByVPC(ctx, vpcID)
		require.Error(t, err)
		assert.Nil(t, groups)
	})
}

func TestSecurityGroupRepositoryAddRule(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		rule := &domain.SecurityRule{
			ID:        uuid.New(),
			GroupID:   uuid.New(),
			Direction: domain.RuleIngress,
			Protocol:  "tcp",
			PortMin:   80,
			PortMax:   80,
			CIDR:      testutil.TestAnyCIDR,
			Priority:  100,
			CreatedAt: time.Now(),
		}

		mock.ExpectExec("INSERT INTO security_rules").
			WithArgs(rule.ID, rule.GroupID, rule.Direction, rule.Protocol, rule.PortMin, rule.PortMax, rule.CIDR, rule.Priority, rule.CreatedAt).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err = repo.AddRule(context.Background(), rule)
		require.NoError(t, err)
	})

	t.Run(testDBError, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		rule := &domain.SecurityRule{
			ID: uuid.New(),
		}

		mock.ExpectExec("INSERT INTO security_rules").
			WillReturnError(errors.New(testDBError))

		err = repo.AddRule(context.Background(), rule)
		require.Error(t, err)
	})
}

func TestSecurityGroupRepositoryDeleteRule(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		ruleID := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectExec("DELETE FROM security_rules").
			WithArgs(ruleID, tenantID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		err = repo.DeleteRule(ctx, ruleID)
		require.NoError(t, err)
	})

	t.Run(testDBError, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		ruleID := uuid.New()
		tenantID := uuid.New()

		mock.ExpectExec("DELETE FROM security_rules").
			WithArgs(ruleID, tenantID).
			WillReturnError(errors.New(testDBError))

		ctx := appcontext.WithTenantID(context.Background(), tenantID)
		err = repo.DeleteRule(ctx, ruleID)
		require.Error(t, err)
	})
}

func TestSecurityGroupRepositoryGetRuleByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		ruleID := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()

		rows := pgxmock.NewRows([]string{"id", "group_id", "direction", "protocol", "port_min", "port_max", "cidr", "priority", "created_at"}).
			AddRow(ruleID, uuid.New(), "ingress", "tcp", 80, 80, "0.0.0.0/0", 100, now)

		mock.ExpectQuery("SELECT .* FROM security_rules").
			WithArgs(ruleID, tenantID).
			WillReturnRows(rows)

		rule, err := repo.GetRuleByID(ctx, ruleID)
		require.NoError(t, err)
		assert.NotNil(t, rule)
		assert.Equal(t, ruleID, rule.ID)
	})

	t.Run("not found", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		ruleID := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectQuery("SELECT .* FROM security_rules").
			WithArgs(ruleID, tenantID).
			WillReturnError(pgx.ErrNoRows)

		rule, err := repo.GetRuleByID(ctx, ruleID)
		require.Error(t, err)
		assert.Nil(t, rule)
		var target theclouderrors.Error
		if errors.As(err, &target) {
			assert.Equal(t, theclouderrors.NotFound, target.Type)
		}
	})
}

func TestSecurityGroupRepositoryDelete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectExec("DELETE FROM security_groups").
			WithArgs(id, tenantID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		err = repo.Delete(ctx, id)
		require.NoError(t, err)
	})

	t.Run(testDBError, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectExec("DELETE FROM security_groups").
			WithArgs(id, tenantID).
			WillReturnError(errors.New(testDBError))

		err = repo.Delete(ctx, id)
		require.Error(t, err)
	})
}

func TestSecurityGroupRepositoryAddInstanceToGroup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		instanceID := uuid.New()
		groupID := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectBegin()
		mock.ExpectQuery(selectInstanceUser).
			WithArgs(instanceID).
			WillReturnRows(pgxmock.NewRows([]string{"tenant_id"}).AddRow(tenantID))
		mock.ExpectQuery(selectSgUser).
			WithArgs(groupID).
			WillReturnRows(pgxmock.NewRows([]string{"tenant_id"}).AddRow(tenantID))
		mock.ExpectExec("INSERT INTO instance_security_groups").
			WithArgs(instanceID, groupID).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()

		err = repo.AddInstanceToGroup(ctx, instanceID, groupID)
		require.NoError(t, err)
	})

	t.Run("instance not found", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		instanceID := uuid.New()
		groupID := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectBegin()
		mock.ExpectQuery(selectInstanceUser).
			WithArgs(instanceID).
			WillReturnError(pgx.ErrNoRows)
		mock.ExpectRollback()

		err = repo.AddInstanceToGroup(ctx, instanceID, groupID)
		require.Error(t, err)
		var target *theclouderrors.Error
		ok := errors.As(err, &target)
		if ok {
			assert.Equal(t, theclouderrors.NotFound, target.Type)
		}
	})

	t.Run("security group not found", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		instanceID := uuid.New()
		groupID := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectBegin()
		mock.ExpectQuery(selectInstanceUser).
			WithArgs(instanceID).
			WillReturnRows(pgxmock.NewRows([]string{"tenant_id"}).AddRow(tenantID))
		mock.ExpectQuery(selectSgUser).
			WithArgs(groupID).
			WillReturnError(pgx.ErrNoRows)
		mock.ExpectRollback()

		err = repo.AddInstanceToGroup(ctx, instanceID, groupID)
		require.Error(t, err)
		var target *theclouderrors.Error
		ok := errors.As(err, &target)
		if ok {
			assert.Equal(t, theclouderrors.NotFound, target.Type)
		}
	})
}

func TestSecurityGroupRepositoryRemoveInstanceFromGroup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		instanceID := uuid.New()
		groupID := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectBegin()
		mock.ExpectQuery(selectInstanceUser).
			WithArgs(instanceID).
			WillReturnRows(pgxmock.NewRows([]string{"tenant_id"}).AddRow(tenantID))
		mock.ExpectQuery(selectSgUser).
			WithArgs(groupID).
			WillReturnRows(pgxmock.NewRows([]string{"tenant_id"}).AddRow(tenantID))
		mock.ExpectExec("DELETE FROM instance_security_groups").
			WithArgs(instanceID, groupID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))
		mock.ExpectCommit()

		err = repo.RemoveInstanceFromGroup(ctx, instanceID, groupID)
		require.NoError(t, err)
	})

	t.Run("instance not found", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		instanceID := uuid.New()
		groupID := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectBegin()
		mock.ExpectQuery(selectInstanceUser).
			WithArgs(instanceID).
			WillReturnError(pgx.ErrNoRows)
		mock.ExpectRollback()

		err = repo.RemoveInstanceFromGroup(ctx, instanceID, groupID)
		require.Error(t, err)
		var target *theclouderrors.Error
		ok := errors.As(err, &target)
		if ok {
			assert.Equal(t, theclouderrors.NotFound, target.Type)
		}
	})
}

func TestSecurityGroupRepositoryListInstanceGroups(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		instanceID := uuid.New()
		now := time.Now()

		mock.ExpectQuery("SELECT sg.id, sg.user_id, sg.tenant_id, sg.vpc_id, sg.name, sg.description, sg.arn, sg.created_at").
			WithArgs(instanceID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "vpc_id", "name", "description", "arn", "created_at"}).
				AddRow(uuid.New(), uuid.New(), uuid.New(), uuid.New(), testSgName, "desc", "arn", now))

		groups, err := repo.ListInstanceGroups(context.Background(), instanceID)
		require.NoError(t, err)
		assert.Len(t, groups, 1)
	})

	t.Run(testDBError, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewSecurityGroupRepository(mock)
		instanceID := uuid.New()

		mock.ExpectQuery("SELECT sg.id, sg.user_id, sg.tenant_id, sg.vpc_id, sg.name, sg.description, sg.arn, sg.created_at").
			WithArgs(instanceID).
			WillReturnError(errors.New(testDBError))

		groups, err := repo.ListInstanceGroups(context.Background(), instanceID)
		require.Error(t, err)
		assert.Nil(t, groups)
	})
}
