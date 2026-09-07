package cmd

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/nebari-dev/nebari-frames/cli/internal/testutil"
	framesv1 "github.com/nebari-dev/nebari-frames/gen/go/frames/v1"
)

func TestTemplateListCommand(t *testing.T) {
	tests := []struct {
		name       string
		templates  []*framesv1.FrameTemplateSummary
		wantLines  []string
		wantAbsent []string
	}{
		{
			name: "shows built-ins and org templates with their source",
			templates: []*framesv1.FrameTemplateSummary{
				{Id: "builtin:blank", Title: "Blank", Description: "Empty.", Builtin: true},
				{Id: "01JORG", Title: "Our House Style", Description: "Ours.", Builtin: false},
			},
			wantLines: []string{"builtin:blank", "Blank", "built-in", "01JORG", "Our House Style", "organization"},
		},
		{
			name:       "says so when there are none",
			templates:  nil,
			wantLines:  []string{"No templates"},
			wantAbsent: []string{"ID"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := testutil.NewStubServer(t, &testutil.StubService{
				ListTemplatesFn: func(_ context.Context, _ *connect.Request[framesv1.ListFrameTemplatesRequest]) (*connect.Response[framesv1.ListFrameTemplatesResponse], error) {
					return connect.NewResponse(&framesv1.ListFrameTemplatesResponse{Templates: tt.templates}), nil
				},
			})
			out := runCmd(t, url, "template", "list")
			for _, want := range tt.wantLines {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q:\n%s", want, out)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(out, absent) {
					t.Errorf("output should not contain %q:\n%s", absent, out)
				}
			}
		})
	}
}
