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
