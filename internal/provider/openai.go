package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"google.golang.org/genai"

	"github.com/yldm-ai/neon-switch/internal/translate"
)

// OpenAI is the experimental OpenAI backend. It converts the genai request that internal/translate builds into an OpenAI Responses API request and converts the result back into genai responses, so the Anthropic <-> genai translation and the server are shared with the Gemini backend unchanged. It is opt-in (NEOSWITCH_BACKEND=openai or `neoswitch backend openai`) and has only been exercised against a scripted upstream.
type OpenAI struct {
	client openai.Client
	Model  string
	// key is kept only to scrub it from upstream error messages.
	key string
}

// NewOpenAIFromEnv builds the backend from OPENAI_API_KEY, OPENAI_MODEL and the optional OPENAI_BASE_URL (which must include the API version path, for example https://api.openai.com/v1). OPENAI_MODEL is required: NeoSwitch does not guess a model name for this backend.
func NewOpenAIFromEnv() (*OpenAI, error) {
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" {
		return nil, errors.New("no OpenAI credentials: set OPENAI_API_KEY")
	}
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		return nil, errors.New("OPENAI_MODEL is not set: name the OpenAI model NeoSwitch should use")
	}
	opts := []option.RequestOption{
		option.WithAPIKey(key),
		// Claude Code owns retries and backs off on 429 and 529; SDK retries underneath would multiply them.
		option.WithMaxRetries(0),
	}
	if base := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")); base != "" {
		opts = append(opts, option.WithBaseURL(base))
	}
	return &OpenAI{client: openai.NewClient(opts...), Model: model, key: key}, nil
}

func (o *OpenAI) Generate(ctx context.Context, req translate.Request) (*genai.GenerateContentResponse, error) {
	params, err := o.params(req)
	if err != nil {
		return nil, err
	}
	resp, err := o.client.Responses.New(ctx, params)
	if err != nil {
		return nil, o.upstreamError(err)
	}
	return o.fromResponse(resp)
}

// Stream relays the Responses API event stream as genai chunks: each text delta becomes a text chunk, each function call is emitted whole once its output item is done, and the terminal event carries the finish reason and usage. Nothing is yielded before the first content event, so a request that fails up front keeps its HTTP status.
func (o *OpenAI) Stream(ctx context.Context, req translate.Request) iter.Seq2[*genai.GenerateContentResponse, error] {
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		params, err := o.params(req)
		if err != nil {
			yield(nil, err)
			return
		}
		stream := o.client.Responses.NewStreaming(ctx, params)
		defer stream.Close()
		refused := false
		for stream.Next() {
			event := stream.Current()
			switch event.Type {
			case "response.output_text.delta":
				if delta := event.AsResponseOutputTextDelta().Delta; delta != "" && !yield(partChunk(&genai.Part{Text: delta}), nil) {
					return
				}
			case "response.refusal.delta":
				refused = true
				if delta := event.AsResponseRefusalDelta().Delta; delta != "" && !yield(partChunk(&genai.Part{Text: delta}), nil) {
					return
				}
			case "response.output_item.done":
				part, err := outputItemPart(event.AsResponseOutputItemDone().Item)
				if err != nil {
					yield(nil, err)
					return
				}
				if part != nil && !yield(partChunk(part), nil) {
					return
				}
			case "response.completed", "response.incomplete", "response.failed":
				// The three terminal events share one shape; AsResponseCompleted only selects the response field.
				final := event.AsResponseCompleted().Response
				reason, err := o.finishReason(&final, refused)
				if err != nil {
					yield(nil, err)
					return
				}
				yield(&genai.GenerateContentResponse{
					Candidates:    []*genai.Candidate{{FinishReason: reason}},
					UsageMetadata: openAIUsage(final.Usage),
					ModelVersion:  final.Model,
				}, nil)
				return
			case "error":
				e := event.AsError()
				yield(nil, o.responseError(e.Code, e.Message))
				return
			}
		}
		if err := stream.Err(); err != nil {
			yield(nil, o.upstreamError(err))
			return
		}
		yield(nil, &translate.UpstreamError{Reason: "OpenAI stream ended before the response completed"})
	}
}

