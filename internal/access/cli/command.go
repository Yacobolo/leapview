package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type AuthenticationService interface {
	Login(context.Context, LoginRequest, func(DeviceChallenge)) (LoginResult, error)
	Logout(context.Context, string) error
}

type TargetMetadata struct {
	Origin      string
	InstanceID  string
	Environment string
}

type TargetDiscovery interface {
	Discover(context.Context, string) (TargetMetadata, error)
}

type ProjectIdentityResolver interface {
	ProjectID(string) (string, error)
}

func LoginCommand(ctx context.Context, authentication AuthenticationService, discovery TargetDiscovery, projects ProjectIdentityResolver) *cobra.Command {
	var name string
	projectPath := filepath.Join("dashboards", "leapview.yaml")
	var headless bool
	command := &cobra.Command{
		Use:   "login <target>",
		Short: "Sign in to a LeapView target for project authoring",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if authentication == nil || discovery == nil || projects == nil {
				return fmt.Errorf("login dependencies are unavailable")
			}
			origin := strings.TrimRight(strings.TrimSpace(args[0]), "/")
			metadata, err := discovery.Discover(ctx, origin)
			if err != nil {
				return fmt.Errorf("discover LeapView target: %w", err)
			}
			projectID, err := projects.ProjectID(projectPath)
			if err != nil {
				return fmt.Errorf("read authoring project identity: %w", err)
			}
			if strings.TrimSpace(projectID) == "" {
				return fmt.Errorf("authoring project %q has no identity", projectPath)
			}
			profileName := strings.TrimSpace(name)
			if profileName == "" {
				profileName = metadata.Origin
			}
			result, err := authentication.Login(ctx, LoginRequest{
				Name: profileName, Origin: metadata.Origin, InstanceID: metadata.InstanceID,
				Environment: metadata.Environment, ProjectID: projectID,
				Privileges: []string{
					"USE_WORKSPACE",
					"VIEW_ITEM",
					"AUTHOR_PROJECT",
					"PUBLISH_RELEASE",
					"REQUEST_DEPLOYMENT",
				},
				Headless: headless,
			}, func(challenge DeviceChallenge) {
				fmt.Fprintf(command.OutOrStdout(), "Open %s and enter code %s\n", challenge.VerificationURI, challenge.UserCode)
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Signed in to %s for project %s (session %s)\n", metadata.Origin, projectID, result.SessionID)
			return nil
		},
	}
	command.Flags().StringVar(&name, "name", "", "stable local name for this target")
	command.Flags().StringVar(&projectPath, "project", projectPath, "project entrypoint used to scope authoring credentials")
	command.Flags().BoolVar(&headless, "no-browser", false, "show the verification URL and code without opening a browser")
	return command
}

func LogoutCommand(ctx context.Context, authentication AuthenticationService) *cobra.Command {
	command := &cobra.Command{
		Use:   "logout <target>",
		Short: "Revoke a LeapView authoring session and remove local credentials",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if authentication == nil {
				return fmt.Errorf("logout dependencies are unavailable")
			}
			if err := authentication.Logout(ctx, strings.TrimSpace(args[0])); err != nil {
				return err
			}
			fmt.Fprintf(command.OutOrStdout(), "Signed out from %s\n", strings.TrimSpace(args[0]))
			return nil
		},
	}
	return command
}
