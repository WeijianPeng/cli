package v3_test

import (
	"errors"

	"code.cloudfoundry.org/cli/actor/sharedaction"
	"code.cloudfoundry.org/cli/actor/v3action"
	"code.cloudfoundry.org/cli/command/commandfakes"
	"code.cloudfoundry.org/cli/command/translatableerror"
	"code.cloudfoundry.org/cli/command/v3"
	"code.cloudfoundry.org/cli/command/v3/v3fakes"
	"code.cloudfoundry.org/cli/types"
	"code.cloudfoundry.org/cli/util/configv3"
	"code.cloudfoundry.org/cli/util/ui"
	"code.cloudfoundry.org/cli/version"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("v3-scale Command", func() {
	var (
		cmd             v3.V3ScaleCommand
		input           *Buffer
		output          *Buffer
		testUI          *ui.UI
		fakeConfig      *commandfakes.FakeConfig
		fakeSharedActor *commandfakes.FakeSharedActor
		fakeActor       *v3fakes.FakeV3ScaleActor
		appName         string
		binaryName      string
		executeErr      error
	)

	BeforeEach(func() {
		input = NewBuffer()
		output = NewBuffer()
		testUI = ui.NewTestUI(input, output, NewBuffer())
		fakeConfig = new(commandfakes.FakeConfig)
		fakeSharedActor = new(commandfakes.FakeSharedActor)
		fakeActor = new(v3fakes.FakeV3ScaleActor)
		appName = "some-app"

		cmd = v3.V3ScaleCommand{
			UI:          testUI,
			Config:      fakeConfig,
			SharedActor: fakeSharedActor,
			Actor:       fakeActor,
		}

		binaryName = "faceman"
		fakeConfig.BinaryNameReturns(binaryName)

		cmd.RequiredArgs.AppName = appName
		cmd.ProcessType = "web"

		fakeActor.CloudControllerAPIVersionReturns(version.MinVersionV3)
	})

	JustBeforeEach(func() {
		executeErr = cmd.Execute(nil)
	})

	Context("when the API version is below the minimum", func() {
		BeforeEach(func() {
			fakeActor.CloudControllerAPIVersionReturns("0.0.0")
		})

		It("returns a MinimumAPIVersionNotMetError", func() {
			Expect(executeErr).To(MatchError(translatableerror.MinimumAPIVersionNotMetError{
				CurrentVersion: "0.0.0",
				MinimumVersion: version.MinVersionV3,
			}))
		})
	})

	Context("when checking target fails", func() {
		BeforeEach(func() {
			fakeSharedActor.CheckTargetReturns(sharedaction.NotLoggedInError{BinaryName: binaryName})
		})

		It("returns an error", func() {
			Expect(executeErr).To(MatchError(translatableerror.NotLoggedInError{BinaryName: binaryName}))

			Expect(fakeSharedActor.CheckTargetCallCount()).To(Equal(1))
			_, checkTargetedOrg, checkTargetedSpace := fakeSharedActor.CheckTargetArgsForCall(0)
			Expect(checkTargetedOrg).To(BeTrue())
			Expect(checkTargetedSpace).To(BeTrue())
		})
	})

	Context("when the user is logged in, and org and space are targeted", func() {
		BeforeEach(func() {
			fakeConfig.HasTargetedOrganizationReturns(true)
			fakeConfig.TargetedOrganizationReturns(configv3.Organization{Name: "some-org"})
			fakeConfig.HasTargetedSpaceReturns(true)
			fakeConfig.TargetedSpaceReturns(configv3.Space{
				GUID: "some-space-guid",
				Name: "some-space"})
			fakeConfig.CurrentUserReturns(
				configv3.User{Name: "some-user"},
				nil)
		})

		Context("when getting the current user returns an error", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("getting current user error")
				fakeConfig.CurrentUserReturns(
					configv3.User{},
					expectedErr)
			})

			It("returns the error", func() {
				Expect(executeErr).To(MatchError(expectedErr))
			})
		})

		Context("when the application does not exist", func() {
			BeforeEach(func() {
				fakeActor.GetApplicationByNameAndSpaceReturns(
					v3action.Application{},
					v3action.Warnings{"get-app-warning"},
					v3action.ApplicationNotFoundError{Name: appName})
			})

			It("returns an ApplicationNotFoundError and all warnings", func() {
				Expect(executeErr).To(Equal(translatableerror.ApplicationNotFoundError{Name: appName}))

				Expect(testUI.Out).ToNot(Say("Showing"))
				Expect(testUI.Out).ToNot(Say("Scaling"))
				Expect(testUI.Err).To(Say("get-app-warning"))

				Expect(fakeActor.GetApplicationByNameAndSpaceCallCount()).To(Equal(1))
				appNameArg, spaceGUIDArg := fakeActor.GetApplicationByNameAndSpaceArgsForCall(0)
				Expect(appNameArg).To(Equal(appName))
				Expect(spaceGUIDArg).To(Equal("some-space-guid"))
			})
		})

		Context("when an error occurs getting the application", func() {
			var expectedErr error

			BeforeEach(func() {
				expectedErr = errors.New("get app error")
				fakeActor.GetApplicationByNameAndSpaceReturns(
					v3action.Application{},
					v3action.Warnings{"get-app-warning"},
					expectedErr)
			})

			It("returns the error and displays all warnings", func() {
				Expect(executeErr).To(Equal(expectedErr))
				Expect(testUI.Err).To(Say("get-app-warning"))
			})
		})

		Context("when the application exists", func() {
			var process v3action.Process

			BeforeEach(func() {
				process = v3action.Process{
					Type:       "web",
					Instances:  types.NullInt{Value: 3, IsSet: true},
					MemoryInMB: types.NullUint64{Value: 32, IsSet: true},
					DiskInMB:   types.NullUint64{Value: 1024, IsSet: true},
				}

				fakeActor.GetApplicationByNameAndSpaceReturns(
					v3action.Application{GUID: "some-app-guid"},
					v3action.Warnings{"get-app-warning"},
					nil)
			})

			Context("when no flag options are provided", func() {
				BeforeEach(func() {
					fakeActor.GetProcessByApplicationAndProcessTypeReturns(
						process,
						v3action.Warnings{"get-instance-warning"},
						nil)
				})

				It("displays current scale properties and all warnings", func() {
					Expect(executeErr).ToNot(HaveOccurred())

					Expect(testUI.Out).ToNot(Say("Scaling"))
					Expect(testUI.Out).ToNot(Say("This will cause the app to restart"))
					Expect(testUI.Out).ToNot(Say("Stopping"))
					Expect(testUI.Out).ToNot(Say("Starting"))
					Expect(testUI.Out).ToNot(Say("Waiting"))
					Expect(testUI.Out).To(Say("Showing current scale of process web of app some-app in org some-org / space some-space as some-user\\.\\.\\."))

					Expect(testUI.Out).To(Say("memory:\\s+32M"))
					Expect(testUI.Out).To(Say("disk:\\s+1G"))
					Expect(testUI.Out).To(Say("instances:\\s+3"))

					Expect(testUI.Err).To(Say("get-app-warning"))
					Expect(testUI.Err).To(Say("get-instance-warning"))

					Expect(fakeActor.GetApplicationByNameAndSpaceCallCount()).To(Equal(1))
					appNameArg, spaceGUIDArg := fakeActor.GetApplicationByNameAndSpaceArgsForCall(0)
					Expect(appNameArg).To(Equal(appName))
					Expect(spaceGUIDArg).To(Equal("some-space-guid"))

					Expect(fakeActor.GetProcessByApplicationAndProcessTypeCallCount()).To(Equal(1))
					appGUIDArg, processTypeArg := fakeActor.GetProcessByApplicationAndProcessTypeArgsForCall(0)
					Expect(appGUIDArg).To(Equal("some-app-guid"))
					Expect(processTypeArg).To(Equal("web"))

					Expect(fakeActor.ScaleProcessByApplicationCallCount()).To(Equal(0))
				})

				Context("when an error is encountered getting process information", func() {
					var expectedErr error

					BeforeEach(func() {
						expectedErr = errors.New("get process error")
						fakeActor.GetProcessByApplicationAndProcessTypeReturns(
							v3action.Process{},
							v3action.Warnings{"get-process-warning"},
							expectedErr,
						)
					})

					It("returns the error and displays all warnings", func() {
						Expect(executeErr).To(Equal(expectedErr))
						Expect(testUI.Err).To(Say("get-process-warning"))
					})
				})
			})

			Context("when all flag options are provided", func() {
				BeforeEach(func() {
					cmd.Instances.Value = 2
					cmd.Instances.IsSet = true
					cmd.DiskLimit.Value = 50
					cmd.DiskLimit.IsSet = true
					cmd.MemoryLimit.Value = 100
					cmd.MemoryLimit.IsSet = true
					fakeActor.ScaleProcessByApplicationReturns(
						v3action.Warnings{"scale-warning"},
						nil)

					process = v3action.Process{
						Type:       "web",
						Instances:  types.NullInt{Value: 2, IsSet: true},
						MemoryInMB: types.NullUint64{Value: 50, IsSet: true},
						DiskInMB:   types.NullUint64{Value: 1024, IsSet: true},
					}
					fakeActor.GetProcessByApplicationAndProcessTypeReturns(
						process,
						v3action.Warnings{"get-instances-warning"},
						nil)
				})

				Context("when force flag is not provided", func() {
					Context("when given the choice to restart the app", func() {
						Context("when the user chooses default", func() {
							BeforeEach(func() {
								_, err := input.Write([]byte("\n"))
								Expect(err).ToNot(HaveOccurred())
							})

							It("does not scale the app", func() {
								Expect(executeErr).ToNot(HaveOccurred())

								Expect(testUI.Out).ToNot(Say("Showing"))
								Expect(testUI.Out).To(Say("Scaling process web of app some-app in org some-org / space some-space as some-user\\.\\.\\."))
								Expect(testUI.Out).To(Say("This will cause the app to restart\\. Are you sure you want to scale some-app\\? \\[yN\\]:"))
								Expect(testUI.Out).To(Say("Scaling cancelled"))
								Expect(testUI.Out).ToNot(Say("Stopping"))
								Expect(testUI.Out).ToNot(Say("Starting"))
								Expect(testUI.Out).ToNot(Say("Waiting"))

								Expect(testUI.Out).To(Say("memory:\\s+50M"))
								Expect(testUI.Out).To(Say("disk:\\s+1G"))
								Expect(testUI.Out).To(Say("instances:\\s+2"))

								Expect(fakeActor.ScaleProcessByApplicationCallCount()).To(Equal(0))
							})
						})

						Context("when the user chooses no", func() {
							BeforeEach(func() {
								_, err := input.Write([]byte("n\n"))
								Expect(err).ToNot(HaveOccurred())
							})

							It("does not scale the app", func() {
								Expect(executeErr).ToNot(HaveOccurred())

								Expect(testUI.Out).ToNot(Say("Showing"))
								Expect(testUI.Out).To(Say("Scaling process web of app some-app in org some-org / space some-space as some-user\\.\\.\\."))
								Expect(testUI.Out).To(Say("This will cause the app to restart\\. Are you sure you want to scale some-app\\? \\[yN\\]:"))
								Expect(testUI.Out).To(Say("Scaling cancelled"))
								Expect(testUI.Out).ToNot(Say("Stopping"))
								Expect(testUI.Out).ToNot(Say("Starting"))
								Expect(testUI.Out).ToNot(Say("Waiting"))

								Expect(fakeActor.ScaleProcessByApplicationCallCount()).To(Equal(0))
							})
						})

						Context("when the user chooses yes", func() {
							BeforeEach(func() {
								_, err := input.Write([]byte("y\n"))
								Expect(err).ToNot(HaveOccurred())
							})

							Context("when polling succeeds", func() {
								BeforeEach(func() {
									fakeActor.PollStartStub = func(appGUID string, warnings chan<- v3action.Warnings) error {
										warnings <- v3action.Warnings{"some-poll-warning-1", "some-poll-warning-2"}
										return nil
									}
								})

								It("scales, restarts, and displays scale properties", func() {
									Expect(executeErr).ToNot(HaveOccurred())

									Expect(testUI.Out).ToNot(Say("Showing"))
									Expect(testUI.Out).To(Say("Scaling process web of app some-app in org some-org / space some-space as some-user\\.\\.\\."))
									Expect(testUI.Out).To(Say("This will cause the app to restart\\. Are you sure you want to scale some-app\\? \\[yN\\]:"))
									Expect(testUI.Out).To(Say("Stopping app some-app in org some-org / space some-space as some-user\\.\\.\\."))
									Expect(testUI.Out).To(Say("Starting app some-app in org some-org / space some-space as some-user\\.\\.\\."))

									Expect(testUI.Out).To(Say("memory:\\s+50M"))
									Expect(testUI.Out).To(Say("disk:\\s+1G"))
									Expect(testUI.Out).To(Say("instances:\\s+2"))

									Expect(testUI.Err).To(Say("get-app-warning"))
									Expect(testUI.Err).To(Say("scale-warning"))
									Expect(testUI.Err).To(Say("some-poll-warning-1"))
									Expect(testUI.Err).To(Say("some-poll-warning-2"))
									Expect(testUI.Err).To(Say("get-instances-warning"))

									Expect(fakeActor.GetApplicationByNameAndSpaceCallCount()).To(Equal(1))
									appNameArg, spaceGUIDArg := fakeActor.GetApplicationByNameAndSpaceArgsForCall(0)
									Expect(appNameArg).To(Equal(appName))
									Expect(spaceGUIDArg).To(Equal("some-space-guid"))

									Expect(fakeActor.ScaleProcessByApplicationCallCount()).To(Equal(1))
									appGUIDArg, scaleProcess := fakeActor.ScaleProcessByApplicationArgsForCall(0)
									Expect(appGUIDArg).To(Equal("some-app-guid"))
									Expect(scaleProcess).To(Equal(v3action.Process{
										Type:       "web",
										Instances:  types.NullInt{Value: 2, IsSet: true},
										DiskInMB:   types.NullUint64{Value: 50, IsSet: true},
										MemoryInMB: types.NullUint64{Value: 100, IsSet: true},
									}))

									Expect(fakeActor.StopApplicationCallCount()).To(Equal(1))
									Expect(fakeActor.StopApplicationArgsForCall(0)).To(Equal("some-app-guid"))

									Expect(fakeActor.StartApplicationCallCount()).To(Equal(1))
									Expect(fakeActor.StartApplicationArgsForCall(0)).To(Equal("some-app-guid"))

									Expect(fakeActor.GetProcessByApplicationAndProcessTypeCallCount()).To(Equal(1))
									appGUID, processType := fakeActor.GetProcessByApplicationAndProcessTypeArgsForCall(0)
									Expect(appGUID).To(Equal("some-app-guid"))
									Expect(processType).To(Equal("web"))
								})
							})

							Context("when polling the start fails", func() {
								BeforeEach(func() {
									fakeActor.PollStartStub = func(appGUID string, warnings chan<- v3action.Warnings) error {
										warnings <- v3action.Warnings{"some-poll-warning-1", "some-poll-warning-2"}
										return errors.New("some-error")
									}
								})

								It("displays all warnings and fails", func() {
									Expect(testUI.Err).To(Say("some-poll-warning-1"))
									Expect(testUI.Err).To(Say("some-poll-warning-2"))

									Expect(executeErr).To(MatchError("some-error"))
								})
							})

							Context("when polling times out", func() {
								BeforeEach(func() {
									fakeActor.PollStartReturns(v3action.StartupTimeoutError{})
								})

								It("returns the StartupTimeoutError", func() {
									Expect(executeErr).To(MatchError(translatableerror.StartupTimeoutError{
										AppName:    "some-app",
										BinaryName: binaryName,
									}))
								})
							})
						})
					})
				})

				Context("when force flag is provided", func() {
					BeforeEach(func() {
						cmd.Force = true
					})

					It("does not prompt user to confirm app restart", func() {
						Expect(executeErr).ToNot(HaveOccurred())

						Expect(testUI.Out).To(Say("Scaling process web of app some-app in org some-org / space some-space as some-user\\.\\.\\."))
						Expect(testUI.Out).NotTo(Say("This will cause the app to restart\\. Are you sure you want to scale some-app\\? \\[yN\\]:"))
						Expect(testUI.Out).To(Say("Stopping app some-app in org some-org / space some-space as some-user\\.\\.\\."))
						Expect(testUI.Out).To(Say("Starting app some-app in org some-org / space some-space as some-user\\.\\.\\."))

						Expect(fakeActor.GetApplicationByNameAndSpaceCallCount()).To(Equal(1))
						Expect(fakeActor.ScaleProcessByApplicationCallCount()).To(Equal(1))
						Expect(fakeActor.StopApplicationCallCount()).To(Equal(1))
						Expect(fakeActor.StartApplicationCallCount()).To(Equal(1))
						Expect(fakeActor.GetProcessByApplicationAndProcessTypeCallCount()).To(Equal(1))
					})
				})

			})

			Context("when only the instances flag option is provided", func() {
				BeforeEach(func() {
					cmd.Instances.Value = 3
					cmd.Instances.IsSet = true
					fakeActor.ScaleProcessByApplicationReturns(
						v3action.Warnings{"scale-warning"},
						nil)
					fakeActor.GetProcessByApplicationAndProcessTypeReturns(
						process,
						v3action.Warnings{"get-instances-warning"},
						nil)
				})

				It("scales the number of instances, displays scale properties, and does not restart the application", func() {
					Expect(executeErr).ToNot(HaveOccurred())

					Expect(testUI.Out).ToNot(Say("Showing"))
					Expect(testUI.Out).To(Say("Scaling"))
					Expect(testUI.Out).NotTo(Say("This will cause the app to restart"))
					Expect(testUI.Out).NotTo(Say("Stopping"))
					Expect(testUI.Out).NotTo(Say("Starting"))

					Expect(testUI.Err).To(Say("get-app-warning"))
					Expect(testUI.Err).To(Say("scale-warning"))
					Expect(testUI.Err).To(Say("get-instances-warning"))

					Expect(fakeActor.GetApplicationByNameAndSpaceCallCount()).To(Equal(1))
					appNameArg, spaceGUIDArg := fakeActor.GetApplicationByNameAndSpaceArgsForCall(0)
					Expect(appNameArg).To(Equal(appName))
					Expect(spaceGUIDArg).To(Equal("some-space-guid"))

					Expect(fakeActor.ScaleProcessByApplicationCallCount()).To(Equal(1))
					appGUIDArg, scaleProcess := fakeActor.ScaleProcessByApplicationArgsForCall(0)
					Expect(appGUIDArg).To(Equal("some-app-guid"))
					Expect(scaleProcess).To(Equal(v3action.Process{
						Type:      "web",
						Instances: types.NullInt{Value: 3, IsSet: true},
					}))

					Expect(fakeActor.StopApplicationCallCount()).To(Equal(0))
					Expect(fakeActor.StartApplicationCallCount()).To(Equal(0))

					Expect(fakeActor.GetProcessByApplicationAndProcessTypeCallCount()).To(Equal(1))
					appGUID, processType := fakeActor.GetProcessByApplicationAndProcessTypeArgsForCall(0)
					Expect(appGUID).To(Equal("some-app-guid"))
					Expect(processType).To(Equal("web"))
				})
			})

			Context("when only the memory flag option is provided", func() {
				BeforeEach(func() {
					cmd.MemoryLimit.Value = 256
					cmd.MemoryLimit.IsSet = true
					fakeActor.ScaleProcessByApplicationReturns(
						v3action.Warnings{"scale-warning"},
						nil)
					fakeActor.GetProcessByApplicationAndProcessTypeReturns(
						process,
						v3action.Warnings{"get-instances-warning"},
						nil)

					_, err := input.Write([]byte("y\n"))
					Expect(err).ToNot(HaveOccurred())
				})

				It("scales, restarts, and displays scale properties", func() {
					Expect(executeErr).ToNot(HaveOccurred())

					Expect(testUI.Out).ToNot(Say("Showing"))
					Expect(testUI.Out).To(Say("Scaling"))
					Expect(testUI.Out).To(Say("This will cause the app to restart"))
					Expect(testUI.Out).To(Say("Stopping"))
					Expect(testUI.Out).To(Say("Starting"))

					Expect(testUI.Err).To(Say("get-app-warning"))
					Expect(testUI.Err).To(Say("scale-warning"))
					Expect(testUI.Err).To(Say("get-instances-warning"))

					Expect(fakeActor.GetApplicationByNameAndSpaceCallCount()).To(Equal(1))
					appNameArg, spaceGUIDArg := fakeActor.GetApplicationByNameAndSpaceArgsForCall(0)
					Expect(appNameArg).To(Equal(appName))
					Expect(spaceGUIDArg).To(Equal("some-space-guid"))

					Expect(fakeActor.ScaleProcessByApplicationCallCount()).To(Equal(1))
					appGUIDArg, scaleProcess := fakeActor.ScaleProcessByApplicationArgsForCall(0)
					Expect(appGUIDArg).To(Equal("some-app-guid"))
					Expect(scaleProcess).To(Equal(v3action.Process{
						Type:       "web",
						MemoryInMB: types.NullUint64{Value: 256, IsSet: true},
					}))

					Expect(fakeActor.StopApplicationCallCount()).To(Equal(1))
					appGUID := fakeActor.StopApplicationArgsForCall(0)
					Expect(appGUID).To(Equal("some-app-guid"))

					Expect(fakeActor.StartApplicationCallCount()).To(Equal(1))
					appGUID = fakeActor.StartApplicationArgsForCall(0)
					Expect(appGUID).To(Equal("some-app-guid"))

					Expect(fakeActor.GetProcessByApplicationAndProcessTypeCallCount()).To(Equal(1))
					appGUID, processType := fakeActor.GetProcessByApplicationAndProcessTypeArgsForCall(0)
					Expect(appGUID).To(Equal("some-app-guid"))
					Expect(processType).To(Equal("web"))
				})
			})

			Context("when only the disk flag option is provided", func() {
				BeforeEach(func() {
					cmd.DiskLimit.Value = 1025
					cmd.DiskLimit.IsSet = true
					fakeActor.ScaleProcessByApplicationReturns(
						v3action.Warnings{"scale-warning"},
						nil)
					fakeActor.GetProcessByApplicationAndProcessTypeReturns(
						process,
						v3action.Warnings{"get-instances-warning"},
						nil)
					_, err := input.Write([]byte("y\n"))
					Expect(err).ToNot(HaveOccurred())
				})

				It("scales the number of instances, displays scale properties, and restarts the application", func() {
					Expect(executeErr).ToNot(HaveOccurred())

					Expect(testUI.Out).ToNot(Say("Showing"))
					Expect(testUI.Out).To(Say("Scaling"))
					Expect(testUI.Out).To(Say("This will cause the app to restart"))
					Expect(testUI.Out).To(Say("Stopping"))
					Expect(testUI.Out).To(Say("Starting"))

					Expect(testUI.Err).To(Say("get-app-warning"))
					Expect(testUI.Err).To(Say("scale-warning"))
					Expect(testUI.Err).To(Say("get-instances-warning"))

					Expect(fakeActor.GetApplicationByNameAndSpaceCallCount()).To(Equal(1))
					appNameArg, spaceGUIDArg := fakeActor.GetApplicationByNameAndSpaceArgsForCall(0)
					Expect(appNameArg).To(Equal(appName))
					Expect(spaceGUIDArg).To(Equal("some-space-guid"))

					Expect(fakeActor.ScaleProcessByApplicationCallCount()).To(Equal(1))
					appGUIDArg, scaleProcess := fakeActor.ScaleProcessByApplicationArgsForCall(0)
					Expect(appGUIDArg).To(Equal("some-app-guid"))
					Expect(scaleProcess).To(Equal(v3action.Process{
						Type:     "web",
						DiskInMB: types.NullUint64{Value: 1025, IsSet: true},
					}))

					Expect(fakeActor.StopApplicationCallCount()).To(Equal(1))
					appGUID := fakeActor.StopApplicationArgsForCall(0)
					Expect(appGUID).To(Equal("some-app-guid"))

					Expect(fakeActor.StartApplicationCallCount()).To(Equal(1))
					appGUID = fakeActor.StartApplicationArgsForCall(0)
					Expect(appGUID).To(Equal("some-app-guid"))

					Expect(fakeActor.GetProcessByApplicationAndProcessTypeCallCount()).To(Equal(1))
					appGUID, processType := fakeActor.GetProcessByApplicationAndProcessTypeArgsForCall(0)
					Expect(appGUID).To(Equal("some-app-guid"))
					Expect(processType).To(Equal("web"))
				})
			})

			Context("when process flag is provided", func() {
				BeforeEach(func() {
					cmd.ProcessType = "some-process-type"
					cmd.Instances.Value = 2
					cmd.Instances.IsSet = true
					fakeActor.ScaleProcessByApplicationReturns(
						v3action.Warnings{"scale-warning"},
						nil)
					fakeActor.GetProcessByApplicationAndProcessTypeReturns(
						process,
						v3action.Warnings{"get-instances-warning"},
						nil)
					_, err := input.Write([]byte("y\n"))
					Expect(err).ToNot(HaveOccurred())
				})

				It("scales the specified process", func() {
					Expect(executeErr).ToNot(HaveOccurred())

					Expect(testUI.Out).ToNot(Say("Showing"))
					Expect(testUI.Out).To(Say("Scaling"))

					Expect(testUI.Err).To(Say("get-app-warning"))
					Expect(testUI.Err).To(Say("scale-warning"))
					Expect(testUI.Err).To(Say("get-instances-warning"))

					Expect(fakeActor.GetApplicationByNameAndSpaceCallCount()).To(Equal(1))
					appNameArg, spaceGUIDArg := fakeActor.GetApplicationByNameAndSpaceArgsForCall(0)
					Expect(appNameArg).To(Equal(appName))
					Expect(spaceGUIDArg).To(Equal("some-space-guid"))

					Expect(fakeActor.ScaleProcessByApplicationCallCount()).To(Equal(1))
					appGUIDArg, scaleProcess := fakeActor.ScaleProcessByApplicationArgsForCall(0)
					Expect(appGUIDArg).To(Equal("some-app-guid"))
					Expect(scaleProcess).To(Equal(v3action.Process{
						Type:      "some-process-type",
						Instances: types.NullInt{Value: 2, IsSet: true},
					}))

					Expect(fakeActor.GetProcessByApplicationAndProcessTypeCallCount()).To(Equal(1))
					appGUID, processType := fakeActor.GetProcessByApplicationAndProcessTypeArgsForCall(0)
					Expect(appGUID).To(Equal("some-app-guid"))
					Expect(processType).To(Equal("some-process-type"))
				})
			})

			Context("when an error is encountered scaling the application", func() {
				var expectedErr error

				BeforeEach(func() {
					cmd.Instances.Value = 3
					cmd.Instances.IsSet = true
					expectedErr = errors.New("scale process error")
					fakeActor.ScaleProcessByApplicationReturns(
						v3action.Warnings{"scale-process-warning"},
						expectedErr,
					)
				})

				It("returns the error and displays all warnings", func() {
					Expect(executeErr).To(Equal(expectedErr))
					Expect(testUI.Err).To(Say("scale-process-warning"))
				})
			})
		})
	})
})