// CountTokens uses the Responses API input token count endpoint with the same input, instructions and tools the request would send.
func (o *OpenAI) CountTokens(ctx context.Context, req translate.Request) (int, error) {
	input, err := openAIInput(req.Contents)
	if err != nil {
		return 0, err
	}
	tools, err := openAITools(req.Config.Tools)
	if err != nil {
		return 0, err
	}
	params := responses.InputTokenCountParams{
		Model: openai.String(o.Model),
		Input: responses.InputTokenCountParamsInputUnion{OfResponseInputItemArray: input},
		Tools: tools,
	}
	if instructions := openAIInstructions(req.Config.SystemInstruction); instructions != "" {
		params.Instructions = openai.String(instructions)
	}
	mode, name := openAIToolChoice(req.Config.ToolConfig)
	if name != "" {
		params.ToolChoice.OfFunctionTool = &responses.ToolChoiceFunctionParam{Name: name}
	} else if mode != "" {
		params.ToolChoice.OfToolChoiceMode = openai.Opt(mode)
	}
	resp, err := o.client.Responses.InputTokens.Count(ctx, params)
	if err != nil {
		return 0, o.upstreamError(err)
	}
	return int(resp.InputTokens), nil
}

// Check verifies that the key can use the model: models.retrieve separates an unknown model from a rejected key, and a short generation catches quota, billing and region limits that only apply when generating.
func (o *OpenAI) Check(ctx context.Context) error {
	if _, err := o.client.Models.Get(ctx, o.Model); err != nil {
		return o.checkError(ctx, "look up model "+o.Model, err)
	}
	_, err := o.client.Responses.New(ctx, responses.ResponseNewParams{
		Model:           o.Model,
		Input:           responses.ResponseNewParamsInputUnion{OfString: openai.String("Reply with OK.")},
		MaxOutputTokens: openai.Int(256),
		Store:           openai.Bool(false),
	})
	if err != nil {
		return o.checkError(ctx, "generate with "+o.Model, err)
	}
	return nil
}

// ListModels returns the IDs of every model the key can see, sorted. The OpenAI models API does not say what a model supports, so embedding, audio and image models are listed too.
func (o *OpenAI) ListModels(ctx context.Context) ([]string, error) {
	var ids []string
	pager := o.client.Models.ListAutoPaging(ctx)
	for pager.Next() {
		ids = append(ids, pager.Current().ID)
	}
	if err := pager.Err(); err != nil {
		return nil, o.checkError(ctx, "list models", err)
	}
	slices.Sort(ids)
	return ids, nil
}

// params builds the Responses API request. Requests are sent with store=false so OpenAI does not retain the conversation; NeoSwitch is stateless and replays the history on every turn. stop_sequences and top_k have no Responses API equivalent and are not sent. The request's effort is not sent as reasoning.effort because OPENAI_MODEL may name a model without reasoning support, which rejects the parameter.
func (o *OpenAI) params(req translate.Request) (responses.ResponseNewParams, error) {
	input, err := openAIInput(req.Contents)
	if err != nil {
		return responses.ResponseNewParams{}, err
	}
	tools, err := openAITools(req.Config.Tools)
	if err != nil {
		return responses.ResponseNewParams{}, err
	}
	params := responses.ResponseNewParams{
		Model: o.Model,
		Input: responses.ResponseNewParamsInputUnion{OfInputItemList: input},
		Tools: tools,
		Store: openai.Bool(false),
	}
	config := req.Config
	if instructions := openAIInstructions(config.SystemInstruction); instructions != "" {
		params.Instructions = openai.String(instructions)
	}
	if config.MaxOutputTokens > 0 {
		params.MaxOutputTokens = openai.Int(int64(config.MaxOutputTokens))
	}
	if config.Temperature != nil {
		params.Temperature = openai.Float(widen(*config.Temperature))
	}
	if config.TopP != nil {
		params.TopP = openai.Float(widen(*config.TopP))
	}
	mode, name := openAIToolChoice(config.ToolConfig)
	if name != "" {
		params.ToolChoice.OfFunctionTool = &responses.ToolChoiceFunctionParam{Name: name}
	} else if mode != "" {
		params.ToolChoice.OfToolChoiceMode = openai.Opt(mode)
	}
	return params, nil
}

func openAIInstructions(system *genai.Content) string {
	if system == nil {
		return ""
	}
	var texts []string
	for _, part := range system.Parts {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, "\n\n")
}

