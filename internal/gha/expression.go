package gha

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	actexpr "github.com/nektos/act/pkg/exprparser"
	actmodel "github.com/nektos/act/pkg/model"
	"github.com/rhysd/actionlint"
)

type EvaluationInput struct {
	Github    *actmodel.GithubContext
	Env       map[string]string
	JobStatus string
	Steps     map[string]*actmodel.StepResult
	Runner    map[string]interface{}
	Secrets   map[string]string
	Vars      map[string]string
	Inputs    map[string]interface{}
	Needs     map[string]actexpr.Needs
}

type Evaluator struct {
	interpreter actexpr.Interpreter
}

func NewEvaluator(input EvaluationInput) *Evaluator {
	environment := &actexpr.EvaluationEnvironment{
		Github: input.Github,
		Env:    input.Env,
		Job: &actmodel.JobContext{
			Status: input.JobStatus,
		},
		Steps:   input.Steps,
		Runner:  input.Runner,
		Secrets: input.Secrets,
		Vars:    input.Vars,
		Inputs:  input.Inputs,
		Needs:   input.Needs,
	}
	return &Evaluator{
		interpreter: actexpr.NewInterpeter(environment, actexpr.Config{Context: "step"}),
	}
}

func (e *Evaluator) Evaluate(input string, defaultStatusCheck actexpr.DefaultStatusCheck) (interface{}, error) {
	if e == nil {
		return nil, fmt.Errorf("expression evaluator is nil")
	}
	return e.interpreter.Evaluate(input, defaultStatusCheck)
}

func (e *Evaluator) EvalBool(input string, defaultStatusCheck actexpr.DefaultStatusCheck) (bool, error) {
	rewritten, err := rewriteSubExpression(input, false)
	if err != nil {
		return false, err
	}
	value, err := e.Evaluate(rewritten, defaultStatusCheck)
	if err != nil {
		return false, err
	}
	return actexpr.IsTruthy(value), nil
}

func (e *Evaluator) Interpolate(input string) string {
	if !strings.Contains(input, "${{") || !strings.Contains(input, "}}") {
		return input
	}

	rewritten, err := rewriteSubExpression(input, true)
	if err != nil {
		return ""
	}

	value, err := e.Evaluate(rewritten, actexpr.DefaultStatusCheckNone)
	if err != nil {
		return ""
	}
	return stringifyValue(value)
}

func stringifyValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(typed)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float32:
		return stringifyFloat(float64(typed))
	case float64:
		return stringifyFloat(typed)
	default:
		return fmt.Sprint(value)
	}
}

func stringifyFloat(value float64) string {
	if math.IsInf(value, 1) {
		return "Infinity"
	}
	if math.IsInf(value, -1) {
		return "-Infinity"
	}
	if math.IsNaN(value) {
		return "NaN"
	}
	if value == 0 {
		return "0"
	}
	return strconv.FormatFloat(value, 'G', 15, 64)
}

func rewriteSubExpression(input string, forceFormat bool) (string, error) {
	if !strings.Contains(input, "${{") || !strings.Contains(input, "}}") {
		return input, nil
	}

	trimmed := strings.TrimSpace(input)
	if !forceFormat && strings.HasPrefix(trimmed, "${{") && strings.HasSuffix(trimmed, "}}") {
		inner := strings.TrimSpace(trimmed[3 : len(trimmed)-2])
		if _, err := actionlint.NewExprParser().Parse(actionlint.NewExprLexer(inner + "}}")); err == nil {
			return inner, nil
		}
	}

	remaining := input
	results := make([]string, 0)
	var formatOut strings.Builder
	for {
		start := strings.Index(remaining, "${{")
		if start == -1 {
			formatOut.WriteString(escapeFormatString(remaining))
			break
		}

		formatOut.WriteString(escapeFormatString(remaining[:start]))
		afterStart := remaining[start+3:]
		lexer := actionlint.NewExprLexer(afterStart)
		if _, err := actionlint.NewExprParser().Parse(lexer); err != nil {
			return "", fmt.Errorf("parse expression: %s", err.Message)
		}

		consumed := lexer.Offset()
		if consumed > len(afterStart) {
			return "", fmt.Errorf("parsed expression exceeded input length")
		}

		exprText := afterStart[:consumed]
		rest := afterStart[consumed:]
		switch {
		case strings.HasPrefix(rest, "}}"):
			rest = rest[2:]
		case strings.HasSuffix(exprText, "}}"):
			exprText = strings.TrimSuffix(exprText, "}}")
		default:
			return "", fmt.Errorf("expression is missing closing braces")
		}

		results = append(results, strings.TrimSpace(exprText))
		formatOut.WriteString(fmt.Sprintf("{%d}", len(results)-1))
		remaining = rest
	}

	if len(results) == 1 && formatOut.String() == "{0}" && !forceFormat {
		return results[0], nil
	}

	return fmt.Sprintf("format('%s', %s)", strings.ReplaceAll(formatOut.String(), "'", "''"), strings.Join(results, ", ")), nil
}

func escapeFormatString(input string) string {
	return strings.ReplaceAll(strings.ReplaceAll(input, "{", "{{"), "}", "}}")
}
