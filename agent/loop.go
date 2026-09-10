// loop.go implements the actual agent decision-making: a standard LLM
// tool-calling loop (send the conversation + available tools, execute
// whatever the model calls, feed results back, repeat until it gives a final
// answer). This is what makes it an agent rather than a script that happens
// to speak MCP -- the LLM decides which tools to call and when, not this code.
package main

import (
	"context"
	"encoding/json"
	"log"
)

const maxLoopIterations = 6

const systemPrompt = `You are an on-call SRE incident response copilot. When asked about an incident:
1. Call get_unhealthy_pods to see what's failing.
2. Call get_deployment_history to check for a recent deployment that could be the cause.
3. If a previous, healthy version is available, call rollback_deployment with that exact deployment name and version to fix it.
4. Report back clearly and concisely what you found and what you did.
Only call rollback_deployment once you have identified a specific previous version from get_deployment_history - never guess a version.
If a tool you need for the next step isn't in your available tools list, don't retry other tools hoping it appears - report your findings so far and stop.
If asked "whoami", about your identity, or what token/claims a server received, call the whoami tool on the relevant server and report exactly what it returns - do not just describe what the tool does instead of calling it.`

// discoverTools lists every server's tools and returns them as one merged
// OpenAI-compatible tool spec list, plus a map from tool name to the server
// URL that owns it (tool names are unique across these three servers). One
// server being unreachable or misconfigured shouldn't take down tool
// discovery for the others, so a per-server error is logged and skipped --
// only a cancelled/expired context aborts the whole call.
func discoverTools(ctx context.Context, serverURLs []string, bearer string) ([]toolSpec, map[string]string, error) {
	var tools []toolSpec
	toolServer := map[string]string{}
	for _, url := range serverURLs {
		serverTools, err := listTools(ctx, url, bearer)
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, err
			}
			log.Printf("agent: skipping unreachable MCP server %s: %v", url, err)
			continue
		}
		for _, t := range serverTools {
			tools = append(tools, toolSpec{
				Type: "function",
				Function: functionSpec{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
			toolServer[t.Name] = url
		}
	}
	return tools, toolServer, nil
}

type loopResult struct {
	Steps      []string
	FinalText  string
	ConsentURL string
	// Set only when ConsentURL is non-empty -- what a later
	// resumeLoop call needs to retry the gated call and continue.
	Messages      []chatMessage
	PendingCall   toolCall
	PendingServer string
}

// runLoop drives the standard tool-calling loop: ask the LLM, execute
// whatever it calls, feed results back, until it stops calling tools or a
// call is gated by elicitation (in which case the caller should save the
// returned state and resume via resumeLoop once consent is complete).
func runLoop(ctx context.Context, llmURL, model, bearer, stsURL string, toolServer map[string]string, tools []toolSpec, messages []chatMessage) loopResult {
	var steps []string
	for i := 0; i < maxLoopIterations; i++ {
		msg, err := chatCompletion(ctx, llmURL, model, messages, tools)
		if err != nil {
			steps = append(steps, "agent: llm call failed: "+err.Error())
			return loopResult{Steps: steps}
		}
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			steps = append(steps, "agent: "+msg.Content)
			return loopResult{Steps: steps, FinalText: msg.Content, Messages: messages}
		}

		for _, tc := range msg.ToolCalls {
			serverURL, ok := toolServer[tc.Function.Name]
			if !ok {
				messages = append(messages, chatMessage{Role: "tool", ToolCallID: tc.ID, Content: "error: unknown tool " + tc.Function.Name})
				continue
			}

			var args any
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)

			steps = append(steps, "agent: calling "+tc.Function.Name)
			result, err := callToolJSON(ctx, serverURL, bearer, tc.Function.Name, args)
			if err != nil {
				elicitations, listErr := listElicitations(stsURL, bearer)
				if listErr == nil {
					if pending, ok := firstPending(elicitations); ok {
						steps = append(steps, "agent: "+tc.Function.Name+" requires one-time consent (elicitation)")
						return loopResult{
							Steps:         steps,
							ConsentURL:    pending.OAuth.AuthorizeURL,
							Messages:      messages,
							PendingCall:   tc,
							PendingServer: serverURL,
						}
					}
				}
				messages = append(messages, chatMessage{Role: "tool", ToolCallID: tc.ID, Content: "error: " + err.Error()})
				continue
			}
			messages = append(messages, chatMessage{Role: "tool", ToolCallID: tc.ID, Content: result})
		}
	}
	steps = append(steps, "agent: reached the iteration limit without a final answer")
	return loopResult{Steps: steps}
}

// resumeLoop retries the tool call an elicitation gated, appends its (now
// successful) result to the saved conversation, and continues the same loop.
func resumeLoop(ctx context.Context, llmURL, model, bearer, stsURL string, toolServer map[string]string, tools []toolSpec, messages []chatMessage, pending toolCall, pendingServer string) loopResult {
	var args any
	_ = json.Unmarshal([]byte(pending.Function.Arguments), &args)

	result, err := callToolJSON(ctx, pendingServer, bearer, pending.Function.Name, args)
	if err != nil {
		return loopResult{Steps: []string{"agent: retried " + pending.Function.Name + " still failed: " + err.Error()}}
	}
	messages = append(messages, chatMessage{Role: "tool", ToolCallID: pending.ID, Content: result})
	return runLoop(ctx, llmURL, model, bearer, stsURL, toolServer, tools, messages)
}