// openAIInput converts genai contents into Responses API input items, keeping their order. A function call keeps its call_id in FunctionCall.ID, which internal/translate recovers from the tool_use ID it minted. Tool IDs minted elsewhere (a session started on Anthropic and resumed here) carry no call ID; those calls get a synthetic call_id, and their results are paired with them by function name in order.
func openAIInput(contents []*genai.Content) (responses.ResponseInputParam, error) {
	var items responses.ResponseInputParam
	pending := map[string][]string{}
	synthetic := 0
	for i, content := range contents {
		assistant := content.Role == genai.RoleModel
		var user responses.ResponseInputMessageContentListParam
		flushUser := func() {
			if len(user) == 0 {
				return
			}
			items = append(items, responses.ResponseInputItemUnionParam{OfMessage: &responses.EasyInputMessageParam{
				Role:    responses.EasyInputMessageRoleUser,
				Content: responses.EasyInputMessageContentUnionParam{OfInputItemContentList: user},
			}})
			user = nil
		}
		for j, part := range content.Parts {
			switch {
			case part.FunctionCall != nil:
				if !assistant {
					return nil, fmt.Errorf("contents[%d].parts[%d]: function calls are only valid in assistant turns", i, j)
				}
				call := part.FunctionCall
				callID := call.ID
				if callID == "" {
					synthetic++
					callID = fmt.Sprintf("call_ns_%d", synthetic)
					pending[call.Name] = append(pending[call.Name], callID)
				}
				arguments := []byte("{}")
				if call.Args != nil {
					var err error
					if arguments, err = json.Marshal(call.Args); err != nil {
						return nil, fmt.Errorf("contents[%d].parts[%d]: encode arguments of %q: %w", i, j, call.Name, err)
					}
				}
				items = append(items, responses.ResponseInputItemUnionParam{OfFunctionCall: &responses.ResponseFunctionToolCallParam{
					CallID:    callID,
					Name:      call.Name,
					Arguments: string(arguments),
				}})
			case part.FunctionResponse != nil:
				flushUser()
				response := part.FunctionResponse
				callID := response.ID
				if callID == "" {
					queue := pending[response.Name]
					if len(queue) == 0 {
						return nil, fmt.Errorf("contents[%d].parts[%d]: result of %q has no matching function call", i, j, response.Name)
					}
					callID, pending[response.Name] = queue[0], queue[1:]
				}
				output, err := functionOutput(response)
				if err != nil {
					return nil, fmt.Errorf("contents[%d].parts[%d]: %w", i, j, err)
				}
				items = append(items, responses.ResponseInputItemUnionParam{OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
					CallID: openai.String(callID),
					Output: output,
				}})
			case part.Text != "":
				if assistant {
					// Assistant input messages take plain text (sent back as output_text); input_text content is rejected there.
					items = append(items, responses.ResponseInputItemUnionParam{OfMessage: &responses.EasyInputMessageParam{
						Role:    responses.EasyInputMessageRoleAssistant,
						Content: responses.EasyInputMessageContentUnionParam{OfString: openai.String(part.Text)},
					}})
					continue
				}
				user = append(user, responses.ResponseInputContentUnionParam{OfInputText: &responses.ResponseInputTextParam{Text: part.Text}})
			case part.InlineData != nil:
				if assistant {
					return nil, fmt.Errorf("contents[%d].parts[%d]: OpenAI accepts images and documents only in user turns", i, j)
				}
				media, err := inputMedia(part.InlineData.MIMEType, part.InlineData.Data)
				if err != nil {
					return nil, fmt.Errorf("contents[%d].parts[%d]: %w", i, j, err)
				}
				user = append(user, media)
			default:
				return nil, fmt.Errorf("contents[%d].parts[%d]: part type is not supported by the OpenAI backend", i, j)
			}
		}
		flushUser()
	}
	return items, nil
}

// inputMedia converts inline data into an input_image or input_file content part, sent as a data URL.
func inputMedia(mimeType string, data []byte) (responses.ResponseInputContentUnionParam, error) {
	url := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return responses.ResponseInputContentUnionParam{OfInputImage: &responses.ResponseInputImageParam{
			ImageURL: openai.String(url),
			Detail:   responses.ResponseInputImageDetailAuto,
		}}, nil
	case mimeType == "application/pdf":
		return responses.ResponseInputContentUnionParam{OfInputFile: &responses.ResponseInputFileParam{
			FileData: openai.String(url),
			Filename: openai.String("document.pdf"),
		}}, nil
	default:
		return responses.ResponseInputContentUnionParam{}, fmt.Errorf("media type %q is not supported by the OpenAI backend", mimeType)
	}
}

