package warmpool

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdutil"
	"github.com/bitrise-io/bitrise-cli/cmd/rde/rdeflags"
	"github.com/bitrise-io/bitrise-cli/internal/output"
	internalrde "github.com/bitrise-io/bitrise-cli/internal/rde"
)

// inputFlags collects the session-input flags 'create' and 'update' share.
// They mirror the flags of 'session create' so a pool's inputs read the same
// as a session's.
type inputFlags struct {
	plain  []string
	secret []string
	saved  []string
}

func (f *inputFlags) bind(c *cobra.Command) {
	c.Flags().StringArrayVar(&f.plain, "input", nil, "session input as key=value (repeatable)")
	c.Flags().StringArrayVar(&f.secret, "secret-input", nil, "session input as key=value, stored as a secret at rest (repeatable; the value is visible in shell history and process args — prefer --saved-input on a user pool)")
	c.Flags().StringArrayVar(&f.saved, "saved-input", nil, "session input as key=savedInputID — uses one of your stored saved-input values (repeatable; user pools only)")
}

// set reports whether any input flag was given.
func (f *inputFlags) set() bool {
	return len(f.plain) > 0 || len(f.secret) > 0 || len(f.saved) > 0
}

func (f *inputFlags) parse() ([]internalrde.SessionInputValue, error) {
	return rdeflags.ParseSessionInputs(f.plain, f.secret, f.saved)
}

