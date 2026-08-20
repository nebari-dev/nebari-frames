package mcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"connectrpc.com/connect"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/nebari-dev/nebari-frames/backend/internal/auth"
	"github.com/nebari-dev/nebari-frames/backend/internal/frames"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

type stubSource struct {
	readable []frames.ReadableFrame
	docs     map[string]*frames.Doc // key: org/name
	listErr  error
}

func (s stubSource) ListReadable(context.Context) ([]frames.ReadableFrame, error) {
	return s.readable, s.listErr
}

// PublishDoc and SourceDoc keep stubSource satisfying FrameSource for the
// read-path tests. Those tests never write, so reaching either is a bug in the
// code under test rather than an expected error - panic so it cannot be mistaken
// for a normal error result somewhere downstream.
func (s stubSource) PublishDocFrom(context.Context, *frames.Doc, string, frames.PublishIntent, string) (*framesv1.Frame, *framesv1.FrameVersion, error) {
	panic("stubSource: unexpected publish call from a read path")
}

func (s stubSource) SourceDoc(context.Context, string, string) (*frames.Doc, error) {
	panic("stubSource: unexpected SourceDoc call from a read path")
}

func (s stubSource) ResolveDoc(_ context.Context, org, name, _ string) (*frames.Doc, error) {
	d, ok := s.docs[org+"/"+name]
	if !ok {
		return nil, errors.New("not found") // mirror denied/missing -> error
	}
	return d, nil
}

func TestGetServer_DevModeBuildsServer(t *testing.T) {
	src := stubSource{
		readable: []frames.ReadableFrame{
			{OrgSlug: "openteams", OrgDisplay: "OpenTeams", Name: "alpha", Version: "1.0.0", Description: "A"},
		},
		docs: map[string]*frames.Doc{
			"openteams/alpha": {Name: "alpha", Description: "A", Version: "1.0.0", Slots: frames.Slots{Rules: []string{"r1"}}},
		},
	}
	rs := &resourceServer{src: src, cfg: Config{DevMode: true}}
	req := httptest.NewRequest("POST", "/mcp", nil)
	srv := rs.getServer(req)
	if srv == nil {
		t.Fatal("getServer returned nil in dev mode")
	}
}

func TestReadHandler(t *testing.T) {
	src := stubSource{
		docs: map[string]*frames.Doc{
			"openteams/alpha": {Name: "alpha", Description: "A", Version: "1.0.0", Slots: frames.Slots{Rules: []string{"r1"}}},
		},
	}
	rs := &resourceServer{src: src, cfg: Config{DevMode: true}}
	h := rs.readHandler(auth.DevClaims())

	t.Run("reads composed markdown", func(t *testing.T) {
		req := &gomcp.ReadResourceRequest{Params: &gomcp.ReadResourceParams{URI: formatFrameURI("openteams", "alpha", "1.0.0")}}
		res, err := h(context.Background(), req)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if len(res.Contents) != 1 || !strings.Contains(res.Contents[0].Text, "# Frame: alpha") {
			t.Errorf("unexpected contents: %+v", res.Contents)
		}
		if res.Contents[0].MIMEType != "text/markdown" {
			t.Errorf("mime=%q", res.Contents[0].MIMEType)
		}
	})

	t.Run("unreadable and missing are identical not-found", func(t *testing.T) {
		missing := &gomcp.ReadResourceRequest{Params: &gomcp.ReadResourceParams{URI: formatFrameURI("openteams", "ghost", "1.0.0")}}
		_, errMissing := h(context.Background(), missing)
		if errMissing == nil {
			t.Fatal("expected not-found for missing frame")
		}
		// A malformed URI must also be not-found (no distinct error surface).
		bad := &gomcp.ReadResourceRequest{Params: &gomcp.ReadResourceParams{URI: "https://evil/x"}}
		_, errBad := h(context.Background(), bad)
		if errBad == nil {
			t.Fatal("expected not-found for malformed URI")
		}
	})
}

// A transient ListReadable failure must not produce a nil server: the SDK turns
// a nil server into HTTP 400 (client error), misrepresenting a backend fault.
// getServer should log and serve an empty resource set instead.
func TestGetServer_ListErrorServesEmptyNotNil(t *testing.T) {
	rs := &resourceServer{src: stubSource{listErr: errors.New("db down")}, cfg: Config{DevMode: true}}
	req := httptest.NewRequest("POST", "/mcp", nil)
	if srv := rs.getServer(req); srv == nil {
		t.Fatal("getServer returned nil on ListReadable error; want non-nil empty server (avoids HTTP 400)")
	}
}