// functionOutput converts a genai function response into function_call_output content. internal/translate puts the tool_result text under "output", or "error" when is_error is set; the Responses API has no error flag, so both are sent as the output text. Images and PDFs inside the result become content items after the text.
func functionOutput(response *genai.FunctionResponse) (responses.ResponseInputItemFunctionCallOutputOutputUnionParam, error) {
	var text string
	if value, ok := response.Response["output"].(string); ok {
		text = value
	} else if value, ok := response.Response["error"].(string); ok {
		text = value
	} else if response.Response != nil {
		data, err := json.Marshal(response.Response)
		if err != nil {
			return responses.ResponseInputItemFunctionCallOutputOutputUnionParam{}, fmt.Errorf("encode result of %q: %w", response.Name, err)
		}
		text = string(data)
	}
	if len(response.Parts) == 0 {
		return responses.ResponseInputItemFunctionCallOutputOutputUnionParam{OfString: openai.String(text)}, nil
	}
	var list responses.ResponseFunctionCallOutputItemListParam
	if text != "" {
		list = append(list, responses.ResponseFunctionCallOutputItemUnionParam{OfInputText: &responses.ResponseInputTextContentParam{Text: text}})
	}
	for _, part := range response.Parts {
		if part.InlineData == nil {
			return responses.ResponseInputItemFunctionCallOutputOutputUnionParam{}, fmt.Errorf("result of %q has a part type the OpenAI backend does not support", response.Name)
		}
		media, err := inputMedia(part.InlineData.MIMEType, part.InlineData.Data)
		if err != nil {
			return responses.ResponseInputItemFunctionCallOutputOutputUnionParam{}, fmt.Errorf("result of %q: %w", response.Name, err)
		}
		switch {
		case media.OfInputImage != nil:
			list = append(list, responses.ResponseFunctionCallOutputItemUnionParam{OfInputImage: &responses.ResponseInputImageContentParam{ImageURL: media.OfInputImage.ImageURL}})
		case media.OfInputFile != nil:
			list = append(list, responses.ResponseFunctionCallOutputItemUnionParam{OfInputFile: &responses.ResponseInputFileContentParam{FileData: media.OfInputFile.FileData, Filename: media.OfInputFile.Filename}})
		}
	}
	return responses.ResponseInputItemFunctionCallOutputOutputUnionParam{OfResponseFunctionCallOutputItemArray: list}, nil
}

// openAITools converts function declarations into function tools. The JSON Schema from input_schema is passed as-is with strict mode off: strict mode requires every property to be required and additionalProperties to be false, which Claude Code's tool schemas do not satisfy.
func openAITools(tools []*genai.Tool) ([]responses.ToolUnionParam, error) {
	var out []responses.ToolUnionParam
	for _, tool := range tools {
		for _, declaration := range tool.FunctionDeclarations {
			parameters := map[string]any{"type": "object", "properties": map[string]any{}}
			if declaration.ParametersJsonSchema != nil {
				schema, ok := declaration.ParametersJsonSchema.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("tool %q: input_schema must be a JSON object", declaration.Name)
				}
				parameters = schema
			}
			function := &responses.FunctionToolParam{
				Name:       declaration.Name,
				Parameters: parameters,
				Strict:     openai.Bool(false),
			}
			if declaration.Description != "" {
				function.Description = openai.String(declaration.Description)
			}
			out = append(out, responses.ToolUnionParam{OfFunction: function})
		}
	}
	return out, nil
}

// openAIToolChoice maps the function calling mode: AUTO -> auto, NONE -> none, ANY -> required, and ANY restricted to one function -> that function.
func openAIToolChoice(config *genai.ToolConfig) (responses.ToolChoiceOptions, string) {
	if config == nil || config.FunctionCallingConfig == nil {
		return "", ""
	}
	calling := config.FunctionCallingConfig
	switch calling.Mode {
	case genai.FunctionCallingConfigModeAuto:
		return responses.ToolChoiceOptionsAuto, ""
	case genai.FunctionCallingConfigModeNone:
		return responses.ToolChoiceOptionsNone, ""
	case genai.FunctionCallingConfigModeAny:
		if len(calling.AllowedFunctionNames) == 1 {
			return "", calling.AllowedFunctionNames[0]
		}
		return responses.ToolChoiceOptionsRequired, ""
	}
	return "", ""
}

