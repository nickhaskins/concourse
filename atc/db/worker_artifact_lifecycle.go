package db

import (
	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . WorkerArtifactLifecycle

type WorkerArtifactLifecycle interface {
	RemoveExpiredArtifacts() error
	RemoveUnassociatedArtifacts() error
}

type artifactLifecycle struct {
	conn Conn
}

func NewArtifactLifecycle(conn Conn) *artifactLifecycle {
	return &artifactLifecycle{
		conn: conn,
	}
}

func (lifecycle *artifactLifecycle) RemoveExpiredArtifacts() error {

	_, err := psql.Delete("worker_artifacts").
		Where(sq.Expr("created_at < NOW() - interval '12 hours'")).
		RunWith(lifecycle.conn).
		Exec()

	return err
}

func (lifecycle *artifactLifecycle) RemoveUnassociatedArtifacts() error {
	query, args, err := psql.Delete("worker_artifacts USING workers").
		Where(
			sq.Expr("worker_artifacts.worker_name = workers.name"),
		).
		Where(
			sq.Eq{
				"build_id":                     nil,
				"worker_resource_cache_id":     nil,
				"worker_task_cache_id":         nil,
				"worker_base_resource_type_id": nil,
				"worker_resource_certs_id":     nil,
			}).
		Where(sq.Or{
			sq.Expr("workers.state = 'running'::worker_state"),
			sq.Expr("workers.state = 'landing'::worker_state"),
			sq.Expr("workers.state = 'retiring'::worker_state"),
		}).
		RunWith(lifecycle.conn).
		ToSql()

	if err != nil {
		return err
	}

	rows, err := lifecycle.conn.Query(query, args...)
	if err != nil {
		return err
	}
	defer Close(rows)
	return nil
}
