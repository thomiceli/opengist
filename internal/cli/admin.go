package cli

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/auth/password"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/validator"
	"github.com/urfave/cli/v2"
)

// cliError prints a message to stderr and returns it as an error, so that
// user-facing validation/guidance failures are visible even though main() only
// acts on the presence of an error (os.Exit(1)) without printing it.
func cliError(format string, a ...any) error {
	err := fmt.Errorf(format, a...)
	fmt.Fprintln(os.Stderr, "Error:", err)
	return err
}

func initialize(ctx *cli.Context) {
	if err := config.InitConfig(ctx.String("config"), io.Discard); err != nil {
		panic(err)
	}
	config.InitLog()

	db.DeprecationDBFilename()
	if err := db.Setup(config.C.DBUri); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
}

var CmdAdmin = cli.Command{
	Name:  "admin",
	Usage: "Admin commands",
	Subcommands: []*cli.Command{
		&CmdAdminCreateUser,
		&CmdAdminResetPassword,
		&CmdAdminToggleAdmin,
	},
}

// CmdAdminCreateUser creates a user from the CLI. Its primary purpose is to
// seed an account (typically an administrator) at install time, so that
// deployment can be fully automated with tools like Ansible, without having
// to interact with the web setup form.
//
// The command is idempotent: if the user already exists it does nothing and
// exits successfully, making it safe to re-run from a provisioning playbook.
// It only ever creates users — promoting an existing user or changing its
// password is the job of toggle-admin / reset-password.
//
// All inputs are flags (including --username). Unlike the sibling
// reset-password/toggle-admin commands which take a positional username,
// create-user avoids a positional argument on purpose: urfave/cli's
// default-command resolution would otherwise misroute the invocation when a
// positional value happens to match a flag name (e.g. a user named "admin"
// combined with --admin).
var CmdAdminCreateUser = cli.Command{
	Name:      "create-user",
	Usage:     "Create a new user (useful to seed an admin account at install time)",
	ArgsUsage: "[command options]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "username",
			Aliases: []string{"u"},
			Usage:   "Username of the new user",
		},
		&cli.StringFlag{
			Name:    "password",
			Aliases: []string{"p"},
			Usage:   "Password for the new user (exposed in the process list; prefer --password-stdin for automation)",
		},
		&cli.BoolFlag{
			Name:  "password-stdin",
			Usage: "Read the password from stdin",
		},
		&cli.StringFlag{
			Name:  "email",
			Usage: "Email address of the new user",
		},
		&cli.BoolFlag{
			Name:  "admin",
			Usage: "Grant administrator privileges to the new user",
		},
	},
	Action: func(ctx *cli.Context) error {
		initialize(ctx)

		username := ctx.String("username")
		if username == "" {
			if ctx.NArg() > 0 {
				return cliError("create-user takes flags, not a positional argument; pass the username via --username (e.g. opengist admin create-user --username %s ...)", ctx.Args().Get(0))
			}
			return cliError("username is required (use --username)")
		}

		// Validate the username using the same rules as the web registration
		// form (alphanumerics and dashes, max 24 chars, no reserved names).
		v := validator.NewValidator()
		if err := v.Var(username, "required,max=24,alphanumdash,notreserved"); err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid username: %s\n", err)
			return err
		}

		plainPassword, err := resolveCreateUserPassword(ctx)
		if err != nil {
			return cliError("%s", err)
		}
		if plainPassword == "" {
			return cliError("password is required (use --password or --password-stdin)")
		}

		// Idempotent: provisioning tools may re-run this command, so a no-op on
		// an existing user is preferred over an error.
		exists, err := db.UserExists(username)
		if err != nil {
			fmt.Printf("Cannot check if user %s exists: %s\n", username, err)
			return err
		}
		if exists {
			fmt.Printf("User %s already exists; nothing to do.\n", username)
			return nil
		}

		email := strings.ToLower(strings.TrimSpace(ctx.String("email")))

		hashedPassword, err := password.HashPassword(plainPassword)
		if err != nil {
			fmt.Printf("Cannot hash password for user %s: %s\n", username, err)
			return err
		}

		user := &db.User{
			Username: username,
			Password: hashedPassword,
			Email:    email,
			MD5Hash:  gravatarHash(email),
			IsAdmin:  ctx.Bool("admin"),
		}

		if err = user.Create(); err != nil {
			fmt.Printf("Cannot create user %s: %s\n", username, err)
			return err
		}

		fmt.Printf("User %s has been created", username)
		if user.IsAdmin {
			fmt.Print(" with administrator privileges")
		}
		fmt.Println(".")
		return nil
	},
}

// resolveCreateUserPassword returns the password from the --password flag, or
// reads it from stdin when --password-stdin is set. The two sources are
// mutually exclusive. Reading from stdin keeps the secret out of the process
// list and shell history, which is preferred for automated provisioning.
func resolveCreateUserPassword(ctx *cli.Context) (string, error) {
	if ctx.Bool("password-stdin") {
		if ctx.String("password") != "" {
			return "", fmt.Errorf("--password and --password-stdin are mutually exclusive")
		}
		reader := bufio.NewReader(os.Stdin)
		bytes, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("cannot read password from stdin: %w", err)
		}
		return strings.TrimRight(bytes, "\r\n"), nil
	}
	return ctx.String("password"), nil
}

// gravatarHash computes the gravatar key the same way the web flows do, so an
// avatar resolves identically regardless of how the account was created. With
// no email the hash is left empty, matching the password-registration flow.
func gravatarHash(email string) string {
	if email == "" {
		return ""
	}
	return fmt.Sprintf("%x", md5.Sum([]byte(email)))
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
		password, err := password.HashPassword(plainPassword)
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