func newCreateCmd() *cobra.Command {
	var (
		template          string
		count             int
		owner             string
		inputs            inputFlags
		mapSavedInputs    bool
		featureFlags      []string
		stack             string
		machineType       string
		cluster           string
		devicePlatform    string
		deviceModel       string
		deviceOSVersion   string
		deviceSystemImage string
		noDevice          bool
	)
	c := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a warm pool",
		Long: `Create a warm pool: a stored session configuration the RDE backend keeps
--size sessions of booted and idle, ready to be claimed with 'rde session
create --warm-pool NAME'.

NAME is a human-readable label for the pool; you can use it in place of the
pool ID in later commands as long as it stays unique. --template names the
template (by ID or name) the warm sessions are created from; the other
configuration flags are those of 'rde session create' and are validated the
same way — a pool can only store what a session request could send.

--size is how many warm sessions to keep booted. It defaults to 0: the pool
is created as an inert configuration preset and costs nothing until you
raise the size with 'rde warm-pool set-size'. Every warm session costs
machine time while idle.

By default the pool is yours (--owner user): private, and it may reference
your saved inputs (--saved-input, --map-saved-inputs). Pass --owner
workspace to create a pool shared with every member of the workspace — the
only kind a Workspace API Token can create or claim from, and the only kind
that can back a preview link. A workspace pool carries no personal state:
give every template session input as a value (--input / --secret-input).

Provide session input values via --input (one --input per key),
--secret-input (stored as secret-at-rest), or --saved-input (a saved input
by ID; user pools only). A value passed inline with --secret-input ends up in
your shell history and process arguments; for a user pool, prefer storing it
once with 'rde saved-input create --value-stdin --secret' and referencing it.

--stack, --machine-type and --cluster override the template's defaults for
the warm sessions; omit them to use the template's.

The device the warm sessions boot follows 'rde session create': omit the
device flags to boot the template's declared device as is; --device-model,
--device-os-version and --device-system-image without --device-platform tweak
that device per field; with --device-platform the flags are the complete
device to boot; --no-device boots none. A claimed session's app artifact is
still given at claim time.`,
		Example: `  bitrise-cli rde warm-pool create ios-devs --template TEMPLATE_ID --size 2
  # Shared with the workspace; inputs as plain values (no saved inputs).
  bitrise-cli rde warm-pool create ci-devices --template TEMPLATE_ID --owner workspace --size 3 --secret-input GITHUB_TOKEN=ghp_xxx
  # A private pool that reuses a saved input and a feature flag.
  bitrise-cli rde warm-pool create mine --template TEMPLATE_ID --saved-input gh-token=SAVED_INPUT_ID --feature-flag enable_beta_simulator
  # A configuration preset: nothing booted until 'set-size' raises the count.
  bitrise-cli rde warm-pool create preset --template TEMPLATE_ID --machine-type g2.mac.m2pro.6c-14g
  # The template's simulator, but an iPhone 15 on iOS 17.5; or no device at all.
  bitrise-cli rde warm-pool create ios-17 --template TEMPLATE_ID --device-model "iPhone 15" --device-os-version 17.5
  bitrise-cli rde warm-pool create headless --template TEMPLATE_ID --no-device`,
		Args: cmdutil.RequireArgs("NAME"),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if name == "" {
				return fmt.Errorf("NAME must not be empty")
			}
			if template == "" {
				return fmt.Errorf("--template is required")
			}
			if count < 0 {
				return fmt.Errorf("--size must not be negative")
			}
			switch owner {
			case "", internalrde.SessionOwnerUser, internalrde.SessionOwnerWorkspace:
			default:
				return fmt.Errorf("--owner must be %s or %s", internalrde.SessionOwnerUser, internalrde.SessionOwnerWorkspace)
			}
			// Fail fast on what the backend is certain to reject: saved
			// inputs are personal and have no place on a shared pool.
			if owner == internalrde.SessionOwnerWorkspace && (len(inputs.saved) > 0 || mapSavedInputs) {
				return fmt.Errorf("--owner workspace: saved inputs are personal and cannot be used on a workspace pool — pass the values with --input or --secret-input instead of --saved-input / --map-saved-inputs")
			}
			sessionInputs, err := inputs.parse()
			if err != nil {
				return err
			}
			deviceSpec, err := deviceSpecFromFlags(devicePlatform, deviceModel, deviceOSVersion, deviceSystemImage)
			if err != nil {
				return err
			}
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			svc := internalrde.NewService(client)
			templateID, err := svc.ResolveTemplateID(cmd.Context(), workspaceID, template)
			if err != nil {
				return err
			}
			p, err := svc.CreateWarmPool(cmd.Context(), workspaceID, internalrde.CreateWarmPoolRequest{
				Name:                    name,
				TemplateID:              templateID,
				OwnerType:               owner,
				PoolSize:                count,
				SessionInputs:           sessionInputs,
				MapSavedToSessionInputs: mapSavedInputs,
				EnabledFeatureFlagNames: featureFlags,
				StackID:                 stack,
				MachineType:             machineType,
				Cluster:                 cluster,
				DeviceSpec:              deviceSpec,
				NoDevice:                noDevice,
			})
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, p, renderDetail)
		},
	}
	c.Flags().StringVar(&template, "template", "", "template ID or name the warm sessions are created from (required)")
	c.Flags().IntVar(&count, "size", 0, "how many warm sessions to keep booted; 0 (the default) creates an inert configuration preset")
	c.Flags().StringVar(&owner, "owner", "", "who owns the pool: user (default; private to you) or workspace (shared with every member; inputs as plain values only). A Workspace API Token always creates workspace pools")
	_ = c.RegisterFlagCompletionFunc("owner", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{internalrde.SessionOwnerUser, internalrde.SessionOwnerWorkspace}, cobra.ShellCompDirectiveNoFileComp
	})
	inputs.bind(c)
	c.Flags().BoolVar(&mapSavedInputs, "map-saved-inputs", false, "auto-fill template session inputs from your saved inputs, matched by key (user pools only; stored as saved-input references)")
	c.Flags().StringArrayVar(&featureFlags, "feature-flag", nil, "name of a template feature flag to enable on the warm sessions (repeatable)")
	c.Flags().StringVar(&stack, "stack", "", "stack ID to override the template's (see 'rde stack list')")
	c.Flags().StringVar(&machineType, "machine-type", "", "machine type name to override the template's (see 'rde machine-type list --stack STACK_ID')")
	c.Flags().StringVar(&cluster, "cluster", "", "target cluster name to override the one resolved from stack + machine type")
	bindDeviceFlags(c, &devicePlatform, &deviceModel, &deviceOSVersion, &deviceSystemImage)
	c.Flags().BoolVar(&noDevice, "no-device", false, "boot the warm sessions without the template's device (ignored when the template declares none)")
	for _, f := range deviceFlagNames {
		c.MarkFlagsMutuallyExclusive("no-device", f)
	}
	return c
}

// deviceFlagNames are the flags that describe a device; --no-device excludes
// them all.
var deviceFlagNames = []string{"device-platform", "device-model", "device-os-version", "device-system-image"}

