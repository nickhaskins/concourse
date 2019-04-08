BEGIN;
  ALTER TABLE worker_artifacts
    DROP COLUMN worker_resource_cache_id,
    DROP CONSTRAINT worker_resource_cache_id_fkey;

  ALTER TABLE worker_artifacts
    DROP COLUMN worker_task_cache_id,
    DROP CONSTRAINT worker_task_cache_id_fkey;

  ALTER TABLE worker_artifacts
    DROP COLUMN worker_resource_certs_id,
    DROP CONSTRAINT worker_resource_certs_id_fkey;

  ALTER TABLE worker_artifacts
    DROP COLUMN worker_base_resource_type_id,
    DROP CONSTRAINT worker_base_resource_type_id_fkey;

  ALTER TABLE resource_cache_uses
    DROP COLUMN artifact_id,
    DROP CONSTRAINT worker_artifact_id_fkey;

COMMIT;
