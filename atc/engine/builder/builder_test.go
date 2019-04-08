package builder_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/engine/builder"
	"github.com/concourse/concourse/atc/engine/builder/builderfakes"
	"github.com/concourse/concourse/atc/exec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type StepBuilder interface {
	BuildStep(db.Build) exec.Step
}

var _ = Describe("Builder", func() {
	var (
		fakeBuild           *dbfakes.FakeBuild
		fakeStepFactory     *builderfakes.FakeStepFactory
		fakeDelegateFactory *builderfakes.FakeDelegateFactory

		planFactory atc.PlanFactory
		stepBuilder StepBuilder

		expectedPlan     atc.Plan
		expectedMetadata builder.StepMetadata
		// step             exec.Step
	)

	BeforeEach(func() {
		fakeBuild = new(dbfakes.FakeBuild)
		fakeBuild.IDReturns(4444)
		fakeBuild.NameReturns("42")
		fakeBuild.JobNameReturns("some-job")
		fakeBuild.JobIDReturns(3333)
		fakeBuild.PipelineNameReturns("some-pipeline")
		fakeBuild.PipelineIDReturns(2222)
		fakeBuild.TeamNameReturns("some-team")
		fakeBuild.TeamIDReturns(1111)

		expectedMetadata = builder.StepMetadata{
			BuildID:      4444,
			BuildName:    "42",
			JobName:      "some-job",
			PipelineName: "some-pipeline",
			TeamName:     "some-team",
			ExternalURL:  "http://example.com",
		}

		fakeStepFactory = new(builderfakes.FakeStepFactory)
		fakeDelegateFactory = new(builderfakes.FakeDelegateFactory)

		stepBuilder = builder.NewStepBuilder(
			fakeStepFactory,
			fakeDelegateFactory,
			"http://example.com",
		)

		planFactory = atc.NewPlanFactory(123)
	})

	Describe("BuildStep", func() {

		JustBeforeEach(func() {
			fakeBuild.PrivatePlanReturns(expectedPlan)

			stepBuilder.BuildStep(fakeBuild)
			// step = stepBuilder.BuildStep(fakeBuild)
			// Expect(step).NotTo(BeNil())
		})

		Describe("with a putget in an aggregate", func() {
			var (
				putPlan               atc.Plan
				dependentGetPlan      atc.Plan
				otherPutPlan          atc.Plan
				otherDependentGetPlan atc.Plan
			)

			BeforeEach(func() {
				putPlan = planFactory.NewPlan(atc.PutPlan{
					Name:     "some-put",
					Resource: "some-output-resource",
					Type:     "put",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"some": "params"},
				})

				otherPutPlan = planFactory.NewPlan(atc.PutPlan{
					Name:     "some-put-2",
					Resource: "some-output-resource-2",
					Type:     "put",
					Source:   atc.Source{"some": "source-2"},
					Params:   atc.Params{"some": "params-2"},
				})

				expectedPlan = planFactory.NewPlan(atc.AggregatePlan{
					planFactory.NewPlan(atc.OnSuccessPlan{
						Step: putPlan,
						Next: dependentGetPlan,
					}),
					planFactory.NewPlan(atc.OnSuccessPlan{
						Step: otherPutPlan,
						Next: otherDependentGetPlan,
					}),
				})
			})

			Context("constructing outputs", func() {
				It("constructs the put correctly", func() {
					plan, build, stepMetadata, containerMetadata, _ := fakeStepFactory.PutStepArgsForCall(0)
					Expect(build).To(Equal(fakeBuild))
					Expect(plan).To(Equal(putPlan))
					Expect(stepMetadata).To(Equal(expectedMetadata))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						Type:         db.ContainerTypePut,
						StepName:     "some-put",
						PipelineID:   2222,
						PipelineName: "some-pipeline",
						JobID:        3333,
						JobName:      "some-job",
						BuildID:      4444,
						BuildName:    "42",
					}))

					plan, build, stepMetadata, containerMetadata, _ = fakeStepFactory.PutStepArgsForCall(1)
					Expect(build).To(Equal(fakeBuild))
					Expect(plan).To(Equal(otherPutPlan))
					Expect(stepMetadata).To(Equal(expectedMetadata))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						Type:         db.ContainerTypePut,
						StepName:     "some-put-2",
						PipelineID:   2222,
						PipelineName: "some-pipeline",
						JobID:        3333,
						JobName:      "some-job",
						BuildID:      4444,
						BuildName:    "42",
					}))
				})
			})
		})

		Context("with a retry plan", func() {
			var (
				getPlan       atc.Plan
				taskPlan      atc.Plan
				aggregatePlan atc.Plan
				doPlan        atc.Plan
				timeoutPlan   atc.Plan
				retryPlanTwo  atc.Plan
			)

			BeforeEach(func() {
				getPlan = planFactory.NewPlan(atc.GetPlan{
					Name:     "some-get",
					Resource: "some-input-resource",
					Type:     "get",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"some": "params"},
				})

				taskPlan = planFactory.NewPlan(atc.TaskPlan{
					Name:       "some-task",
					Privileged: false,
					Tags:       atc.Tags{"some", "task", "tags"},
					ConfigPath: "some-config-path",
				})

				retryPlanTwo = planFactory.NewPlan(atc.RetryPlan{
					taskPlan,
					taskPlan,
				})

				aggregatePlan = planFactory.NewPlan(atc.AggregatePlan{retryPlanTwo})

				doPlan = planFactory.NewPlan(atc.DoPlan{aggregatePlan})

				timeoutPlan = planFactory.NewPlan(atc.TimeoutPlan{
					Step:     doPlan,
					Duration: "1m",
				})

				expectedPlan = planFactory.NewPlan(atc.RetryPlan{
					getPlan,
					timeoutPlan,
					getPlan,
				})
			})

			It("constructs the retry correctly", func() {
				Expect(*expectedPlan.Retry).To(HaveLen(3))
			})

			It("constructs the first get correctly", func() {
				plan, build, stepMetadata, containerMetadata, _ := fakeStepFactory.GetStepArgsForCall(0)
				Expect(build).To(Equal(fakeBuild))
				expectedPlan := getPlan
				expectedPlan.Attempts = []int{1}
				Expect(plan).To(Equal(expectedPlan))
				Expect(stepMetadata).To(Equal(expectedMetadata))
				Expect(containerMetadata).To(Equal(db.ContainerMetadata{
					Type:         db.ContainerTypeGet,
					StepName:     "some-get",
					PipelineID:   2222,
					PipelineName: "some-pipeline",
					JobID:        3333,
					JobName:      "some-job",
					BuildID:      4444,
					BuildName:    "42",
					Attempt:      "1",
				}))
			})

			It("constructs the second get correctly", func() {
				plan, build, stepMetadata, containerMetadata, _ := fakeStepFactory.GetStepArgsForCall(1)
				Expect(build).To(Equal(fakeBuild))
				expectedPlan := getPlan
				expectedPlan.Attempts = []int{3}
				Expect(plan).To(Equal(expectedPlan))
				Expect(stepMetadata).To(Equal(expectedMetadata))
				Expect(containerMetadata).To(Equal(db.ContainerMetadata{
					Type:         db.ContainerTypeGet,
					StepName:     "some-get",
					PipelineID:   2222,
					PipelineName: "some-pipeline",
					JobID:        3333,
					JobName:      "some-job",
					BuildID:      4444,
					BuildName:    "42",
					Attempt:      "3",
				}))
			})

			It("constructs nested retries correctly", func() {
				Expect(*retryPlanTwo.Retry).To(HaveLen(2))
			})

			It("constructs nested steps correctly", func() {
				plan, build, containerMetadata, _ := fakeStepFactory.TaskStepArgsForCall(0)
				Expect(build).To(Equal(fakeBuild))
				expectedPlan := taskPlan
				expectedPlan.Attempts = []int{2, 1}
				Expect(plan).To(Equal(expectedPlan))
				Expect(containerMetadata).To(Equal(db.ContainerMetadata{
					Type:         db.ContainerTypeTask,
					StepName:     "some-task",
					PipelineID:   2222,
					PipelineName: "some-pipeline",
					JobID:        3333,
					JobName:      "some-job",
					BuildID:      4444,
					BuildName:    "42",
					Attempt:      "2.1",
				}))

				plan, build, containerMetadata, _ = fakeStepFactory.TaskStepArgsForCall(1)
				Expect(build).To(Equal(fakeBuild))
				expectedPlan = taskPlan
				expectedPlan.Attempts = []int{2, 2}
				Expect(plan).To(Equal(expectedPlan))
				Expect(containerMetadata).To(Equal(db.ContainerMetadata{
					Type:         db.ContainerTypeTask,
					StepName:     "some-task",
					PipelineID:   2222,
					PipelineName: "some-pipeline",
					JobID:        3333,
					JobName:      "some-job",
					BuildID:      4444,
					BuildName:    "42",
					Attempt:      "2.2",
				}))
			})
		})

		Context("with a plan where conditional steps are inside retries", func() {
			var (
				onAbortPlan   atc.Plan
				onErrorPlan   atc.Plan
				onSuccessPlan atc.Plan
				onFailurePlan atc.Plan
				ensurePlan    atc.Plan
				leafPlan      atc.Plan
				err           error
			)

			BeforeEach(func() {
				leafPlan = planFactory.NewPlan(atc.TaskPlan{
					Name:       "some-task",
					Privileged: false,
					Tags:       atc.Tags{"some", "task", "tags"},
					ConfigPath: "some-config-path",
				})

				onAbortPlan = planFactory.NewPlan(atc.OnAbortPlan{
					Step: leafPlan,
					Next: leafPlan,
				})

				onErrorPlan = planFactory.NewPlan(atc.OnErrorPlan{
					Step: onAbortPlan,
					Next: leafPlan,
				})

				onSuccessPlan = planFactory.NewPlan(atc.OnSuccessPlan{
					Step: onErrorPlan,
					Next: leafPlan,
				})

				onFailurePlan = planFactory.NewPlan(atc.OnFailurePlan{
					Step: onSuccessPlan,
					Next: leafPlan,
				})

				ensurePlan = planFactory.NewPlan(atc.EnsurePlan{
					Step: onFailurePlan,
					Next: leafPlan,
				})

				expectedPlan = planFactory.NewPlan(atc.RetryPlan{
					ensurePlan,
				})
			})

			It("constructs nested steps correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(fakeStepFactory.TaskStepCallCount()).To(Equal(6))

				_, _, containerMetadata, _ := fakeStepFactory.TaskStepArgsForCall(0)
				Expect(containerMetadata.Attempt).To(Equal("1"))
				_, _, containerMetadata, _ = fakeStepFactory.TaskStepArgsForCall(1)
				Expect(containerMetadata.Attempt).To(Equal("1"))
				_, _, containerMetadata, _ = fakeStepFactory.TaskStepArgsForCall(2)
				Expect(containerMetadata.Attempt).To(Equal("1"))
				_, _, containerMetadata, _ = fakeStepFactory.TaskStepArgsForCall(3)
				Expect(containerMetadata.Attempt).To(Equal("1"))
				_, _, containerMetadata, _ = fakeStepFactory.TaskStepArgsForCall(4)
				Expect(containerMetadata.Attempt).To(Equal("1"))
			})
		})

		Context("with a basic plan", func() {

			Context("that contains inputs", func() {
				BeforeEach(func() {
					expectedPlan = planFactory.NewPlan(atc.GetPlan{
						Name:     "some-input",
						Resource: "some-input-resource",
						Type:     "get",
						Tags:     []string{"some", "get", "tags"},
						Version:  &atc.Version{"some": "version"},
						Source:   atc.Source{"some": "source"},
						Params:   atc.Params{"some": "params"},
					})

					fakeBuild.PrivatePlanReturns(expectedPlan)
				})

				It("constructs inputs correctly", func() {
					plan, dBuild, stepMetadata, containerMetadata, _ := fakeStepFactory.GetStepArgsForCall(0)
					Expect(dBuild).To(Equal(fakeBuild))
					Expect(plan).To(Equal(expectedPlan))
					Expect(stepMetadata).To(Equal(expectedMetadata))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						Type:         db.ContainerTypeGet,
						StepName:     "some-input",
						PipelineID:   2222,
						PipelineName: "some-pipeline",
						JobID:        3333,
						JobName:      "some-job",
						BuildID:      4444,
						BuildName:    "42",
					}))
				})
			})

			Context("that contains tasks", func() {
				BeforeEach(func() {
					expectedPlan = planFactory.NewPlan(atc.TaskPlan{
						Name:          "some-task",
						ConfigPath:    "some-input/build.yml",
						InputMapping:  map[string]string{"foo": "bar"},
						OutputMapping: map[string]string{"baz": "qux"},
					})

					fakeBuild.PrivatePlanReturns(expectedPlan)
				})

				It("constructs tasks correctly", func() {
					plan, build, containerMetadata, _ := fakeStepFactory.TaskStepArgsForCall(0)
					Expect(build).To(Equal(fakeBuild))
					Expect(plan).To(Equal(expectedPlan))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						Type:         db.ContainerTypeTask,
						StepName:     "some-task",
						PipelineID:   2222,
						PipelineName: "some-pipeline",
						JobID:        3333,
						JobName:      "some-job",
						BuildID:      4444,
						BuildName:    "42",
					}))
				})
			})

			Context("that contains outputs", func() {
				var (
					putPlan          atc.Plan
					dependentGetPlan atc.Plan
				)

				BeforeEach(func() {
					putPlan = planFactory.NewPlan(atc.PutPlan{
						Name:     "some-put",
						Resource: "some-output-resource",
						Tags:     []string{"some", "putget", "tags"},
						Type:     "put",
						Source:   atc.Source{"some": "source"},
						Params:   atc.Params{"some": "params"},
					})

					dependentGetPlan = planFactory.NewPlan(atc.GetPlan{
						Name:        "some-get",
						Resource:    "some-input-resource",
						Tags:        []string{"some", "putget", "tags"},
						Type:        "get",
						VersionFrom: &putPlan.ID,
						Source:      atc.Source{"some": "source"},
						Params:      atc.Params{"another": "params"},
					})

					expectedPlan = planFactory.NewPlan(atc.OnSuccessPlan{
						Step: putPlan,
						Next: dependentGetPlan,
					})
				})

				It("constructs the put correctly", func() {
					plan, build, stepMetadata, containerMetadata, _ := fakeStepFactory.PutStepArgsForCall(0)
					Expect(build).To(Equal(fakeBuild))
					Expect(plan).To(Equal(putPlan))
					Expect(stepMetadata).To(Equal(expectedMetadata))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						Type:         db.ContainerTypePut,
						StepName:     "some-put",
						PipelineID:   2222,
						PipelineName: "some-pipeline",
						JobID:        3333,
						JobName:      "some-job",
						BuildID:      4444,
						BuildName:    "42",
					}))
				})

				It("constructs the dependent get correctly", func() {
					plan, build, stepMetadata, containerMetadata, _ := fakeStepFactory.GetStepArgsForCall(0)
					Expect(build).To(Equal(fakeBuild))
					Expect(plan).To(Equal(dependentGetPlan))
					Expect(stepMetadata).To(Equal(expectedMetadata))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						Type:         db.ContainerTypeGet,
						StepName:     "some-get",
						PipelineID:   2222,
						PipelineName: "some-pipeline",
						JobID:        3333,
						JobName:      "some-job",
						BuildID:      4444,
						BuildName:    "42",
					}))
				})
			})
		})

		Context("running hooked composes", func() {
			Context("with all the hooks", func() {
				var (
					inputPlan          atc.Plan
					failureTaskPlan    atc.Plan
					successTaskPlan    atc.Plan
					completionTaskPlan atc.Plan
					nextTaskPlan       atc.Plan
				)

				BeforeEach(func() {
					inputPlan = planFactory.NewPlan(atc.GetPlan{
						Name: "some-input",
					})
					failureTaskPlan = planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-failure-task",
						Config: &atc.TaskConfig{},
					})
					successTaskPlan = planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-success-task",
						Config: &atc.TaskConfig{},
					})
					completionTaskPlan = planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-completion-task",
						Config: &atc.TaskConfig{},
					})
					nextTaskPlan = planFactory.NewPlan(atc.TaskPlan{
						Name:   "some-next-task",
						Config: &atc.TaskConfig{},
					})

					expectedPlan = planFactory.NewPlan(atc.OnSuccessPlan{
						Step: planFactory.NewPlan(atc.EnsurePlan{
							Step: planFactory.NewPlan(atc.OnSuccessPlan{
								Step: planFactory.NewPlan(atc.OnFailurePlan{
									Step: inputPlan,
									Next: failureTaskPlan,
								}),
								Next: successTaskPlan,
							}),
							Next: completionTaskPlan,
						}),
						Next: nextTaskPlan,
					})
				})

				It("constructs the step correctly", func() {
					Expect(fakeStepFactory.GetStepCallCount()).To(Equal(1))
					plan, build, stepMetadata, containerMetadata, _ := fakeStepFactory.GetStepArgsForCall(0)
					Expect(build).To(Equal(fakeBuild))
					Expect(plan).To(Equal(inputPlan))
					Expect(stepMetadata).To(Equal(expectedMetadata))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   2222,
						PipelineName: "some-pipeline",
						JobID:        3333,
						JobName:      "some-job",
						BuildID:      4444,
						BuildName:    "42",
						StepName:     "some-input",
						Type:         db.ContainerTypeGet,
					}))
				})

				It("constructs the completion hook correctly", func() {
					Expect(fakeStepFactory.TaskStepCallCount()).To(Equal(4))
					plan, build, containerMetadata, _ := fakeStepFactory.TaskStepArgsForCall(2)
					Expect(build).To(Equal(fakeBuild))
					Expect(plan).To(Equal(completionTaskPlan))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   2222,
						PipelineName: "some-pipeline",
						JobID:        3333,
						JobName:      "some-job",
						BuildID:      4444,
						BuildName:    "42",
						StepName:     "some-completion-task",
						Type:         db.ContainerTypeTask,
					}))
				})

				It("constructs the failure hook correctly", func() {
					Expect(fakeStepFactory.TaskStepCallCount()).To(Equal(4))
					plan, build, containerMetadata, _ := fakeStepFactory.TaskStepArgsForCall(0)
					Expect(build).To(Equal(fakeBuild))
					Expect(plan).To(Equal(failureTaskPlan))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   2222,
						PipelineName: "some-pipeline",
						JobID:        3333,
						JobName:      "some-job",
						BuildID:      4444,
						BuildName:    "42",
						StepName:     "some-failure-task",
						Type:         db.ContainerTypeTask,
					}))
				})

				It("constructs the success hook correctly", func() {
					Expect(fakeStepFactory.TaskStepCallCount()).To(Equal(4))
					plan, build, containerMetadata, _ := fakeStepFactory.TaskStepArgsForCall(1)
					Expect(build).To(Equal(fakeBuild))
					Expect(plan).To(Equal(successTaskPlan))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   2222,
						PipelineName: "some-pipeline",
						JobID:        3333,
						JobName:      "some-job",
						BuildID:      4444,
						BuildName:    "42",
						StepName:     "some-success-task",
						Type:         db.ContainerTypeTask,
					}))
				})

				It("constructs the next step correctly", func() {
					Expect(fakeStepFactory.TaskStepCallCount()).To(Equal(4))
					plan, build, containerMetadata, _ := fakeStepFactory.TaskStepArgsForCall(3)
					Expect(build).To(Equal(fakeBuild))
					Expect(plan).To(Equal(nextTaskPlan))
					Expect(containerMetadata).To(Equal(db.ContainerMetadata{
						PipelineID:   2222,
						PipelineName: "some-pipeline",
						JobID:        3333,
						JobName:      "some-job",
						BuildID:      4444,
						BuildName:    "42",
						StepName:     "some-next-task",
						Type:         db.ContainerTypeTask,
					}))
				})
			})
		})

		Context("running try steps", func() {
			var inputPlan atc.Plan

			BeforeEach(func() {
				inputPlan = planFactory.NewPlan(atc.GetPlan{
					Name: "some-input",
				})

				expectedPlan = planFactory.NewPlan(atc.TryPlan{
					Step: inputPlan,
				})
			})

			It("constructs the step correctly", func() {
				Expect(fakeStepFactory.GetStepCallCount()).To(Equal(1))
				plan, build, stepMetadata, containerMetadata, _ := fakeStepFactory.GetStepArgsForCall(0)
				Expect(build).To(Equal(fakeBuild))
				Expect(plan).To(Equal(inputPlan))
				Expect(stepMetadata).To(Equal(expectedMetadata))
				Expect(containerMetadata).To(Equal(db.ContainerMetadata{
					Type:         db.ContainerTypeGet,
					StepName:     "some-input",
					PipelineID:   2222,
					PipelineName: "some-pipeline",
					JobID:        3333,
					JobName:      "some-job",
					BuildID:      4444,
					BuildName:    "42",
				}))
			})
		})
	})

	Describe("StepMetadata", func() {
		var stepMetadata builder.StepMetadata

		Describe("Env", func() {
			Context("when populating fields", func() {
				BeforeEach(func() {
					stepMetadata = builder.StepMetadata{
						BuildID:      1,
						PipelineName: "some-pipeline-name",
						JobName:      "some-job-name",
						BuildName:    "42",
						ExternalURL:  "http://www.example.com",
						TeamName:     "some-team",
					}
				})

				It("returns the specified values", func() {
					Expect(stepMetadata.Env()).To(Equal([]string{
						"BUILD_ID=1",
						"BUILD_PIPELINE_NAME=some-pipeline-name",
						"BUILD_JOB_NAME=some-job-name",
						"BUILD_NAME=42",
						"ATC_EXTERNAL_URL=http://www.example.com",
						"BUILD_TEAM_NAME=some-team",
					}))
				})
			})

			Context("when fields are empty", func() {
				BeforeEach(func() {
					stepMetadata = builder.StepMetadata{
						BuildID: 1,
					}
				})
				It("does not include fields that are not set", func() {
					Expect(stepMetadata.Env()).To(Equal([]string{
						"BUILD_ID=1",
					}))
				})
			})
		})
	})
})
