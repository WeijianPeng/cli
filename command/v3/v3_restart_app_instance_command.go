package v3

import (
	"code.cloudfoundry.org/cli/actor/sharedaction"
	"code.cloudfoundry.org/cli/actor/v3action"
	"code.cloudfoundry.org/cli/command"
	"code.cloudfoundry.org/cli/command/flag"
	"code.cloudfoundry.org/cli/command/v3/shared"
	"code.cloudfoundry.org/cli/version"
)

//go:generate counterfeiter . V3RestartAppInstanceActor

type V3RestartAppInstanceActor interface {
	CloudControllerAPIVersion() string
	DeleteInstanceByApplicationNameSpaceProcessTypeAndIndex(appName string, spaceGUID string, processType string, instanceIndex int) (v3action.Warnings, error)
}

type V3RestartAppInstanceCommand struct {
	RequiredArgs    flag.AppInstance `positional-args:"yes"`
	ProcessType     string           `long:"process" default:"web" description:"Process to restart"`
	usage           interface{}      `usage:"CF_NAME v3-restart-app-instance APP_NAME INDEX [--process PROCESS]"`
	relatedCommands interface{}      `related_commands:"v3-restart"`

	UI          command.UI
	Config      command.Config
	SharedActor command.SharedActor
	Actor       V3RestartAppInstanceActor
}

func (cmd *V3RestartAppInstanceCommand) Setup(config command.Config, ui command.UI) error {
	cmd.UI = ui
	cmd.Config = config
	cmd.SharedActor = sharedaction.NewActor()

	ccClient, _, err := shared.NewClients(config, ui, true)
	if err != nil {
		return err
	}
	cmd.Actor = v3action.NewActor(ccClient, config)

	return nil
}

func (cmd V3RestartAppInstanceCommand) Execute(args []string) error {
	err := version.MinimumAPIVersionCheck(cmd.Actor.CloudControllerAPIVersion(), version.MinVersionV3)
	if err != nil {
		return err
	}

	err = cmd.SharedActor.CheckTarget(cmd.Config, true, true)
	if err != nil {
		return shared.HandleError(err)
	}

	user, err := cmd.Config.CurrentUser()
	if err != nil {
		return shared.HandleError(err)
	}

	cmd.UI.DisplayTextWithFlavor("Restarting instance {{.InstanceIndex}} of process {{.ProcessType}} of app {{.AppName}} in org {{.OrgName}} / space {{.SpaceName}} as {{.Username}}...", map[string]interface{}{
		"InstanceIndex": cmd.RequiredArgs.Index,
		"ProcessType":   cmd.ProcessType,
		"AppName":       cmd.RequiredArgs.AppName,
		"Username":      user.Name,
		"OrgName":       cmd.Config.TargetedOrganization().Name,
		"SpaceName":     cmd.Config.TargetedSpace().Name,
	})

	warnings, err := cmd.Actor.DeleteInstanceByApplicationNameSpaceProcessTypeAndIndex(cmd.RequiredArgs.AppName, cmd.Config.TargetedSpace().GUID, cmd.ProcessType, cmd.RequiredArgs.Index)
	cmd.UI.DisplayWarnings(warnings)
	if err != nil {
		return shared.HandleError(err)
	}

	cmd.UI.DisplayOK()
	return nil
}
