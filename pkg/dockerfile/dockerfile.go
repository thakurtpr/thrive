//go:build linux
// +build linux

// Package dockerfile parses Dockerfiles into a thrivefile.BuildGraph so
// `thrive build` accepts both Thrivefiles and Dockerfiles. Single-stage
// builds are fully supported; multi-stage builds convert the final stage
// (earlier stages are reported as skipped). Unsupported instructions are
// reported as warnings, never silent.
package dockerfile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thakurprasadrout/thrive/pkg/thrivefile"
)

// Result is a parsed Dockerfile with conversion warnings.
type Result struct {
	Graph    *thrivefile.BuildGraph
	Warnings []string
}

// ParseFile reads path as a Dockerfile with the given build args.
func ParseFile(path string, buildArgs map[string]string) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("dockerfile: read %s: %w", path, err)
	}
	return Parse(string(data), buildArgs)
}

// Parse converts Dockerfile text to a BuildGraph.
func Parse(text string, buildArgs map[string]string) (*Result, error) {
	lines := logicalLines(text)
	res := &Result{
		Graph: &thrivefile.BuildGraph{
			Steps: map[string]*thrivefile.Step{},
			Edges: map[string][]string{},
		},
	}

	type stage struct {
		base  string
		steps []*thrivefile.Step
		entry []string
	}
	var stages []stage
	current := -1
	args := map[string]string{}
	for k, v := range buildArgs {
		args[k] = v
	}
	env := map[string]string{}
	workdir := ""

	newStage := func(base string) {
		stages = append(stages, stage{base: base})
		current++
		env = map[string]string{}
		workdir = ""
	}

	flushStep := func(s *thrivefile.Step) {
		if current < 0 {
			return
		}
		stages[current].steps = append(stages[current].steps, s)
	}

	for _, line := range lines {
		upper, rest := splitInstruction(line)
		switch upper {
		case "FROM":
			fields := strings.Fields(rest)
			if len(fields) == 0 {
				return nil, fmt.Errorf("dockerfile: FROM requires an image")
			}
			newStage(substitute(fields[0], args, env))
		case "ARG":
			name, def, _ := strings.Cut(rest, "=")
			name = strings.TrimSpace(name)
			if _, ok := args[name]; !ok && def != "" {
				args[name] = strings.TrimSpace(def)
			}
		case "RUN":
			if current < 0 {
				return nil, fmt.Errorf("dockerfile: RUN before any FROM")
			}
			body := strings.TrimSpace(rest)
			if strings.HasPrefix(body, "[") {
				var parts []string
				if err := json.Unmarshal([]byte(body), &parts); err == nil {
					body = shellJoin(parts)
				}
			}
			body = substitute(body, args, env)
			if workdir != "" {
				body = "cd " + shellQuote(workdir) + " && " + body
			}
			flushStep(&thrivefile.Step{Run: body})
		case "COPY", "ADD":
			if current < 0 {
				return nil, fmt.Errorf("dockerfile: COPY before any FROM")
			}
			if upper == "ADD" {
				res.Warnings = append(res.Warnings, "ADD treated as COPY (remote URLs and tar auto-extraction are not supported)")
			}
			srcs, dst := parseCopyRest(rest)
			if len(srcs) == 0 || dst == "" {
				return nil, fmt.Errorf("dockerfile: %s requires source and destination", upper)
			}
			var specs []thrivefile.CopySpec
			for _, src := range srcs {
				src = substitute(src, args, env)
				specs = append(specs, thrivefile.CopySpec{Source: src, Dest: substitute(dst, args, env)})
			}
			// The build engine materialises Copy specs (context mount + cp);
			// WORKDIR applies at copy time via the engine.
			step := &thrivefile.Step{Run: "", Copy: specs}
			if workdir != "" {
				for i := range step.Copy {
					if !filepath.IsAbs(step.Copy[i].Dest) {
						step.Copy[i].Dest = filepath.Join(workdir, step.Copy[i].Dest)
					}
				}
			}
			flushStep(step)
		case "ENV":
			for k, v := range parseEnvRest(rest) {
				env[k] = substitute(v, args, env)
			}
		case "WORKDIR":
			workdir = substitute(strings.TrimSpace(rest), args, env)
		case "CMD":
			if current < 0 {
				return nil, fmt.Errorf("dockerfile: CMD before any FROM")
			}
			stages[current].entry = parseStringList(rest, args, env)
		case "ENTRYPOINT":
			if current < 0 {
				return nil, fmt.Errorf("dockerfile: ENTRYPOINT before any FROM")
			}
			stages[current].entry = parseStringList(rest, args, env)
		case "EXPOSE", "LABEL", "VOLUME", "USER", "HEALTHCHECK", "STOPSIGNAL", "SHELL", "ONBUILD", "MAINTAINER":
			res.Warnings = append(res.Warnings, upper+" is accepted but has no effect on the build")
		default:
			return nil, fmt.Errorf("dockerfile: unknown instruction %q", upper)
		}
	}

	if len(stages) == 0 {
		return nil, fmt.Errorf("dockerfile: no FROM instruction found")
	}
	if len(stages) > 1 {
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("multi-stage build: only the final stage (base %s) is built; %d earlier stage(s) skipped",
				stages[len(stages)-1].base, len(stages)-1))
	}
	final := stages[len(stages)-1]
	res.Graph.BaseImage = final.base
	res.Graph.EntryCmd = final.entry
	prev := ""
	for i, s := range final.steps {
		name := fmt.Sprintf("step-%d", i)
		s.Name = name
		if prev != "" {
			s.DependsOn = []string{prev}
			res.Graph.Edges[name] = []string{prev}
		} else {
			res.Graph.Edges[name] = nil
		}
		res.Graph.Steps[name] = s
		prev = name
	}
	return res, nil
}

