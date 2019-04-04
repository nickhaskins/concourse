package engine_test

import (
	"context"
	"errors"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	. "github.com/concourse/concourse/atc/engine"
	"github.com/concourse/concourse/atc/engine/enginefakes"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/exec/execfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("Engine", func() {
	var (
		logger lager.Logger

		fakeDBBuild     *dbfakes.FakeBuild
		fakeStepBuilder *enginefakes.FakeStepBuilder

		engine Engine
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")

		fakeDBBuild = new(dbfakes.FakeBuild)
		fakeDBBuild.IDReturns(128)

		fakeStepBuilder = new(enginefakes.FakeStepBuilder)

		engine = NewEngine(fakeStepBuilder)
	})

	Describe("LookupBuild", func() {
		var (
			foundBuild Build
			lookupErr  error
		)

		JustBeforeEach(func() {
			foundBuild, lookupErr = engine.LookupBuild(logger, fakeDBBuild)
		})

		It("succeeds", func() {
			Expect(lookupErr).NotTo(HaveOccurred())
		})

		It("returns a build", func() {
			Expect(foundBuild).NotTo(BeNil())
		})
	})

	Describe("Builds", func() {
		var build Build
		var cancelled bool
		var release chan bool

		BeforeEach(func() {

			ctx := context.Background()
			cancel := func() { cancelled = true }

			release = make(chan bool)
			trackedStates := new(sync.Map)
			waitGroup := new(sync.WaitGroup)

			build = NewBuild(
				ctx,
				cancel,
				fakeDBBuild,
				fakeStepBuilder,
				release,
				trackedStates,
				waitGroup,
			)
		})

		Describe("Resume", func() {
			var logger lager.Logger

			BeforeEach(func() {
				logger = lagertest.NewTestLogger("test")
			})

			JustBeforeEach(func() {
				build.Resume(logger)
			})

			Context("when acquiring the lock succeeds", func() {
				var fakeLock *lockfakes.FakeLock

				BeforeEach(func() {
					fakeLock = new(lockfakes.FakeLock)

					fakeDBBuild.AcquireTrackingLockReturns(fakeLock, true, nil)
				})

				Context("when the build is active", func() {
					BeforeEach(func() {
						fakeDBBuild.IsRunningReturns(true)
						fakeDBBuild.ReloadReturns(true, nil)
					})

					Context("when finding the notifier succeeds", func() {
						var fakeNotifier *dbfakes.FakeNotifier

						BeforeEach(func() {
							fakeNotifier = new(dbfakes.FakeNotifier)
							fakeNotifier.NotifyReturns(make(chan struct{}))

							fakeDBBuild.AbortNotifierReturns(fakeNotifier, nil)
						})

						Context("when converting the plan to a step succeeds", func() {
							var fakeStep *execfakes.FakeStep

							BeforeEach(func() {
								fakeStep = new(execfakes.FakeStep)

								fakeStepBuilder.BuildStepReturns(fakeStep, nil)
							})

							FContext("when builds are released", func() {
								BeforeEach(func() {

									fakeStep.RunStub = func(context.Context, exec.RunState) error {
										<-time.After(time.Second)
										return nil
									}

									go func() {
										release <- true
									}()
								})

								It("releases the lock", func() {
									Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
								})

								It("closes the notifier", func() {
									Expect(fakeNotifier.CloseCallCount()).To(Equal(1))
								})
							})

							Context("when listening for aborts succeeds", func() {
								var (
									aborts chan struct{}
								)

								BeforeEach(func() {
									aborts = make(chan struct{}, 1)

									fakeNotifier.NotifyReturns(aborts)
								})

								It("listens for aborts", func() {
									Expect(fakeDBBuild.AbortNotifierCallCount()).To(Equal(1))
								})

								Context("when the build is aborted", func() {

									BeforeEach(func() {
										go func() {
											aborts <- struct{}{}
										}()
									})

									It("releases the lock", func() {
										Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
									})

									It("closes the notifier", func() {
										Eventually(func() int {
											return fakeNotifier.CloseCallCount()
										}).Should(Equal(1))
									})

									// TODO flaky
									It("cancels the context", func() {
										Eventually(func() bool {
											return cancelled
										}).Should(BeTrue())
									})
								})
							})
						})

						Context("when listening for aborts fails", func() {
							disaster := errors.New("oh no!")

							BeforeEach(func() {
								fakeDBBuild.AbortNotifierReturns(nil, disaster)
							})

							It("releases the lock", func() {
								Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
							})
						})
					})
				})

				Context("when the build is not yet active", func() {
					BeforeEach(func() {
						fakeDBBuild.ReloadReturns(true, nil)
					})

					It("does not build the step", func() {
						Expect(fakeStepBuilder.BuildStepCallCount()).To(BeZero())
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})

				Context("when the build has already finished", func() {
					BeforeEach(func() {
						fakeDBBuild.ReloadReturns(true, nil)
						fakeDBBuild.StatusReturns(db.BuildStatusSucceeded)
					})

					It("does not build the step", func() {
						Expect(fakeStepBuilder.BuildStepCallCount()).To(BeZero())
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})

				Context("when the build is no longer in the database", func() {
					BeforeEach(func() {
						fakeDBBuild.ReloadReturns(false, nil)
					})

					It("does not build the step", func() {
						Expect(fakeStepBuilder.BuildStepCallCount()).To(BeZero())
					})

					It("releases the lock", func() {
						Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
					})
				})
			})

			Context("when acquiring the lock fails", func() {
				BeforeEach(func() {
					fakeDBBuild.AcquireTrackingLockReturns(nil, false, errors.New("no lock for you"))
				})

				It("does not build the step", func() {
					Expect(fakeStepBuilder.BuildStepCallCount()).To(BeZero())
				})
			})
		})
	})
})