// fromResponse converts a complete Responses API result into one genai candidate.
func (o *OpenAI) fromResponse(resp *responses.Response) (*genai.GenerateContentResponse, error) {
	content := &genai.Content{Role: genai.RoleModel}
	refused := false
	for _, item := range resp.Output {
		if item.Type == "message" {
			for _, c := range item.Content {
				switch c.Type {
				case "output_text":
					if c.Text != "" {
						content.Parts = append(content.Parts, &genai.Part{Text: c.Text})
					}
				case "refusal":
					refused = true
					if c.Refusal != "" {
						content.Parts = append(content.Parts, &genai.Part{Text: c.Refusal})
					}
				}
			}
			continue
		}
		part, err := outputItemPart(item)
		if err != nil {
			return nil, err
		}
		if part != nil {
			content.Parts = append(content.Parts, part)
		}
	}
	reason, err := o.finishReason(resp, refused)
	if err != nil {
		return nil, err
	}
	return &genai.GenerateContentResponse{
		Candidates:    []*genai.Candidate{{Content: content, FinishReason: reason}},
		UsageMetadata: openAIUsage(resp.Usage),
		ModelVersion:  resp.Model,
	}, nil
}

// outputItemPart converts a non-message output item: a function call becomes a genai FunctionCall carrying the call_id, reasoning items are dropped (they are not replayed, see docs/protocol-contract.md), and messages are handled by the caller. Any other item type means the model used a tool NeoSwitch never declared.
func outputItemPart(item responses.ResponseOutputItemUnion) (*genai.Part, error) {
	switch item.Type {
	case "message", "reasoning":
		return nil, nil
	case "function_call":
		call := item.AsFunctionCall()
		var args map[string]any
		if strings.TrimSpace(call.Arguments) != "" {
			if err := json.Unmarshal([]byte(call.Arguments), &args); err != nil {
				return nil, &translate.UpstreamError{Reason: fmt.Sprintf("OpenAI returned malformed arguments for function %q", call.Name)}
			}
		}
		return &genai.Part{FunctionCall: &genai.FunctionCall{ID: call.CallID, Name: call.Name, Args: args}}, nil
	default:
		return nil, &translate.UpstreamError{Reason: fmt.Sprintf("OpenAI returned an output item of type %q, which Claude Code cannot receive", item.Type)}
	}
}

// finishReason maps the response status onto the genai finish reasons the server already understands: completed -> STOP (SAFETY when the model refused), incomplete for max_output_tokens -> MAX_TOKENS, incomplete for content_filter -> SAFETY. A failed response becomes an error with an HTTP-like status.
func (o *OpenAI) finishReason(resp *responses.Response, refused bool) (genai.FinishReason, error) {
	switch resp.Status {
	case responses.ResponseStatusCompleted:
		if refused {
			return genai.FinishReasonSafety, nil
		}
		return genai.FinishReasonStop, nil
	case responses.ResponseStatusIncomplete:
		switch resp.IncompleteDetails.Reason {
		case "max_output_tokens":
			return genai.FinishReasonMaxTokens, nil
		case "content_filter":
			return genai.FinishReasonSafety, nil
		}
		return "", &translate.UpstreamError{Reason: "OpenAI stopped the response early: " + resp.IncompleteDetails.Reason}
	case responses.ResponseStatusFailed:
		return "", o.responseError(string(resp.Error.Code), resp.Error.Message)
	}
	return "", &translate.UpstreamError{Reason: fmt.Sprintf("OpenAI returned a response with status %q", resp.Status)}
}