// bindDeviceFlags adds the device flags 'rde session create' has, with the
// pool's wording.
func bindDeviceFlags(c *cobra.Command, platform, model, osVersion, systemImage *string) {
	c.Flags().StringVar(platform, "device-platform", "", "device the warm sessions boot: ios (simulator, macOS stack) or android (emulator, Linux stack); omit with --template to tweak the template's device per field")
	c.Flags().StringVar(model, "device-model", "", "device to boot: simctl device type (\"iPhone 16\") or emulator device profile (\"pixel_7\") — a screen profile, not that phone's firmware; default: the template's, else the platform default")
	c.Flags().StringVar(osVersion, "device-os-version", "", "iOS only: an iOS version (\"18.2\") or simctl runtime id — anything else is rejected; default: the template's, else newest installed")
	c.Flags().StringVar(systemImage, "device-system-image", "", "Android only: the API-level knob — sdkmanager system image package (\"system-images;android-34;google_apis;x86_64\"); default: the template's, else the platform default")
}

// deviceSpecFromFlags turns the device flags into a request spec, or nil when
// none was given. An empty platform is deliberate: the backend merges the spec
// over the template's declared device and fills the platform (and any other
// unset field) from it — the same contract as 'rde session create --template'.
func deviceSpecFromFlags(platform, model, osVersion, systemImage string) (*internalrde.DeviceSpec, error) {
	switch platform {
	case "", "ios", "android":
	default:
		return nil, fmt.Errorf("--device-platform must be ios or android")
	}
	if platform == "ios" && systemImage != "" {
		return nil, fmt.Errorf("--device-system-image applies to Android only")
	}
	if platform == "android" && osVersion != "" {
		return nil, fmt.Errorf("--device-os-version applies to iOS only")
	}
	if platform == "" && model == "" && osVersion == "" && systemImage == "" {
		return nil, nil
	}
	return &internalrde.DeviceSpec{Platform: platform, DeviceModel: model, OSVersion: osVersion, SystemImage: systemImage}, nil
}

