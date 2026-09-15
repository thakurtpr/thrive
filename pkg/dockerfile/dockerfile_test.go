//go:build linux
// +build linux

package dockerfile

import (
	"strings"
	"testing"
)

// TestParse_Basic verifies a typical single-stage Dockerfile converts.
func TestParse_Basic(t *testing.T) {
	text := `
FROM alpine:3.19
ENV APP_HOME=/app VERSION=1
WORKDIR /app
COPY app.bin /app/
RUN chmod +x /app/app.bin && echo done
EXPOSE 8080
CMD ["/app/app.bin", "--serve"]
`
	res, err := Parse(text, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Graph.BaseImage != "alpine:3.19" {
		t.Errorf("BaseImage: got %q", res.Graph.BaseImage)
	}
	if len(res.Graph.Steps) != 2 {
		t.Fatalf("Steps: got %d, want 2 (copy + run)", len(res.Graph.Steps))
	}
	copyStep := res.Graph.Steps["step-0"]
	if len(copyStep.Copy) != 1 || copyStep.Copy[0].Source != "app.bin" {
		t.Errorf("step-0 copy: got %+v", copyStep.Copy)
	}
	if copyStep.Copy[0].Dest != "/app/" {
		t.Errorf("step-0 dest: got %q", copyStep.Copy[0].Dest)
	}
	runStep := res.Graph.Steps["step-1"]
	if !strings.Contains(runStep.Run, "chmod +x") {
		t.Errorf("step-1 run: got %q", runStep.Run)
	}
	if len(res.Graph.EntryCmd) != 2 || res.Graph.EntryCmd[0] != "/app/app.bin" {
		t.Errorf("EntryCmd: got %v", res.Graph.EntryCmd)
	}
	if len(res.Warnings) == 0 {
		t.Error("expected EXPOSE warning, got none")
	}
}

// TestParse_BuildArgs verifies ARG defaults and --build-arg overrides.
func TestParse_BuildArgs(t *testing.T) {
	text := "FROM alpine\nARG VER=1.0\nARG CHANNEL\nRUN echo ${VER} ${CHANNEL:-stable}\n"
	res, err := Parse(text, map[string]string{"CHANNEL": "edge"})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	run := res.Graph.Steps["step-0"].Run
	if !strings.Contains(run, "1.0") || !strings.Contains(run, "edge") {
		t.Errorf("substitution: got %q", run)
	}
}

// TestParse_MultiStage verifies only the final stage converts with a warning.
func TestParse_MultiStage(t *testing.T) {
	text := "FROM golang:1.22 AS builder\nRUN go build -o app .\nFROM alpine\nCOPY --from=builder /app /app\n"
	res, err := Parse(text, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Graph.BaseImage != "alpine" {
		t.Errorf("BaseImage: got %q, want alpine", res.Graph.BaseImage)
	}
	if len(res.Warnings) == 0 {
		t.Error("expected multi-stage warning, got none")
	}
}

// TestParse_Errors verifies malformed Dockerfiles fail clearly.
func TestParse_Errors(t *testing.T) {
	for _, text := range []string{
		"RUN echo hi\n",       // no FROM
		"FROM \n",             // empty FROM
		"FROBNICATE x\n",      // unknown instruction
		"FROM alpine\nCOPY\n", // COPY without args
	} {
		if _, err := Parse(text, nil); err == nil {
			t.Errorf("Parse(%q): expected error, got nil", text)
		}
	}
}

// TestSubstitute verifies variable expansion forms.
func TestSubstitute(t *testing.T) {
	args := map[string]string{"A": "1", "EMPTY": ""}
	env := map[string]string{"E": "2"}
	cases := []struct{ in, want string }{
		{"$A", "1"},
		{"${A}", "1"},
		{"$E", "2"},
		{"${MISSING:-d}", "d"},
		{"${EMPTY:-d}", "d"},
		{"${A:+alt}", "alt"},
		{"${MISSING:+alt}", ""},
		{"$$A", "$A"},
		{"pre-$A-post", "pre-1-post"},
		{"$MISSING", ""},
	}
	for _, tc := range cases {
		if got := substitute(tc.in, args, env); got != tc.want {
			t.Errorf("substitute(%q): got %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestLogicalLines verifies continuations and comments.
func TestLogicalLines(t *testing.T) {
	text := "# comment\nRUN echo a \\\n  b\n\n# another\nFROM x\n"
	lines := logicalLines(text)
	if len(lines) != 2 {
		t.Fatalf("logicalLines: got %v", lines)
	}
	if lines[0] != "RUN echo a    b" {
		t.Errorf("continuation: got %q", lines[0])
	}
}
