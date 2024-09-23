package cli

import (
	"fmt"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/utils"
	"github.com/urfave/cli/v2"
)

var CmdAdmin = cli.Command{
	Name:  "admin",
	Usage: "Admin commands",
	Subcommands: []*cli.Command{
		&CmdAdminResetPassword,
		&CmdAdminToggleAdmin,
	},
}

var CmdAdminResetPassword = cli.Command{
	Name:      "reset-password",
	Usage:     "Reset the password for a given user",
	ArgsUsage: "[username] [password]",
	Action: func(ctx *cli.Context) error {
		initialize(ctx)
		if ctx.NArg() < 2 {
			return fmt.Errorf("username and password are required")
		}
		username := ctx.Args().Get(0)
		plainPassword := ctx.Args().Get(1)

		user, err := db.GetUserByUsername(username)
		if err != nil {
			fmt.Printf("Cannot get user %s: %s\n", username, err)
			return err
		}
		password, err := utils.Argon2id.Hash(plainPassword)
		if err != nil {
			fmt.Printf("Cannot hash password for user %s: %s\n", username, err)
			return err
		}
		user.Password = password

		if err = user.Update(); err != nil {
			fmt.Printf("Cannot update password for user %s: %s\n", username, err)
			return err
		}

		fmt.Printf("Password for user %s has been reset.\n", username)
		return nil
	},
}

var CmdAdminToggleAdmin = cli.Command{
	Name:      "toggle-admin",
	Usage:     "Toggle the admin status for a given user",
	ArgsUsage: "[username]",
	Action: func(ctx *cli.Context) error {
		initialize(ctx)
		if ctx.NArg() < 1 {
			return fmt.Errorf("username is required")
		}
		username := ctx.Args().Get(0)

		user, err := db.GetUserByUsername(username)
		if err != nil {
			fmt.Printf("Cannot get user %s: %s\n", username, err)
			return err
		}

		user.IsAdmin = !user.IsAdmin
		if err = user.Update(); err != nil {
			fmt.Printf("Cannot update user %s: %s\n", username, err)
		}

		fmt.Printf("User %s admin set to %t\n", username, user.IsAdmin)
		return nil
	},
}
