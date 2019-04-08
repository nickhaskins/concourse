BEGIN;
  ALTER TABLE worker_artifacts
    ADD COLUMN worker_resource_cache_id integer,
    ADD CONSTRAINT worker_resource_cache_id_fkey FOREIGN KEY(worker_resource_cache_id) REFERENCES worker_resource_caches(id);

  ALTER TABLE worker_artifacts
    ADD COLUMN worker_task_cache_id integer,
    ADD CONSTRAINT worker_task_cache_id_fkey FOREIGN KEY(worker_task_cache_id) REFERENCES worker_task_caches(id);

  ALTER TABLE worker_artifacts
    ADD COLUMN worker_resource_certs_id integer,
    ADD CONSTRAINT worker_resource_certs_id_fkey FOREIGN KEY(worker_resource_certs_id) REFERENCES worker_resource_certs(id);

  ALTER TABLE worker_artifacts
    ADD COLUMN worker_base_resource_type_id integer,
    ADD CONSTRAINT worker_base_resource_type_id_fkey FOREIGN KEY(worker_base_resource_type_id) REFERENCES worker_base_resource_types(id);

  ALTER TABLE worker_artifacts
    ADD COLUMN worker_name text NOT NULL,
    ADD CONSTRAINT worker_name_fkey FOREIGN KEY(worker_name) REFERENCES workers(name);

  ALTER TABLE resource_cache_uses
    ADD COLUMN artifact_id integer,
    ADD CONSTRAINT worker_artifact_id_fkey FOREIGN KEY(artifact_id) REFERENCES worker_artifacts(id);

COMMIT;