// logicalLines joins backslash continuations and drops comments/blanks.
func logicalLines(text string) []string {
	var out []string
	var pending string
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, " \t\r")
		if strings.HasSuffix(line, "\\") {
			pending += strings.TrimSuffix(line, "\\") + " "
			continue
		}
		line = pending + line
		pending = ""
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		out = append(out, trimmed)
	}
	if pending = strings.TrimSpace(pending); pending != "" && !strings.HasPrefix(pending, "#") {
		out = append(out, pending)
	}
	return out
}

// splitInstruction splits "RUN echo hi" into ("RUN", "echo hi").
func splitInstruction(line string) (string, string) {
	if i := strings.IndexAny(line, " \t"); i >= 0 {
		return strings.ToUpper(line[:i]), strings.TrimSpace(line[i+1:])
	}
	return strings.ToUpper(line), ""
}

// parseCopyRest splits COPY args into sources and destination, skipping flags.
func parseCopyRest(rest string) ([]string, string) {
	var fields []string
	for _, f := range strings.Fields(rest) {
		if strings.HasPrefix(f, "--") && len(fields) == 0 {
			continue
		}
		fields = append(fields, f)
	}
	if len(fields) < 2 {
		// Try JSON form: ["src", "dst"].
		var arr []string
		if err := json.Unmarshal([]byte(rest), &arr); err == nil && len(arr) >= 2 {
			return arr[:len(arr)-1], arr[len(arr)-1]
		}
		return nil, ""
	}
	return fields[:len(fields)-1], fields[len(fields)-1]
}

// parseEnvRest parses both ENV forms: "K=V K2=V2" and legacy "K V".
func parseEnvRest(rest string) map[string]string {
	out := map[string]string{}
	if strings.Contains(rest, "=") {
		for _, f := range strings.Fields(rest) {
			k, v, _ := strings.Cut(f, "=")
			out[k] = v
		}
		return out
	}
	fields := strings.Fields(rest)
	if len(fields) >= 2 {
		out[fields[0]] = strings.Join(fields[1:], " ")
	}
	return out
}

// parseStringList parses shell and exec-form CMD/ENTRYPOINT.
func parseStringList(rest string, args, env map[string]string) []string {
	rest = strings.TrimSpace(rest)
	if strings.HasPrefix(rest, "[") {
		var parts []string
		if err := json.Unmarshal([]byte(rest), &parts); err == nil {
			for i, p := range parts {
				parts[i] = substitute(p, args, env)
			}
			return parts
		}
	}
	return strings.Fields(substitute(rest, args, env))
}

// substitute expands $VAR, ${VAR}, ${VAR:-default}, ${VAR:+alt}.
func substitute(s string, args, env map[string]string) string {
	lookup := func(name string) (string, bool) {
		if v, ok := env[name]; ok {
			return v, true
		}
		v, ok := args[name]
		return v, ok
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '$' {
			b.WriteByte(s[i])
			i++
			continue
		}
		if i+1 < len(s) && s[i+1] == '$' {
			b.WriteByte('$')
			i += 2
			continue
		}
		if i+1 < len(s) && s[i+1] == '{' {
			end := strings.Index(s[i:], "}")
			if end < 0 {
				b.WriteString(s[i:])
				break
			}
			expr := s[i+2 : i+end]
			name := expr
			def := ""
			hasDefault := false
			alt := ""
			hasAlt := false
			if idx := strings.Index(expr, ":-"); idx >= 0 {
				name, def, hasDefault = expr[:idx], expr[idx+2:], true
			} else if idx := strings.Index(expr, ":+"); idx >= 0 {
				name, alt, hasAlt = expr[:idx], expr[idx+2:], true
			}
			val, ok := lookup(name)
			switch {
			case hasDefault && (!ok || val == ""):
				b.WriteString(def)
			case hasAlt && ok && val != "":
				b.WriteString(alt)
			case ok:
				b.WriteString(val)
			}
			i += end + 1
			continue
		}
		j := i + 1
		for j < len(s) && (s[j] == '_' || s[j] >= 'a' && s[j] <= 'z' || s[j] >= 'A' && s[j] <= 'Z' || s[j] >= '0' && s[j] <= '9') {
			j++
		}
		if j == i+1 {
			b.WriteByte('$')
			i++
			continue
		}
		if val, ok := lookup(s[i+1 : j]); ok {
			b.WriteString(val)
		}
		i = j
	}
	return b.String()
}

func shellJoin(parts []string) string {
	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = shellQuote(p)
	}
	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
			c == '_' || c == '-' || c == '.' || c == '/' || c == ':' || c == '=') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// ContextDirFor returns the absolute build context directory for a build path.
func ContextDirFor(path string) string {
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		abs, err := filepath.Abs(path)
		if err == nil {
			return abs
		}
		return path
	}
	abs, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return "."
	}
	return abs
}
