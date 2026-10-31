//go:build integration
// +build integration

package core_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"testing"

	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/testhelpers"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const nextNumber = `SELECT core.fn_next_document_number($1, $2, $3, $4, $5)`

// TestNextDocumentNumber_ConcurrentCallsNeverRepeat - 50 callers on one series get 1..50, each once.
func TestNextDocumentNumber_ConcurrentCallsNeverRepeat(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	tenant := uuid.New()

	numbers := make([]string, 50)
	var wg sync.WaitGroup
	for i := range numbers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := helper.DB().QueryRow(context.Background(), nextNumber, tenant, "test.order", "PED", 3, false).Scan(&numbers[i]); err != nil {
				t.Errorf("next number: %v", err)
			}
		}(i)
	}
	wg.Wait()

	sort.Strings(numbers)
	for i, number := range numbers {
		assert.Equal(t, fmt.Sprintf("PED-%03d", i+1), number)
	}
}

// TestNextDocumentNumber_RollbackGivesItBack - a document that fails leaves no gap (D1).
func TestNextDocumentNumber_RollbackGivesItBack(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	ctx := context.Background()
	tenant := uuid.New()

	tx, err := helper.DB().BeginTransaction(ctx)
	require.NoError(t, err)
	var first string
	require.NoError(t, tx.QueryRow(ctx, nextNumber, tenant, "test.order", "PED", 6, false).Scan(&first))
	assert.Equal(t, "PED-000001", first)
	require.NoError(t, tx.Rollback(ctx))

	var again string
	require.NoError(t, helper.DB().QueryRow(ctx, nextNumber, tenant, "test.order", "PED", 6, false).Scan(&again))
	assert.Equal(t, "PED-000001", again)
}

// TestNextDocumentNumber_SeriesAndTenantsAreApart - each tenant and each series counts on its own; a
// yearly series carries its year, so two years never repeat a number.
func TestNextDocumentNumber_SeriesAndTenantsAreApart(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()
	ctx := context.Background()
	one, other := uuid.New(), uuid.New()
	next := func(tenant uuid.UUID, series string, yearly bool) string {
		var number string
		require.NoError(t, helper.DB().QueryRow(ctx, nextNumber, tenant, series, "X", 2, yearly).Scan(&number))
		return number
	}

	assert.Equal(t, "X-01", next(one, "a", false))
	assert.Equal(t, "X-02", next(one, "a", false))
	assert.Equal(t, "X-01", next(one, "b", false))
	assert.Equal(t, "X-01", next(other, "a", false))
	assert.Regexp(t, `^X-20\d\d-01$`, next(one, "c", true))
}

// TestStatusOf - the naming rules, a registered exception over them, and the no-rows answer.
func TestStatusOf(t *testing.T) {
	pg := func(code string) error { return &pgconn.PgError{Message: code} }
	coreErrors.RegisterStatus("test.invalid-but-conflict", http.StatusConflict)

	for code, want := range map[string]int{
		"customer.not-found":         http.StatusNotFound,
		"customer.already-exists":    http.StatusConflict,
		"sales-order.invalid-status": http.StatusConflict,
		"validation.required":        http.StatusBadRequest,
		"order-item.invalid-qty":     http.StatusBadRequest,
		"stock.insufficient-units":   http.StatusConflict,
		"test.invalid-but-conflict":  http.StatusConflict,
		"something.else":             http.StatusInternalServerError,
	} {
		assert.Equal(t, want, coreErrors.StatusOf(pg(code)), code)
	}
	assert.Equal(t, http.StatusNotFound, coreErrors.StatusOf(pgx.ErrNoRows))
	assert.Equal(t, http.StatusConflict, coreErrors.StatusOf(errors.New("wrapped already-exists")))
	assert.Equal(t, http.StatusInternalServerError, coreErrors.StatusOf(nil))
}
