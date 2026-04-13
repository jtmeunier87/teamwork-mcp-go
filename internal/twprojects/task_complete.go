package twprojects

// NOTE: Teamwork's v3 API has no dedicated task completion endpoint.
// The v3 PATCH /tasks/{id} with status:'completed' returns 400 for stage-0 tasks.
// The only reliable path is v1: PUT /tasks/{id}/complete.json and /uncomplete.json.
// This is an intentional v1 exception — documented in percy-process/knowledge/api-behaviour.md.
//
// These tools are the primary additions in the jtmeunier87/teamwork-mcp-go fork.
// Upstream (Teamwork/mcp) does not implement task completion.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/teamwork/mcp/internal/helpers"
	"github.com/teamwork/mcp/internal/toolsets"
	twapi "github.com/teamwork/twapi-go-sdk"
)

const (
	MethodTaskComplete   toolsets.Method = "twprojects-complete_task"
	MethodTaskUncomplete toolsets.Method = "twprojects-uncomplete_task"
)

// --- v1 complete/uncomplete request ---

type taskCompleteRequest struct {
	taskID int64
	action string // "complete" or "uncomplete"
}

func (r taskCompleteRequest) HTTPRequest(ctx context.Context, server string) (*http.Request, error) {
	url := server + "/tasks/" + strconv.FormatInt(r.taskID, 10) + "/" + r.action + ".json"
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

type taskCompleteResponse struct {
	Status string `json:"STATUS"`
}

func (r *taskCompleteResponse) HandleHTTPResponse(resp *http.Response) error {
	if resp.StatusCode != http.StatusOK {
		return twapi.NewHTTPError(resp, "task complete/uncomplete failed")
	}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

// --- TaskComplete tool ---

// TaskComplete marks a task as complete using the v1 API.
func TaskComplete(engine *twapi.Engine) toolsets.ToolWrapper {
	return toolsets.ToolWrapper{
		Tool: &mcp.Tool{
			Name: string(MethodTaskComplete),
			Description: "Mark a task as complete (checked off). " +
				"Uses the Teamwork v1 API — the only reliable completion path. " +
				"Note: completed tasks cannot be updated directly; uncomplete first, update, then re-complete.",
			Annotations: &mcp.ToolAnnotations{
				Title: "Complete Task",
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"id": {
						Type:        "integer",
						Description: "The ID of the task to mark as complete.",
					},
				},
				Required: []string{"id"},
			},
		},
		Handler: func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var arguments map[string]any
			if err := json.Unmarshal(request.Params.Arguments, &arguments); err != nil {
				return helpers.NewToolResultTextError("failed to decode request: %s", err.Error()), nil
			}
			var taskID int64
			if err := helpers.ParamGroup(arguments,
				helpers.RequiredNumericParam(&taskID, "id"),
			); err != nil {
				return helpers.NewToolResultTextError("invalid parameters: %s", err.Error()), nil
			}
			_, err := twapi.Execute[taskCompleteRequest, *taskCompleteResponse](
				ctx, engine, taskCompleteRequest{taskID: taskID, action: "complete"},
			)
			if err != nil {
				return helpers.HandleAPIError(err, "failed to complete task")
			}
			return helpers.NewToolResultText("Task %d marked as complete", taskID), nil
		},
	}
}

// --- TaskUncomplete tool ---

// TaskUncomplete reopens a completed task using the v1 API.
func TaskUncomplete(engine *twapi.Engine) toolsets.ToolWrapper {
	return toolsets.ToolWrapper{
		Tool: &mcp.Tool{
			Name: string(MethodTaskUncomplete),
			Description: "Reopen a completed task (mark as incomplete). " +
				"Required before updating a completed task — Teamwork does not allow updating completed tasks directly.",
			Annotations: &mcp.ToolAnnotations{
				Title: "Uncomplete Task",
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"id": {
						Type:        "integer",
						Description: "The ID of the task to reopen.",
					},
				},
				Required: []string{"id"},
			},
		},
		Handler: func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var arguments map[string]any
			if err := json.Unmarshal(request.Params.Arguments, &arguments); err != nil {
				return helpers.NewToolResultTextError("failed to decode request: %s", err.Error()), nil
			}
			var taskID int64
			if err := helpers.ParamGroup(arguments,
				helpers.RequiredNumericParam(&taskID, "id"),
			); err != nil {
				return helpers.NewToolResultTextError("invalid parameters: %s", err.Error()), nil
			}
			_, err := twapi.Execute[taskCompleteRequest, *taskCompleteResponse](
				ctx, engine, taskCompleteRequest{taskID: taskID, action: "uncomplete"},
			)
			if err != nil {
				return helpers.HandleAPIError(err, "failed to uncomplete task")
			}
			return helpers.NewToolResultText("Task %d reopened (marked incomplete)", taskID), nil
		},
	}
}
