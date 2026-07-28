// Package cli owns the Agent capability's command behavior.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"text/tabwriter"
	"time"

	"github.com/Yacobolo/leapview/internal/agent/api"
	agenttools "github.com/Yacobolo/leapview/internal/agent/tools"
	"github.com/Yacobolo/leapview/internal/platform/cliapi"
	"github.com/spf13/cobra"
)

// Dependencies are application facilities required by the Agent CLI adapter.
type Dependencies struct {
	Client     cliapi.Client
	Operations func() []agenttools.APIGenOperation
}

type options struct {
	target       string
	token        string
	conversation string
	jsonOutput   bool
	limit        int
	pageToken    string
}

// Command constructs the Agent command tree without depending on application
// globals or process startup.
func Command(ctx context.Context, dependencies Dependencies) *cobra.Command {
	values := &options{}
	parent := &cobra.Command{Use: "agent", Short: "Use the LeapView read-only agent"}
	ask := &cobra.Command{
		Use:   "ask [question]",
		Short: "Ask the LeapView read-only agent a question",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAsk(ctx, dependencies.Client, values, args[0], cmd.OutOrStdout())
		},
	}
	ask.Flags().StringVar(&values.target, "target", "", "LeapView server URL")
	ask.Flags().StringVar(&values.token, "token", "", "API token")
	ask.Flags().StringVar(&values.conversation, "conversation", "", "existing agent conversation id")
	ask.Flags().BoolVar(&values.jsonOutput, "json", false, "print JSON response")

	conversations := &cobra.Command{
		Use:   "conversations",
		Short: "List agent conversations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConversations(ctx, dependencies.Client, values, cmd.OutOrStdout())
		},
	}
	conversations.Flags().StringVar(&values.target, "target", "", "LeapView server URL")
	conversations.Flags().StringVar(&values.token, "token", "", "API token")
	conversations.Flags().BoolVar(&values.jsonOutput, "json", false, "print JSON response")
	conversations.Flags().IntVar(&values.limit, "limit", 0, "maximum items to return")
	conversations.Flags().StringVar(&values.pageToken, "page-token", "", "opaque page token")

	tools := &cobra.Command{
		Use:   "tools",
		Short: "List the canonical agent tools",
		Long:  "List the canonical agent tools exposed by built-in chat and deployment MCP, including each tool's privilege, effect, defaults, closed input schema, and backing operation.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTools(cmd.OutOrStdout(), dependencies.Operations)
		},
	}

	parent.AddCommand(ask, conversations, tools)
	return parent
}

func runAsk(ctx context.Context, client cliapi.Client, values *options, question string, out io.Writer) error {
	if client == nil {
		return fmt.Errorf("agent CLI API client is required")
	}
	credentials := cliapi.Credentials{Target: values.target, Token: values.token}
	conversationID := values.conversation
	if conversationID == "" {
		var conversation api.AgentConversationResponse
		if err := client.DoJSON(ctx, credentials, cliapi.Request{
			Method: http.MethodPost, OperationID: "createAgentConversation",
			Headers: map[string]string{"Idempotency-Key": fmt.Sprintf("cli-conversation-%d", time.Now().UnixNano())},
			Body:    api.AgentConversationCreateRequest{Title: "CLI conversation"},
		}, &conversation); err != nil {
			return err
		}
		conversationID = conversation.ID
	}
	var run api.AgentRunResponse
	if err := client.DoJSON(ctx, credentials, cliapi.Request{
		Method: http.MethodPost, OperationID: "createAgentRun",
		PathParams: map[string]string{"conversation": conversationID},
		Headers:    map[string]string{"Idempotency-Key": fmt.Sprintf("cli-run-%d", time.Now().UnixNano())},
		Body:       api.AgentTurnRequest{Input: question},
	}, &run); err != nil {
		return err
	}
	for run.Status == "queued" || run.Status == "running" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
		if err := client.DoJSON(ctx, credentials, cliapi.Request{
			Method: http.MethodGet, OperationID: "getAgentRun",
			PathParams: map[string]string{"conversation": conversationID, "run": run.ID},
		}, &run); err != nil {
			return err
		}
	}
	var messages listResponse[api.AgentMessageResponse]
	if err := client.DoJSON(ctx, credentials, cliapi.Request{
		Method: http.MethodGet, OperationID: "listAgentMessages",
		PathParams: map[string]string{"conversation": conversationID},
	}, &messages); err != nil {
		return err
	}
	content := ""
	for _, message := range messages.Items {
		if message.RunID == run.ID && message.Role == "assistant" && message.ContentText != "" {
			content = message.ContentText
		}
	}
	if values.jsonOutput {
		return json.NewEncoder(out).Encode(map[string]any{"conversationId": conversationID, "run": run, "content": content})
	}
	fmt.Fprintln(out, content)
	fmt.Fprintf(out, "\nconversation=%s run=%s stop=%s\n", conversationID, run.ID, run.StopReason)
	if run.Status != "completed" {
		return fmt.Errorf("agent run ended with status %s: %s", run.Status, run.Error)
	}
	return nil
}

func runConversations(ctx context.Context, client cliapi.Client, values *options, out io.Writer) error {
	if client == nil {
		return fmt.Errorf("agent CLI API client is required")
	}
	query := url.Values{}
	if values.limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", values.limit))
	}
	if values.pageToken != "" {
		query.Set("pageToken", values.pageToken)
	}
	var response listResponse[api.AgentConversationResponse]
	if err := client.DoJSON(ctx, cliapi.Credentials{Target: values.target, Token: values.token}, cliapi.Request{
		Method: http.MethodGet, OperationID: "listAgentConversations", Query: query,
	}, &response); err != nil {
		return err
	}
	if values.jsonOutput {
		return json.NewEncoder(out).Encode(response.Items)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tTITLE\tUPDATED")
	for _, row := range response.Items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", row.ID, row.Status, row.Title, row.UpdatedAt)
	}
	return tw.Flush()
}

func runTools(out io.Writer, operations func() []agenttools.APIGenOperation) error {
	if operations == nil {
		return fmt.Errorf("agent CLI operation catalog is required")
	}
	reference, err := agenttools.ReferenceCatalog(operations())
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tPRIVILEGE\tEFFECT\tDEFAULTS\tINPUT_SCHEMA\tOPERATION")
	for _, tool := range reference {
		defaults, _ := json.Marshal(tool.Defaults)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			tool.Name, tool.Privilege, tool.Effect, defaults, compactJSON(tool.InputSchema), tool.OperationID)
	}
	return tw.Flush()
}

func compactJSON(value json.RawMessage) string {
	var output bytes.Buffer
	if err := json.Compact(&output, value); err != nil {
		return string(value)
	}
	return output.String()
}

type listResponse[T any] struct {
	Items []T `json:"items"`
	Page  struct {
		NextCursor string `json:"nextCursor"`
	} `json:"page"`
}
