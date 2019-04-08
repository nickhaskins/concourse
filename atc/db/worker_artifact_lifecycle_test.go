package db_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
)

var _ = Describe("WorkerArtifactLifecycle", func() {
	var workerArtifactLifecycle db.WorkerArtifactLifecycle

	BeforeEach(func() {
		workerArtifactLifecycle = db.NewArtifactLifecycle(dbConn)
	})

	Describe("RemoveExpiredArtifacts", func() {
		JustBeforeEach(func() {
			err := workerArtifactLifecycle.RemoveExpiredArtifacts()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("removes artifacts created more than 12 hours ago", func() {

			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO worker_artifacts(name, created_at) VALUES('some-name', NOW() - '13 hours'::interval)")
				Expect(err).ToNot(HaveOccurred())
			})

			It("removes the record", func() {
				var count int
				err := dbConn.QueryRow("SELECT count(*) from worker_artifacts").Scan(&count)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(0))
			})
		})

		Context("keeps artifacts for 12 hours", func() {

			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO worker_artifacts(name, created_at) VALUES('some-name', NOW() - '13 hours'::interval)")
				Expect(err).ToNot(HaveOccurred())

				_, err = dbConn.Exec("INSERT INTO worker_artifacts(name, created_at) VALUES('some-other-name', NOW())")
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not remove the record", func() {
				var count int
				err := dbConn.QueryRow("SELECT count(*) from worker_artifacts").Scan(&count)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(1))
			})
		})

		Describe("RemoveUnassociatedWorkerArtifacts", func() {
			JustBeforeEach(func() {
				err := workerArtifactLifecycle.RemoveUnassociatedArtifacts()
				Expect(err).ToNot(HaveOccurred())
			})

			EnsureArtifactWithoutAssociationIsRemoved := func() {
				It("removes the artifact without the association", func() {
					var count int
					err := dbConn.QueryRow("SELECT count(*) from worker_artifacts").Scan(&count)
					Expect(err).ToNot(HaveOccurred())
					Expect(count).To(Equal(1))
				})
			}
			Context("when the worker is in 'stalling' state", func() {

				BeforeEach(func() {
					stallingWorkerPayload := atc.Worker{
						ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
						Name:            "stalling-worker",
						GardenAddr:      "2.1.2.1:7777",
						BaggageclaimURL: "3.4.3.4:7878",
					}

					stallingWorker, err := workerFactory.SaveWorker(stallingWorkerPayload, -5*time.Minute)
					Expect(err).ToNot(HaveOccurred())
					_, err = dbConn.Exec(fmt.Sprintf("INSERT INTO worker_artifacts(name, created_at, worker_name) VALUES('some-name', NOW() - '1 hour'::interval, '%s')", defaultWorker.Name()))
					Expect(err).ToNot(HaveOccurred())

					_, err = dbConn.Exec(fmt.Sprintf("INSERT INTO worker_artifacts(name, created_at, worker_name) VALUES('some-name', NOW() - '1 hour'::interval, '%s')", stallingWorker.Name()))
					Expect(err).ToNot(HaveOccurred())

					_, err = workerLifecycle.StallUnresponsiveWorkers()
					Expect(err).ToNot(HaveOccurred())
				})

				EnsureArtifactWithoutAssociationIsRemoved()

			})

			Context("base resource types", func() {
				BeforeEach(func() {
					tx, err := dbConn.Begin()
					Expect(err).NotTo(HaveOccurred())

					workerResourceCerts, err := db.WorkerResourceCerts{
						WorkerName: defaultWorker.Name(),
						CertsPath:  "/etc/blah/blah/certs",
					}.FindOrCreate(tx)
					Expect(err).ToNot(HaveOccurred())

					_, err = tx.Exec(fmt.Sprintf("INSERT INTO worker_artifacts(name, created_at, worker_name, worker_resource_certs_id) VALUES('some-name', NOW() - '1 hour'::interval,'%s', %d)", defaultWorker.Name(), workerResourceCerts.ID))
					Expect(err).ToNot(HaveOccurred())

					_, err = tx.Exec(fmt.Sprintf("INSERT INTO worker_artifacts(name, created_at, worker_name) VALUES('some-other-name', NOW() - '1 hour'::interval, '%s')", defaultWorker.Name()))
					Expect(err).ToNot(HaveOccurred())

					err = tx.Commit()
					Expect(err).ToNot(HaveOccurred())
				})

				EnsureArtifactWithoutAssociationIsRemoved()
			})

			Context("removes artifacts when not associated to base resource types", func() {
				BeforeEach(func() {
					baseResourceType, found, err := workerBaseResourceTypeFactory.Find(
						"some-base-resource-type",
						defaultWorker,
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					_, err = dbConn.Exec(fmt.Sprintf("INSERT INTO worker_artifacts(name, created_at, worker_name, worker_base_resource_type_id) VALUES('some-name', NOW() - '1 hour'::interval,'%s', %d)", defaultWorker.Name(), baseResourceType.ID))
					Expect(err).ToNot(HaveOccurred())

					_, err = dbConn.Exec(fmt.Sprintf("INSERT INTO worker_artifacts(name, created_at, worker_name) VALUES('some-other-name', NOW() - '1 hour'::interval, '%s')", defaultWorker.Name()))
					Expect(err).ToNot(HaveOccurred())

				})

				EnsureArtifactWithoutAssociationIsRemoved()
			})

			Context("removes artifacts when not associated to task caches", func() {
				BeforeEach(func() {
					usedTaskCache, err := workerTaskCacheFactory.FindOrCreate(
						defaultJob.ID(),
						"somestep",
						"/some/task/cache",
						defaultWorker.Name(),
					)
					Expect(err).ToNot(HaveOccurred())

					_, err = dbConn.Exec(fmt.Sprintf("INSERT INTO worker_artifacts(name, created_at, worker_name, worker_task_cache_id) VALUES('some-name', NOW() - '1 hour'::interval,'%s', %d)", defaultWorker.Name(), usedTaskCache.ID))
					Expect(err).ToNot(HaveOccurred())

					_, err = dbConn.Exec(fmt.Sprintf("INSERT INTO worker_artifacts(name, created_at, worker_name) VALUES('some-other-name', NOW() - '1 hour'::interval, '%s')", defaultWorker.Name()))
					Expect(err).ToNot(HaveOccurred())

				})
				EnsureArtifactWithoutAssociationIsRemoved()
			})

			Context("removes artifacts when not associated to resource caches", func() {
				BeforeEach(func() {
					build, err := defaultTeam.CreateOneOffBuild()
					Expect(err).ToNot(HaveOccurred())

					resourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
						logger,
						db.ForBuild(build.ID()),
						"some-base-resource-type",
						atc.Version{"some": "version"},
						atc.Source{"some": "source"},
						atc.Params{},
						creds.VersionedResourceTypes{},
					)
					Expect(err).ToNot(HaveOccurred())

					workerResourceCache := db.WorkerResourceCache{
						ResourceCache: resourceCache,
						WorkerName:    defaultWorker.Name(),
					}

					tx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())
					defer tx.Rollback()

					usedCache, err := workerResourceCache.FindOrCreate(tx)
					Expect(err).ToNot(HaveOccurred())

					_, err = tx.Exec(fmt.Sprintf("INSERT INTO worker_artifacts(name, created_at, worker_name, worker_resource_cache_id) VALUES('some-name', NOW() - '1 hour'::interval,'%s', %d)", defaultWorker.Name(), usedCache.ID))
					Expect(err).ToNot(HaveOccurred())

					_, err = tx.Exec(fmt.Sprintf("INSERT INTO worker_artifacts(name, created_at, worker_name) VALUES('some-other-name', NOW() - '1 hour'::interval, '%s')", defaultWorker.Name()))
					Expect(err).ToNot(HaveOccurred())

					err = tx.Commit()
					Expect(err).ToNot(HaveOccurred())
				})

				EnsureArtifactWithoutAssociationIsRemoved()
			})
		})
	})
})
