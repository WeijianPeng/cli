package v3

import (
	"code.cloudfoundry.org/cli/actor/sharedaction"
	"code.cloudfoundry.org/cli/actor/v3action"
	"code.cloudfoundry.org/cli/command"
	"code.cloudfoundry.org/cli/command/flag"
	"code.cloudfoundry.org/cli/command/v3/shared"
	"code.cloudfoundry.org/cli/version"
)

//go:generate counterfeiter . V3CreateAppActor

type V3CreateAppActor interface {
	CloudControllerAPIVersion() string
	CreateApplicationInSpace(app v3action.Application, spaceGUID string) (v3action.Application, v3action.Warnings, error)
}

type V3CreateAppCommand struct {
	RequiredArgs flag.AppName `positional-args:"yes"`
	usage        interface{}  `usage:"CF_NAME v3-create-app APP_NAME"`

	UI          command.UI
	Config      command.Config
	SharedActor command.SharedActor
	Actor       V3CreateAppActor
}

func (cmd *V3CreateAppCommand) Setup(config command.Config, ui command.UI) error {
	cmd.UI = ui
	cmd.Config = config
	cmd.SharedActor = sharedaction.NewActor()

	client, _, err := shared.NewClients(config, ui, true)
	if err != nil {
		return err
	}
	cmd.Actor = v3action.NewActor(client, config)

	return nil
}

func (cmd V3CreateAppCommand) Execute(args []string) error {
	cmd.UI.DisplayText(command.ExperimentalWarning)
	cmd.UI.DisplayNewline()

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
		return err
	}

	cmd.UI.DisplayTextWithFlavor("Creating V3 app {{.AppName}} in org {{.CurrentOrg}} / space {{.CurrentSpace}} as {{.CurrentUser}}...", map[string]interface{}{
		"AppName":      cmd.RequiredArgs.AppName,
		"CurrentSpace": cmd.Config.TargetedSpace().Name,
		"CurrentOrg":   cmd.Config.TargetedOrganization().Name,
		"CurrentUser":  user.Name,
	})

	_, warnings, err := cmd.Actor.CreateApplicationInSpace(
		v3action.Application{
			Name: cmd.RequiredArgs.AppName,
		},
		cmd.Config.TargetedSpace().GUID,
	)
	cmd.UI.DisplayWarnings(warnings)
	if err != nil {
		switch err.(type) {
		case v3action.ApplicationAlreadyExistsError:
			cmd.UI.DisplayWarning("App {{.AppName}} already exists", map[string]interface{}{
				"AppName": cmd.RequiredArgs.AppName,
			})
		default:
			return shared.HandleError(err)
		}
	}

	cmd.UI.DisplayOK()

	return nil
}