// Constructing the bearer-protected endpoint without a validator is a wiring
// bug; Mount must fail fast at startup rather than panic at request time.
func TestMount_PanicsWhenNonDevAndNilVerifier(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic: non-dev mode with nil verifier")
		}
	}()
	c := NewComponent(Config{DevMode: false, PublicURL: "https://frames.example.com"}, stubSource{}, nil)
	c.Mount(http.NewServeMux())
}

// toolText concatenates the text content of a tool result.
func toolText(res *gomcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*gomcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestListFramesTool(t *testing.T) {
	src := stubSource{readable: []frames.ReadableFrame{
		{OrgSlug: "openteams", OrgDisplay: "OpenTeams", Name: "alpha", Version: "1.0.0", Description: "A"},
		{OrgSlug: "openteams", OrgDisplay: "OpenTeams", Name: "beta", Version: "2.0.0", Description: "B"},
	}}
	rs := &resourceServer{src: src, cfg: Config{DevMode: true}}
	h := rs.listFramesTool(auth.DevClaims())
	res, _, err := h(context.Background(), &gomcp.CallToolRequest{}, listFramesInput{})
	if err != nil {
		t.Fatalf("list_frames: %v", err)
	}
	txt := toolText(res)
	if !strings.Contains(txt, "alpha") || !strings.Contains(txt, "beta") {
		t.Errorf("list_frames output missing frames: %q", txt)
	}
}

func TestGetFrameTool(t *testing.T) {
	src := stubSource{
		readable: []frames.ReadableFrame{{OrgSlug: "openteams", OrgDisplay: "OpenTeams", Name: "alpha", Version: "1.0.0", Description: "A"}},
		docs:     map[string]*frames.Doc{"openteams/alpha": {Name: "alpha", Description: "A", Version: "1.0.0", Slots: frames.Slots{Rules: []string{"r1"}}}},
	}
	rs := &resourceServer{src: src, cfg: Config{DevMode: true}}
	h := rs.getFrameTool(auth.DevClaims())

	t.Run("returns composed markdown for a readable frame", func(t *testing.T) {
		res, _, err := h(context.Background(), &gomcp.CallToolRequest{}, getFrameInput{Name: "alpha"})
		if err != nil {
			t.Fatalf("get_frame: %v", err)
		}
		if res.IsError {
			t.Fatalf("unexpected IsError; text=%q", toolText(res))
		}
		if !strings.Contains(toolText(res), "# Frame: alpha") {
			t.Errorf("get_frame missing composed markdown: %q", toolText(res))
		}
	})

	t.Run("unknown or unreadable frame -> not-found error result", func(t *testing.T) {
		res, _, err := h(context.Background(), &gomcp.CallToolRequest{}, getFrameInput{Name: "ghost"})
		if err != nil {
			t.Fatalf("get_frame: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected IsError result for unknown frame, got: %q", toolText(res))
		}
	})
}

// publishCall records what the stub was asked to publish so tests can assert the
// adapter passed the caller's input through faithfully.
type publishCall struct {
	doc         *frames.Doc
	changelog   string
	intent      frames.PublishIntent
	baseVersion string
}

type stubWriter struct {
	stubSource
	calls []publishCall
	err   error
	// current is the document SourceDoc returns; srcErr overrides it.
	current *frames.Doc
	srcErr  error
}

func (s *stubWriter) SourceDoc(context.Context, string, string) (*frames.Doc, error) {
	if s.srcErr != nil {
		return nil, s.srcErr
	}
	if s.current != nil {
		return s.current, nil
	}
	return &frames.Doc{}, nil
}

func (s *stubWriter) PublishDocFrom(_ context.Context, doc *frames.Doc, changelog string, intent frames.PublishIntent, baseVersion string) (*framesv1.Frame, *framesv1.FrameVersion, error) {
	s.calls = append(s.calls, publishCall{doc: doc, changelog: changelog, intent: intent, baseVersion: baseVersion})
	if s.err != nil {
		return nil, nil, s.err
	}
	return &framesv1.Frame{Name: doc.Name}, &framesv1.FrameVersion{Version: doc.Version}, nil
}

// ptr is shorthand for the optional string fields.
func ptr(s string) *string { return &s }

func TestWriteFrameTools(t *testing.T) {
	validInput := func() writeFrameInput {
		return writeFrameInput{
			Name:        "brand-voice",
			Description: ptr("How we write"),
			Version:     "1.0.0",
			// Required by update_frame and ignored by create_frame.
			BaseVersion: "0.9.0",
			Rules:       []string{"Cite benchmarks."},
		}
	}

	tests := []struct {
		name       string
		tool       string // "create" or "update"
		input      writeFrameInput
		publishErr error

		wantIsError bool
		wantIntent  frames.PublishIntent
		wantCalls   int
		wantText    string // substring the result must contain
	}{
		{
			name: "create publishes with create intent",
			tool: "create", input: validInput(),
			wantIntent: frames.PublishCreate, wantCalls: 1, wantText: "brand-voice",
		},
		{
			name: "update publishes with update intent",
			tool: "update", input: validInput(),
			wantIntent: frames.PublishUpdate, wantCalls: 1, wantText: "brand-voice",
		},
		{
			name: "a denied create surfaces an error result, not a transport error",
			tool: "create", input: validInput(),
			publishErr:  connect.NewError(connect.CodePermissionDenied, errors.New("publisher or admin role required")),
			wantIsError: true, wantCalls: 1, wantIntent: frames.PublishCreate, wantText: "permission",
		},
		{
			name: "an update of a missing frame surfaces an error result",
			tool: "update", input: validInput(),
			publishErr:  connect.NewError(connect.CodeNotFound, errors.New(`no frame named "ghost" to update`)),
			wantIsError: true, wantCalls: 1, wantIntent: frames.PublishUpdate, wantText: "not found",
		},
		{
			name: "a create over an existing name surfaces an error result",
			tool: "create", input: validInput(),
			publishErr:  connect.NewError(connect.CodeAlreadyExists, errors.New("already exists")),
			wantIsError: true, wantCalls: 1, wantIntent: frames.PublishCreate, wantText: "already exists",
		},
		{
			name: "an internal fault does not leak detail to the client",
			tool: "create", input: validInput(),
			publishErr:  connect.NewError(connect.CodeInternal, errors.New("sqlite: disk image is malformed")),
			wantIsError: true, wantCalls: 1, wantIntent: frames.PublishCreate, wantText: "could not publish frame",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &stubWriter{err: tt.publishErr}
			rs := &resourceServer{src: src, cfg: Config{DevMode: true}}

			h := rs.createFrameTool(auth.DevClaims())
			if tt.tool == "update" {
				h = rs.updateFrameTool(auth.DevClaims())
			}
			res, _, err := h(context.Background(), &gomcp.CallToolRequest{}, tt.input)
			if err != nil {
				t.Fatalf("tool returned transport error: %v", err)
			}
			if res.IsError != tt.wantIsError {
				t.Fatalf("IsError = %v, want %v; text=%q", res.IsError, tt.wantIsError, toolText(res))
			}
			if len(src.calls) != tt.wantCalls {
				t.Fatalf("PublishDoc called %d times, want %d", len(src.calls), tt.wantCalls)
			}
			if tt.wantText != "" && !strings.Contains(strings.ToLower(toolText(res)), strings.ToLower(tt.wantText)) {
				t.Errorf("result text %q should mention %q", toolText(res), tt.wantText)
			}
			if tt.wantCalls > 0 {
				call := src.calls[0]
				if call.intent != tt.wantIntent {
					t.Errorf("intent = %v, want %v", call.intent, tt.wantIntent)
				}
				if call.doc.Name != tt.input.Name || call.doc.Version != tt.input.Version {
					t.Errorf("doc = %+v, does not match input %+v", call.doc, tt.input)
				}
			}
		})
	}
}