// responseError converts an error reported inside a response or stream (not as an HTTP status) into a genai.APIError with the status the same failure would have had over HTTP, so the server maps it the same way.
func (o *OpenAI) responseError(code, message string) error {
	status := http.StatusBadGateway
	switch {
	case code == "rate_limit_exceeded":
		status = http.StatusTooManyRequests
	case code == "server_error":
		status = http.StatusInternalServerError
	case code == "vector_store_timeout":
		status = http.StatusGatewayTimeout
	case code == "invalid_prompt", strings.Contains(code, "image"):
		status = http.StatusBadRequest
	}
	if message == "" {
		message = "response failed"
	}
	return genai.APIError{Code: status, Status: code, Message: "OpenAI: " + o.redact(message)}
}

// upstreamError converts an SDK HTTP error into the genai.APIError the server maps onto Anthropic statuses. Other errors (network, timeout) are returned unchanged; they never contain the key.
func (o *OpenAI) upstreamError(err error) error {
	var apiErr *openai.Error
	if !errors.As(err, &apiErr) {
		return err
	}
	message := o.redact(apiErr.Message)
	if message == "" {
		message = http.StatusText(apiErr.StatusCode)
	}
	if apiErr.Code == "context_length_exceeded" {
		// internal/server recognises a token-limit 400 by this wording and answers "prompt is too long", which makes Claude Code offer compaction.
		message = "input token count exceeds the model's context window: " + message
	}
	return genai.APIError{Code: apiErr.StatusCode, Status: apiErr.Type, Message: "OpenAI: " + message}
}

func (o *OpenAI) checkError(ctx context.Context, step string, err error) error {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		return fmt.Errorf("%s: %s (HTTP %d %s: %s)", step, diagnoseOpenAI(apiErr), apiErr.StatusCode, apiErr.Code, o.redact(apiErr.Message))
	}
	if ctx.Err() != nil {
		return fmt.Errorf("%s: OpenAI did not answer in time", step)
	}
	return fmt.Errorf("%s: could not reach OpenAI: %w", step, err)
}

// diagnoseOpenAI names the cause of a failed check in terms the user can act on.
func diagnoseOpenAI(e *openai.Error) string {
	switch {
	case e.StatusCode == http.StatusUnauthorized || e.Code == "invalid_api_key":
		return "authentication failed: OPENAI_API_KEY was rejected"
	case e.Code == "unsupported_country_region_territory":
		return "region restricted: OpenAI does not serve requests from this location"
	case e.Code == "insufficient_quota":
		return "quota exhausted: the account has no remaining credit or billing is not set up"
	case e.StatusCode == http.StatusForbidden:
		return "permission denied: this key or project has no access to the model"
	case e.StatusCode == http.StatusNotFound || e.Code == "model_not_found":
		return "model not found: check OPENAI_MODEL and whether this key can use it"
	case e.StatusCode == http.StatusTooManyRequests:
		return "rate limit reached"
	case e.StatusCode >= http.StatusInternalServerError:
		return "OpenAI service unavailable"
	default:
		return "request rejected"
	}
}

// keyPattern matches OpenAI secret keys, including the masked form OpenAI echoes in "Incorrect API key provided" errors.
var keyPattern = regexp.MustCompile(`sk-[A-Za-z0-9_*-]+`)

// redact removes the configured key and anything shaped like an OpenAI key from upstream text before it reaches Claude Code or the terminal.
func (o *OpenAI) redact(message string) string {
	if o.key != "" {
		message = strings.ReplaceAll(message, o.key, "[redacted]")
	}
	return keyPattern.ReplaceAllString(message, "[redacted]")
}

func openAIUsage(usage responses.ResponseUsage) *genai.GenerateContentResponseUsageMetadata {
	reasoning := usage.OutputTokensDetails.ReasoningTokens
	return &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:        int32(usage.InputTokens),
		CachedContentTokenCount: int32(usage.InputTokensDetails.CachedTokens),
		CandidatesTokenCount:    int32(usage.OutputTokens - reasoning),
		ThoughtsTokenCount:      int32(reasoning),
		TotalTokenCount:         int32(usage.TotalTokens),
	}
}

// widen converts a float32 from genai's config back to the shortest float64 with the same decimal form, so 0.9 is sent as 0.9 rather than 0.8999999761581421.
func widen(f float32) float64 {
	value, _ := strconv.ParseFloat(strconv.FormatFloat(float64(f), 'g', -1, 32), 64)
	return value
}

func partChunk(part *genai.Part) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{Candidates: []*genai.Candidate{{Content: &genai.Content{Role: genai.RoleModel, Parts: []*genai.Part{part}}}}}
}