func newUpdateCmd() *cobra.Command {
	var (
		name              string
		count             int
		inputs            inputFlags
		clearInputs       bool
		featureFlags      []string
		clearFeatureFlags bool
		stack             string
		machineType       string
		cluster           string
		devicePlatform    string
		deviceModel       string
		deviceOSVersion   string
		deviceSystemImage string
		clearDevice       bool
		noDevice          bool
	)
	c := &cobra.Command{
		Use:   "update WARM_POOL_ID",
		Short: "Update a warm pool's name, count or configuration",
		Long: `Update a warm pool. Only the flags you pass are sent; everything else is
left as it is.

Session inputs and feature flags are replaced as a whole: passing any
--input / --secret-input / --saved-input flag replaces the pool's entire
input list with the ones given, and any --feature-flag replaces the enabled
flags. Use --clear-inputs / --clear-feature-flags to empty a list.
--stack, --machine-type and --cluster set an override; pass an empty value
("") to remove one and fall back to the template's. The device flags set the
pool's device override (without --device-platform they tweak the template's
declared device per field; with it they are the complete device);
--clear-device removes the override so the template's device applies as
declared; --no-device / --no-device=false skip or restore the template's
device.

Changing the configuration invalidates the pool's current inventory: the
backend terminates the existing warm sessions and boots new ones from the
updated configuration. Changing only --name or --size does not. For the
count alone, 'rde warm-pool set-size' is the shorter spelling.`,
		Example: `  bitrise-cli rde warm-pool update WARM_POOL_ID --name ios-devs-eu --size 4
  # Replace the inputs (all of them) and switch to a bigger machine.
  bitrise-cli rde warm-pool update ios-devs --secret-input GITHUB_TOKEN=ghp_yyy --machine-type g2.mac.m2pro.12c-32g
  # Drop the stack override so the template's stack applies again.
  bitrise-cli rde warm-pool update ios-devs --stack ""
  bitrise-cli rde warm-pool update ios-devs --clear-feature-flags
  # Boot an iPhone 15 instead of the template's device; then go back to the template's.
  bitrise-cli rde warm-pool update ios-devs --device-model "iPhone 15"
  bitrise-cli rde warm-pool update ios-devs --clear-device`,
		Args: cmdutil.RequireArgs("WARM_POOL_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputs.set() && clearInputs {
				return fmt.Errorf("--clear-inputs cannot be combined with --input, --secret-input or --saved-input")
			}
			if len(featureFlags) > 0 && clearFeatureFlags {
				return fmt.Errorf("--clear-feature-flags cannot be combined with --feature-flag")
			}
			if count < 0 {
				return fmt.Errorf("--size must not be negative")
			}
			flags := cmd.Flags()
			req := internalrde.UpdateWarmPoolRequest{}
			changed := false
			if flags.Changed("name") {
				if name == "" {
					return fmt.Errorf("--name must not be empty")
				}
				req.Name = &name
				changed = true
			}
			if flags.Changed("size") {
				n := count
				req.PoolSize = &n
				changed = true
			}
			if inputs.set() || clearInputs {
				list, err := inputs.parse()
				if err != nil {
					return err
				}
				req.SessionInputs = &list
				changed = true
			}
			if len(featureFlags) > 0 || clearFeatureFlags {
				list := featureFlags
				if list == nil {
					list = []string{}
				}
				req.EnabledFeatureFlagNames = &list
				changed = true
			}
			if flags.Changed("stack") {
				req.StackID = &stack
				changed = true
			}
			if flags.Changed("machine-type") {
				req.MachineType = &machineType
				changed = true
			}
			if flags.Changed("cluster") {
				req.Cluster = &cluster
				changed = true
			}
			deviceSpec, err := deviceSpecFromFlags(devicePlatform, deviceModel, deviceOSVersion, deviceSystemImage)
			if err != nil {
				return err
			}
			if deviceSpec != nil && clearDevice {
				return fmt.Errorf("--clear-device cannot be combined with the --device-* flags")
			}
			if deviceSpec != nil {
				req.DeviceSpec = deviceSpec
				changed = true
			}
			if clearDevice {
				// The update switch alone: the backend drops the pool's
				// device override.
				req.ClearDeviceSpec = true
				changed = true
			}
			if flags.Changed("no-device") {
				v := noDevice
				req.NoDevice = &v
				changed = true
			}
			if !changed {
				return fmt.Errorf("nothing to update: pass at least one of --name, --size, --input, --secret-input, --saved-input, --clear-inputs, --feature-flag, --clear-feature-flags, --stack, --machine-type, --cluster, the --device-* flags, --clear-device or --no-device")
			}
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			svc := internalrde.NewService(client)
			warmPoolID, err := svc.ResolveWarmPoolID(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			p, err := svc.UpdateWarmPool(cmd.Context(), workspaceID, warmPoolID, req)
			if err != nil {
				return err
			}
			return output.Render(cmd.OutOrStdout(), format, p, renderDetail)
		},
	}
	c.Flags().StringVar(&name, "name", "", "new name")
	c.Flags().IntVar(&count, "size", 0, "new pool size of warm sessions; 0 drains the pool but keeps it as a preset")
	inputs.bind(c)
	c.Flags().BoolVar(&clearInputs, "clear-inputs", false, "remove every stored session input")
	c.Flags().StringArrayVar(&featureFlags, "feature-flag", nil, "feature flag to enable on the warm sessions (repeatable; replaces the enabled flags as a whole)")
	c.Flags().BoolVar(&clearFeatureFlags, "clear-feature-flags", false, "disable every feature flag")
	c.Flags().StringVar(&stack, "stack", "", "stack ID override; \"\" removes the override")
	c.Flags().StringVar(&machineType, "machine-type", "", "machine type override; \"\" removes the override")
	c.Flags().StringVar(&cluster, "cluster", "", "cluster override; \"\" removes the override")
	bindDeviceFlags(c, &devicePlatform, &deviceModel, &deviceOSVersion, &deviceSystemImage)
	c.Flags().BoolVar(&clearDevice, "clear-device", false, "remove the pool's device override; the template's device applies as declared")
	c.Flags().BoolVar(&noDevice, "no-device", false, "true: the warm sessions boot without the template's device; --no-device=false boots it again")
	for _, f := range deviceFlagNames {
		c.MarkFlagsMutuallyExclusive("no-device", f)
	}
	return c
}

func newSetSizeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-size WARM_POOL_ID SIZE",
		Short: "Set a pool's size: how many warm sessions it keeps booted",
		Long: `Set a warm pool's pool size — how many sessions of its configuration
the RDE backend keeps booted and idle. Raising it boots sessions; lowering it
terminates surplus warm sessions (claimed sessions are untouched). 0 drains
the pool but keeps it usable as a configuration preset: 'rde session create
--warm-pool' then creates sessions on demand from its configuration.

This is the knob for scaling a pool to business hours: run it from a cron job
in the morning and again with 0 in the evening. A Workspace API Token works
for workspace pools, so the job does not need a personal token.`,
		Example: `  bitrise-cli rde warm-pool set-size ios-devs 3
  # Cron: warm up at 08:00 on weekdays, drain at 19:00.
  0 8  * * 1-5  BITRISE_TOKEN=bitwat_… bitrise-cli rde warm-pool set-size WARM_POOL_ID 3 --workspace WORKSPACE_ID -q
  0 19 * * 1-5  BITRISE_TOKEN=bitwat_… bitrise-cli rde warm-pool set-size WARM_POOL_ID 0 --workspace WORKSPACE_ID -q`,
		Args: cmdutil.RequireArgs("WARM_POOL_ID", "COUNT"),
		RunE: func(cmd *cobra.Command, args []string) error {
			count, err := strconv.Atoi(args[1])
			if err != nil || count < 0 {
				return fmt.Errorf("COUNT must be a non-negative integer, got %q", args[1])
			}
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			format := cmdutil.ResolveFormat(cmd)
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			svc := internalrde.NewService(client)
			warmPoolID, err := svc.ResolveWarmPoolID(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			p, err := svc.UpdateWarmPool(cmd.Context(), workspaceID, warmPoolID, internalrde.UpdateWarmPoolRequest{PoolSize: &count})
			if err != nil {
				return err
			}
			if format == output.JSON {
				return output.Render(cmd.OutOrStdout(), format, p, renderDetail)
			}
			if !cmdutil.IsQuiet(cmd) {
				_, err := fmt.Fprintf(cmd.ErrOrStderr(), "Warm pool %s now keeps %d warm session(s)\n", p.Name, p.PoolSize)
				return err
			}
			return nil
		},
	}
}

func newDeleteCmd() *cobra.Command {
	var assumeYes bool
	c := &cobra.Command{
		Use:   "delete WARM_POOL_ID",
		Short: "Delete a warm pool and terminate its warm sessions",
		Long: `Delete a warm pool. Its warm sessions — the booted, unclaimed ones — are
terminated and removed by the backend; sessions already claimed from the
pool are ordinary sessions and are untouched. Preview links minted with this
pool stop working: every later open fails as an expired link would.

This cannot be undone. Pass --yes to skip the confirmation prompt. To stop
paying for idle machines while keeping the configuration and its links,
set the pool size to 0 instead ('rde warm-pool set-size POOL 0').`,
		Example: `  bitrise-cli rde warm-pool delete WARM_POOL_ID
  bitrise-cli rde warm-pool delete ios-devs --yes`,
		Args: cmdutil.RequireArgs("WARM_POOL_ID"),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceID, err := cmdutil.ResolveWorkspaceID(cmd)
			if err != nil {
				return err
			}
			client, err := cmdutil.NewRDEClient(cmd)
			if err != nil {
				return err
			}
			svc := internalrde.NewService(client)
			warmPoolID, err := svc.ResolveWarmPoolID(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return err
			}
			if !assumeYes {
				prompt := fmt.Sprintf("This will delete warm pool %s and terminate its warm sessions (sessions already claimed are not affected).\nProceed? [y/N]: ", warmPoolID)
				if _, err := fmt.Fprint(cmd.ErrOrStderr(), prompt); err != nil {
					return err
				}
				answer, err := cmdutil.ReadSecretInput(cmd.InOrStdin(), cmd.ErrOrStderr(), "", true)
				if err != nil {
					return err
				}
				if answer != "y" && answer != "Y" && answer != "yes" {
					return fmt.Errorf("aborted")
				}
			}
			if err := svc.DeleteWarmPool(cmd.Context(), workspaceID, warmPoolID); err != nil {
				return err
			}
			if !cmdutil.IsQuiet(cmd) {
				_, err := fmt.Fprintf(cmd.ErrOrStderr(), "Deleted warm pool %s\n", warmPoolID)
				return err
			}
			return nil
		},
	}
	c.Flags().BoolVar(&assumeYes, "yes", false, "skip the confirmation prompt")
	return c
}
