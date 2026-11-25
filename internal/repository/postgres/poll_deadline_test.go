package postgres_test

import (
	"github.com/ReilEgor/Votivo-vote-service/internal/domain/model"
	repository "github.com/ReilEgor/Votivo-vote-service/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Run only against a disposable database: migrations include the original UUID conversion.
func TestPollDeadlinePostgres(t *testing.T) {
	dsn := os.Getenv("VOTIVO_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set VOTIVO_TEST_DATABASE_URL to a disposable database")
	}
	pool, err := pgxpool.New(t.Context(), dsn)
	require.NoError(t, err)
	defer pool.Close()
	files, err := filepath.Glob("../../../migrations/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, file := range files {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(t.Context(), string(sql))
		require.NoError(t, err, file)
	}
	deadline := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	poll := model.Poll{ID: uuid.NewString(), Status: model.PollStatusActive, EndsAt: &deadline}
	repo := repository.NewPollRepository(pool)
	require.NoError(t, repo.Save(t.Context(), poll))
	loaded, err := repo.GetPoll(t.Context(), poll.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded.EndsAt)
	require.True(t, deadline.Equal(*loaded.EndsAt))
	require.NoError(t, repo.SetStatus(t.Context(), poll.ID, model.PollStatusClosed))
	require.NoError(t, repo.Save(t.Context(), poll))
	loaded, err = repo.GetPoll(t.Context(), poll.ID)
	require.NoError(t, err)
	require.Equal(t, model.PollStatusClosed, loaded.Status)
	legacy := model.Poll{ID: uuid.NewString(), Status: model.PollStatusActive}
	require.NoError(t, repo.Save(t.Context(), legacy))
	loaded, err = repo.GetPoll(t.Context(), legacy.ID)
	require.NoError(t, err)
	require.Nil(t, loaded.EndsAt)
}
