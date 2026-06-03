package user

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	"github.com/bitrise-io/bitrise-cli/internal/output/style"
	internaluser "github.com/bitrise-io/bitrise-cli/internal/user"
	"github.com/bitrise-io/bitrise-cli/internal/webclient"
)

func newCreateCmd() *cobra.Command {
	var (
		email         string
		username      string
		firstName     string
		lastName      string
		passwordStdin bool
	)

	c := &cobra.Command{
		Use:   "create",
		Short: "Create a new Bitrise account",
		Long: `Create a new Bitrise account by email and password.

Required flags:
  --email ADDRESS    the email address to register
  --username NAME    desired username (must be unique)
  --first-name N     first name on the account
  --last-name N      last name on the account

Optional flags:
  --password-stdin   read the password from stdin instead of prompting

Password input:
  By default the command prompts for the password (input is masked when stdin
  is a terminal). Use --password-stdin to read it from stdin without a prompt
  — the right choice for piping or scripts:

      printf '%s' "$NEW_PASSWORD" | bitrise-cli user create \
          --email a@b.io --username alice --password-stdin

Email verification:
  After signup the server emails a verification link. Click it before running
  'bitrise-cli auth login --email <addr>' — sign-in is blocked on unverified
  accounts.`,
		Example: `  bitrise-cli user create --email alice@example.com --username alice --first-name Alice --last-name L
  printf '%s' "$NEW_PASSWORD" | bitrise-cli user create \
      --email alice@example.com --username alice --first-name Alice --last-name L --password-stdin --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if email == "" {
				return fmt.Errorf("--email is required")
			}
			if username == "" {
				return fmt.Errorf("--username is required")
			}
			if firstName == "" {
				return fmt.Errorf("--first-name is required")
			}
			if lastName == "" {
				return fmt.Errorf("--last-name is required")
			}
			pw, err := cmdutil.ReadSecretInput(cmd.InOrStdin(), cmd.ErrOrStderr(), "Choose a password: ", passwordStdin)
			if err != nil {
				return err
			}
			if pw == "" {
				return fmt.Errorf("password is empty")
			}

			webClient, err := webclient.New(cmdutil.ResolveWebBaseURL(cmd))
			if err != nil {
				return err
			}
			svc := internaluser.NewService(webClient)
			acct, err := svc.Signup(cmd.Context(), internaluser.SignupInput{
				Email:     email,
				Username:  username,
				Password:  pw,
				FirstName: firstName,
				LastName:  lastName,
			})
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), cmdutil.ResolveFormat(cmd), acct, renderCreateHuman)
		},
	}

	c.Flags().StringVar(&email, "email", "", "email address to register (required)")
	c.Flags().StringVar(&username, "username", "", "desired username (required)")
	c.Flags().StringVar(&firstName, "first-name", "", "first name on the account (required)")
	c.Flags().StringVar(&lastName, "last-name", "", "last name on the account (required)")
	c.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read the password from stdin without prompting")

	_ = c.MarkFlagRequired("email")
	_ = c.MarkFlagRequired("username")
	_ = c.MarkFlagRequired("first-name")
	_ = c.MarkFlagRequired("last-name")

	return c
}

func renderCreateHuman(w io.Writer, a internaluser.Account) error {
	s := style.New(w)
	ew := cmdutil.NewErrWriter(w)
	lbl := func(label string) string {
		return s.Label.Render(fmt.Sprintf("%-16s", label))
	}
	ew.F("%s %s\n", s.Success.Render("✓"), s.Bold.Render("Account created"))
	ew.F("%s%s\n", lbl("Email:"), a.Email)
	if a.Username != "" {
		ew.F("%s%s\n", lbl("Username:"), a.Username)
	}
	if a.Slug != "" {
		ew.F("%s%s\n", lbl("Slug:"), s.Slug.Render(a.Slug))
	}
	if !a.Confirmed {
		ew.Ln()
		ew.Ln("Check your email and click the verification link, then run:")
		ew.F("  bitrise-cli auth login --email %s\n", a.Email)
	}
	return ew.Err
}