// Every field of the input must reach the published document; one silently
// dropped by the adapter would lose organizational context with no error.
func TestWriteFrameInputCarriesEveryField(t *testing.T) {
	src := &stubWriter{}
	rs := &resourceServer{src: src, cfg: Config{DevMode: true}}
	h := rs.createFrameTool(auth.DevClaims())

	in := writeFrameInput{
		Name: "full", Description: ptr("d"), Version: "1.0.0",
		Visibility:      ptr("private"),
		Scope:           ptr("company"),
		Maintainer:      ptr("platform team"),
		Terminology:     []termInput{{Term: "Frame", Definition: "a context artifact"}},
		Rules:           []string{"rule"},
		Skills:          []string{"skill"},
		Prompts:         []string{"prompt"},
		ToolSpecs:       ptr("tools"),
		Goals:           ptr("goals"),
		Style:           ptr("style"),
		Norms:           ptr("norms"),
		Architecture:    ptr("architecture"),
		BusinessProcess: ptr("process"),
		Extends:         []extendInput{{Ref: "openteams/base", Version: "1.0.0"}},
		Excludes:        []string{"openteams/legacy"},
	}
	if _, _, err := h(context.Background(), &gomcp.CallToolRequest{}, in); err != nil {
		t.Fatalf("create_frame: %v", err)
	}
	if len(src.calls) != 1 {
		t.Fatalf("PublishDoc called %d times, want 1", len(src.calls))
	}
	got := src.calls[0].doc
	want := &frames.Doc{
		Name: "full", Description: "d", Version: "1.0.0",
		Visibility: "private", Scope: "company", Maintainer: "platform team",
		Extends:  []frames.ExtendRef{{Ref: "openteams/base", Version: "1.0.0"}},
		Excludes: []string{"openteams/legacy"},
		Slots: frames.Slots{
			Terminology:     []frames.Term{{Term: "Frame", Definition: "a context artifact"}},
			Rules:           []string{"rule"},
			Skills:          []string{"skill"},
			Prompts:         []string{"prompt"},
			ToolSpecs:       "tools",
			Goals:           "goals",
			Style:           "style",
			Norms:           "norms",
			Architecture:    "architecture",
			BusinessProcess: "process",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("doc mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

// A real drift guard. The previous version of this test compared against a
// hand-written literal, so a newly added slot or document field was zero on both
// sides and passed - which is exactly how visibility, scope, and maintainer came
// to be silently dropped. This walks the canonical definitions instead, so
// extending frames.Doc or frames.SlotTable without extending writeFrameInput
// fails here rather than in production.
func TestWriteFrameInputCoversDocFields(t *testing.T) {
	inputFields := map[string]bool{}
	inT := reflect.TypeOf(writeFrameInput{})
	for i := range inT.NumField() {
		name, _, _ := strings.Cut(inT.Field(i).Tag.Get("json"), ",")
		inputFields[name] = true
	}

	t.Run("every slot has an input field", func(t *testing.T) {
		for _, d := range frames.SlotTable {
			if !inputFields[d.Key] {
				t.Errorf("slot %q has no writeFrameInput field: MCP writes would silently drop it", d.Key)
			}
		}
		// Guard the other direction too: frames.Slots must not grow a field that
		// SlotTable does not describe.
		if got, want := reflect.TypeOf(frames.Slots{}).NumField(), len(frames.SlotTable); got != want {
			t.Errorf("frames.Slots has %d fields but SlotTable describes %d", got, want)
		}
	})

	t.Run("every document field is accounted for", func(t *testing.T) {
		// Doc-level fields that are not slots. "slots" is the container itself;
		// the rest are the Frame Spec metadata plus inheritance.
		expected := map[string]bool{
			"name": true, "description": true, "version": true,
			"visibility": true, "scope": true, "maintainer": true,
			"extends": true, "excludes": true, "slots": true,
		}
		docT := reflect.TypeOf(frames.Doc{})
		for i := range docT.NumField() {
			name, _, _ := strings.Cut(docT.Field(i).Tag.Get("yaml"), ",")
			if !expected[name] {
				t.Errorf("frames.Doc gained field %q: decide whether MCP writes must carry it, then add it here", name)
				continue
			}
			if name == "slots" {
				continue // covered by the slot walk above
			}
			if !inputFields[name] {
				t.Errorf("document field %q has no writeFrameInput field: MCP writes would silently drop it", name)
			}
		}
	})
}

// The write tools must be advertised, or a client has no way to call them.
func TestGetServerAdvertisesWriteTools(t *testing.T) {
	src := &stubWriter{}
	rs := &resourceServer{src: src, cfg: Config{DevMode: true}}
	srv := rs.getServer(httptest.NewRequest("POST", "/mcp", nil))
	if srv == nil {
		t.Fatal("getServer returned nil")
	}
	// The SDK exposes no tool listing on the server value, so assert through a
	// connected client session instead.
	ctx := context.Background()
	clientTransport, serverTransport := gomcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer func() { _ = serverSession.Close() }()
	client := gomcp.NewClient(&gomcp.Implementation{Name: "test", Version: "v1"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer func() { _ = cs.Close() }()

	res, err := cs.ListTools(ctx, &gomcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	got := map[string]bool{}
	for _, tool := range res.Tools {
		got[tool.Name] = true
	}
	for _, want := range []string{"list_frames", "get_frame", "create_frame", "update_frame"} {
		if !got[want] {
			t.Errorf("tool %q not advertised; got %v", want, got)
		}
	}
}

// Guards the remaining half of the drift problem: TestWriteFrameInputCoversDocFields
// forces a new slot or document field to gain an input field, but nothing forced
// that field to be wired into applyTo. An unwired field would leave the target
// at its zero value, which a hand-written want literal cannot catch because it
// is zero on both sides. This sets every input field to a distinct sentinel and
// asserts nothing in the resulting document is still zero.
func TestApplyToWiresEveryInputField(t *testing.T) {
	in := writeFrameInput{}
	v := reflect.ValueOf(&in).Elem()
	inT := v.Type()

	// Fill every field with a non-zero sentinel derived from its name.
	for i := range inT.NumField() {
		name := inT.Field(i).Name
		f := v.Field(i)
		switch f.Kind() {
		case reflect.String:
			f.SetString("sentinel-" + name)
		case reflect.Pointer:
			sv := reflect.New(f.Type().Elem())
			sv.Elem().SetString("sentinel-" + name)
			f.Set(sv)
		case reflect.Bool:
			f.SetBool(true)
		case reflect.Slice:
			elem := f.Type().Elem()
			ev := reflect.New(elem).Elem()
			switch elem.Kind() {
			case reflect.String:
				ev.SetString("sentinel-" + name)
			case reflect.Struct:
				for j := range elem.NumField() {
					if ev.Field(j).Kind() == reflect.String {
						ev.Field(j).SetString("sentinel-" + name)
					}
				}
			default:
				t.Fatalf("field %s: unhandled slice element kind %s", name, elem.Kind())
			}
			f.Set(reflect.Append(f, ev))
		default:
			t.Fatalf("field %s: unhandled kind %s; extend this test", name, f.Kind())
		}
	}
	// Name and Version must look like a valid frame for nothing else to matter,
	// but their values are irrelevant to the zero-check below.
	in.Name, in.Version = "sentinel-name", "1.0.0"

	got := in.applyTo(&frames.Doc{})

	// Changelog is publish metadata, not part of the document.
	docV := reflect.ValueOf(*got)
	docT := docV.Type()
	for i := range docT.NumField() {
		name := docT.Field(i).Name
		if name == "Slots" {
			continue
		}
		if docV.Field(i).IsZero() {
			t.Errorf("Doc.%s is zero after applyTo: the input field exists but is not wired in", name)
		}
	}
	slotsV := reflect.ValueOf(got.Slots)
	slotsT := slotsV.Type()
	for i := range slotsT.NumField() {
		if slotsV.Field(i).IsZero() {
			t.Errorf("Slots.%s is zero after applyTo: the input field exists but is not wired in", slotsT.Field(i).Name)
		}
	}
}

// The concurrency guard only works if update_frame reports the version it
// actually merged onto. Passing an empty base would leave the check inert while
// still looking correct.
func TestUpdateFrameSendsTheBaseVersionItRead(t *testing.T) {
	src := &stubWriter{current: &frames.Doc{
		Name: "brand-voice", Description: "d", Version: "3.4.5",
		Slots: frames.Slots{Rules: []string{"existing"}},
	}}
	rs := &resourceServer{src: src, cfg: Config{DevMode: true}}
	h := rs.updateFrameTool(auth.DevClaims())

	if _, _, err := h(context.Background(), &gomcp.CallToolRequest{}, writeFrameInput{
		Name: "brand-voice", Version: "3.5.0", BaseVersion: "3.4.5",
		Rules: []string{"existing", "new"},
	}); err != nil {
		t.Fatalf("update_frame: %v", err)
	}
	if len(src.calls) != 1 {
		t.Fatalf("publish called %d times, want 1", len(src.calls))
	}
	if got := src.calls[0].baseVersion; got != "3.4.5" {
		t.Errorf("baseVersion = %q, want the value the caller supplied, not one re-read server-side", got)
	}
}

// create_frame has nothing to be stale against, so it must not send a base.
func TestCreateFrameSendsNoBaseVersion(t *testing.T) {
	src := &stubWriter{}
	rs := &resourceServer{src: src, cfg: Config{DevMode: true}}
	h := rs.createFrameTool(auth.DevClaims())
	if _, _, err := h(context.Background(), &gomcp.CallToolRequest{}, writeFrameInput{
		Name: "n", Description: ptr("d"), Version: "1.0.0", Rules: []string{"r"},
	}); err != nil {
		t.Fatalf("create_frame: %v", err)
	}
	if got := src.calls[0].baseVersion; got != "" {
		t.Errorf("baseVersion = %q, want empty for a create", got)
	}
}
