package platformmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/sourceplane/orun/internal/remotestate"
)

// The task-plane seam on the fake: typed fixtures (the CLI-style clients
// return structs, not pages), each call recorded like every other.

type taskFixtures struct {
	tasks     []remotestate.PublicTask
	task      *remotestate.PublicTask
	verdict   *remotestate.TaskVerdictView
	contract  *remotestate.TaskContractView
	epic      *remotestate.EpicView
	milestone *remotestate.MilestoneView
	container *remotestate.ContainerContractView
	docs      []remotestate.EpicDoc
	docBody   string
	// createErr, when set, fails the container/task creates (the 409 path).
	createErr error
	notFound  bool
}

func notFoundErr() error {
	return &remotestate.APIError{Status: http.StatusNotFound, Code: "not_found", Message: "Not found"}
}

func (f *fakeAPI) fx() *taskFixtures {
	if f.tasks == nil {
		f.tasks = &taskFixtures{}
	}
	return f.tasks
}

func (f *fakeAPI) ListTasksWhere(_ context.Context, org string, filter remotestate.TaskListFilter) (*remotestate.TasksList, error) {
	f.calls = append(f.calls, fmt.Sprintf("ListTasksWhere org=%s epic=%s milestone=%s assignee=%s", org, filter.Epic, filter.Milestone, filter.Assignee))
	if f.err != nil {
		return nil, f.err
	}
	return &remotestate.TasksList{Tasks: f.fx().tasks}, nil
}
func (f *fakeAPI) GetTask(_ context.Context, org, ref string) (*remotestate.PublicTask, error) {
	f.calls = append(f.calls, fmt.Sprintf("GetTask org=%s ref=%s", org, ref))
	if f.fx().notFound || f.fx().task == nil {
		return nil, notFoundErr()
	}
	return f.fx().task, nil
}
func (f *fakeAPI) GetTaskContract(_ context.Context, org, ref string) (*remotestate.TaskContractView, error) {
	f.calls = append(f.calls, fmt.Sprintf("GetTaskContract org=%s ref=%s", org, ref))
	return f.fx().contract, nil
}
func (f *fakeAPI) GetTaskVerdict(_ context.Context, org, ref string) (*remotestate.TaskVerdictView, error) {
	f.calls = append(f.calls, fmt.Sprintf("GetTaskVerdict org=%s ref=%s", org, ref))
	return f.fx().verdict, nil
}
func (f *fakeAPI) GetEpic(_ context.Context, org, ref string) (*remotestate.EpicView, error) {
	f.calls = append(f.calls, fmt.Sprintf("GetEpic org=%s ref=%s", org, ref))
	if f.fx().notFound || f.fx().epic == nil {
		return nil, notFoundErr()
	}
	return f.fx().epic, nil
}
func (f *fakeAPI) GetMilestone(_ context.Context, org, ref string) (*remotestate.MilestoneView, error) {
	f.calls = append(f.calls, fmt.Sprintf("GetMilestone org=%s ref=%s", org, ref))
	if f.fx().notFound || f.fx().milestone == nil {
		return nil, notFoundErr()
	}
	return f.fx().milestone, nil
}
func (f *fakeAPI) GetContainerContract(_ context.Context, org, container, ref string) (*remotestate.ContainerContractView, error) {
	f.calls = append(f.calls, fmt.Sprintf("GetContainerContract org=%s %s=%s", org, container, ref))
	if f.fx().container == nil {
		return nil, notFoundErr()
	}
	return f.fx().container, nil
}
func (f *fakeAPI) ListEpicDocs(_ context.Context, org, ref string) (*remotestate.EpicDocsList, error) {
	f.calls = append(f.calls, fmt.Sprintf("ListEpicDocs org=%s epic=%s", org, ref))
	return &remotestate.EpicDocsList{Docs: f.fx().docs}, nil
}
func (f *fakeAPI) GetEpicDoc(_ context.Context, org, ref, slug string) (*remotestate.EpicDocView, error) {
	f.calls = append(f.calls, fmt.Sprintf("GetEpicDoc org=%s epic=%s slug=%s", org, ref, slug))
	return &remotestate.EpicDocView{Content: f.fx().docBody}, nil
}
func (f *fakeAPI) CreateTaskWithKey(_ context.Context, org string, req remotestate.TaskCreateRequest, key string) (*remotestate.PublicTask, error) {
	f.keys = append(f.keys, key)
	f.calls = append(f.calls, fmt.Sprintf("CreateTask org=%s body=%s", org, bodyJSON(req)))
	if f.fx().createErr != nil {
		return nil, f.fx().createErr
	}
	t := &remotestate.PublicTask{ID: "tsk_3KF9TQ2P", Key: "BASE-3", KeyOrigin: "derived"}
	if req.AdoptKey != "" {
		t.Key, t.KeyOrigin = req.AdoptKey, "adopted"
	}
	if req.Milestone != "" {
		t.Milestone = &remotestate.TaskMilestoneRef{ID: req.Milestone, Name: "03 — infrastructure"}
	}
	if req.Epic != "" {
		t.Epic = &remotestate.TaskEpicRef{ID: "epc_AB12CD34", Slug: req.Epic, Key: "EP-1"}
	}
	return t, nil
}
func (f *fakeAPI) AttachTaskContractWithKey(_ context.Context, org, ref string, wire json.RawMessage, hash, key string) (*remotestate.TaskContractSeal, error) {
	f.keys = append(f.keys, key)
	f.calls = append(f.calls, fmt.Sprintf("AttachTaskContract org=%s ref=%s wire=%s hash=%s", org, ref, wire, hash[:13]))
	return &remotestate.TaskContractSeal{ContractHash: hash, SyncedAt: "2026-09-01T00:00:00Z"}, nil
}
func (f *fakeAPI) CreateEpicWithKey(_ context.Context, org string, req remotestate.EpicCreateRequest, key string) (*remotestate.PublicEpic, error) {
	f.keys = append(f.keys, key)
	f.calls = append(f.calls, fmt.Sprintf("CreateEpic org=%s body=%s", org, bodyJSON(req)))
	if f.fx().createErr != nil {
		return nil, f.fx().createErr
	}
	return &remotestate.PublicEpic{ID: "epc_AB12CD34", Slug: orID(req.Slug, "minted-slug"), Key: "EP-1", Name: req.Name}, nil
}
func (f *fakeAPI) CreateMilestoneWithKey(_ context.Context, org, epicRef string, req remotestate.MilestoneCreateRequest, key string) (*remotestate.PublicMilestone, error) {
	f.keys = append(f.keys, key)
	f.calls = append(f.calls, fmt.Sprintf("CreateMilestone org=%s epic=%s body=%s", org, epicRef, bodyJSON(req)))
	if f.fx().createErr != nil {
		return nil, f.fx().createErr
	}
	return &remotestate.PublicMilestone{ID: "mls_EF56GH78", EpicID: "epc_AB12CD34", Name: req.Name}, nil
}

func taskAPI(fx *taskFixtures) *fakeAPI {
	return &fakeAPI{page: page(`{"secrets":[{"secretKey":"STRIPE_TEST_KEY"},{"secretKey":"STRIPE_LIVE_KEY"},{"secretKey":"DB_URL"}]}`, ""), tasks: fx}
}

func TestTaskListSummary(t *testing.T) {
	api := taskAPI(&taskFixtures{tasks: []remotestate.PublicTask{{Key: "BASE-1", ContractHash: "sha256:aa"}, {Key: "BASE-2"}}})
	p := granted(&Provider{API: api}, "ws_1")
	text, isErr := callTool(t, p, "task_list", `{"workspace":"ws_1","epic":"infra-baselining","assignee":"me"}`)
	if isErr || !strings.HasPrefix(text, "2 task(s) under epic infra-baselining assigned to me, 1 with a contract attached\n") {
		t.Fatalf("task_list: %v %s", isErr, text)
	}
	if api.calls[0] != "ListTasksWhere org=ws_1 epic=infra-baselining milestone= assignee=me" {
		t.Fatalf("calls = %v", api.calls)
	}
}

func TestTaskGetWidenedRefs(t *testing.T) {
	verdict := &remotestate.TaskVerdictView{}
	verdict.Verdict.Rung = "in_review"
	verdict.Verdict.Evidence.Reason = "pull request open"
	verdict.Verdict.Blocked = true
	verdict.Verdict.BlockedBy = []string{"BASE-2"}
	epic := &remotestate.EpicView{Epic: remotestate.PublicEpic{Slug: "infra-baselining", State: "Planning"}, Rollup: &remotestate.EpicRollup{Total: 9, Done: 4}}
	epic.Rollup.Governance.Own, epic.Rollup.Governance.Inherited, epic.Rollup.Governance.None = 2, 6, 1
	fx := &taskFixtures{
		task:      &remotestate.PublicTask{ID: "tsk_3KF9TQ2P", Key: "BASE-3", ContractHash: "sha256:aa"},
		verdict:   verdict,
		contract:  &remotestate.TaskContractView{ContractHash: "sha256:aa", Contract: json.RawMessage(`{"goal":"g"}`)},
		epic:      epic,
		milestone: &remotestate.MilestoneView{Milestone: remotestate.PublicMilestone{ID: "mls_EF56GH78", Name: "03 — infrastructure"}, Epic: &remotestate.PublicEpic{Slug: "infra-baselining"}},
		container: &remotestate.ContainerContractView{Contract: json.RawMessage(`{"envs":["stage"]}`)},
		docs:      []remotestate.EpicDoc{{Slug: "readme", SizeBytes: 12}, {Slug: "big", SizeBytes: 20000}},
		docBody:   "# hello",
	}
	cases := []struct{ ref, summary, callFrag string }{
		{"BASE-3", "BASE-3: in_review — pull request open (blocked by BASE-2)", "GetTask org=ws_1 ref=BASE-3"},
		{"tsk_3KF9TQ2P", "BASE-3: in_review", "GetTaskContract org=ws_1 ref=tsk_3KF9TQ2P"},
		{"mls_EF56GH78", "milestone 03 — infrastructure in epic infra-baselining — carries a container contract", "GetContainerContract org=ws_1 milestones=mls_EF56GH78"},
		{"infra-baselining", "epic infra-baselining: orun: Planning · orun: 4/9 done · governance own 2 / inherited 6 / none 1 · 2 spec(s) (large ones by pointer)", "GetEpicDoc org=ws_1 epic=infra-baselining slug=readme"},
		{"EP-1", "epic infra-baselining:", "GetEpic org=ws_1 ref=EP-1"},
	}
	for _, tc := range cases {
		api := taskAPI(fx)
		p := granted(&Provider{API: api}, "ws_1")
		text, isErr := callTool(t, p, "task_get", fmt.Sprintf(`{"workspace":"ws_1","task":%q}`, tc.ref))
		if isErr || !strings.HasPrefix(text, tc.summary) {
			t.Errorf("task_get %s: isErr=%v\n%s", tc.ref, isErr, text)
		}
		if !strings.Contains(strings.Join(api.calls, ";"), tc.callFrag) {
			t.Errorf("task_get %s: calls = %v", tc.ref, api.calls)
		}
	}
	// The big doc travels by pointer (content null), the small one inline.
	api := taskAPI(fx)
	text, _ := callTool(t, granted(&Provider{API: api}, "ws_1"), "task_get", `{"workspace":"ws_1","task":"infra-baselining"}`)
	body := text[strings.Index(text, "\n")+1:]
	var data struct {
		Specs []struct {
			Slug    string  `json:"slug"`
			Content *string `json:"content"`
		} `json:"specs"`
	}
	if err := json.Unmarshal([]byte(body), &data); err != nil || len(data.Specs) != 2 {
		t.Fatalf("specs: %v %s", err, body)
	}
	if data.Specs[0].Content == nil || *data.Specs[0].Content != "# hello" || data.Specs[1].Content != nil {
		t.Fatalf("inline/pointer split wrong: %+v", data.Specs)
	}

	// Unknown refs are answers, not errors.
	for ref, want := range map[string]string{"mls_NOPE": "no milestone mls_NOPE here", "nothing-here": "nothing named nothing-here — not an epic slug", "ZZ-99": "no task ZZ-99 here"} {
		api := taskAPI(&taskFixtures{notFound: true})
		text, isErr := callTool(t, granted(&Provider{API: api}, "ws_1"), "task_get", fmt.Sprintf(`{"workspace":"ws_1","task":%q}`, ref))
		if isErr || !strings.HasPrefix(text, want) {
			t.Errorf("task_get %s: %v %s", ref, isErr, text)
		}
	}
}

func TestPolicyPreviewIntersection(t *testing.T) {
	api := taskAPI(&taskFixtures{})
	p := granted(&Provider{API: api}, "ws_1")
	text, isErr := callTool(t, p, "policy_preview", `{"workspace":"ws_1","project":"prj_a","environment":"stage","contractSecrets":["STRIPE_TEST_*"],"contractEnvs":["stage","dev"]}`)
	if isErr || !strings.HasPrefix(text, "2 of 3 secret(s) would stop resolving in stage; 1 unaffected\n") {
		t.Fatalf("policy_preview: %v %s", isErr, text)
	}
	if !strings.Contains(text, `{"key":"STRIPE_LIVE_KEY","deniedBy":"secrets"}`) || !strings.Contains(text, `"stillResolving":["STRIPE_TEST_KEY"]`) {
		t.Fatalf("payload: %s", text)
	}
	// The env gate denies the whole environment first.
	text, _ = callTool(t, p, "policy_preview", `{"workspace":"ws_1","project":"prj_a","environment":"prod","contractSecrets":["*"],"contractEnvs":["stage"]}`)
	if !strings.HasPrefix(text, "3 of 3 secret(s) would stop resolving in prod; 0 unaffected") || !strings.Contains(text, `"deniedBy":"envs"`) {
		t.Fatalf("env gate: %s", text)
	}
	// No narrowing fields: nothing narrows.
	text, _ = callTool(t, p, "policy_preview", `{"workspace":"ws_1","project":"prj_a","environment":"stage"}`)
	if !strings.HasPrefix(text, "no narrowing fields given — all 3 secret(s) keep resolving") {
		t.Fatalf("no narrowing: %s", text)
	}
	if api.calls[0] != "ListSecretsMetadata org=ws_1 project=prj_a env=stage" {
		t.Fatalf("calls = %v", api.calls)
	}
}

func TestGlobMatchMirrorsPolicyEngine(t *testing.T) {
	for _, tc := range []struct {
		pattern, value string
		want           bool
	}{
		{"*", "anything", true}, {"STRIPE_TEST_*", "STRIPE_TEST_KEY", true}, {"STRIPE_TEST_*", "STRIPE_LIVE_KEY", false},
		{"a.b", "a.b", true}, {"a.b", "axb", false}, {"*_KEY", "DB_KEY", true}, {"stage", "staging", false},
	} {
		if got := globMatch(tc.pattern, tc.value); got != tc.want {
			t.Errorf("globMatch(%q,%q) = %v", tc.pattern, tc.value, got)
		}
	}
}

func TestTaskCreateBornContracted(t *testing.T) {
	api := taskAPI(&taskFixtures{})
	p := granted(&Provider{API: api}, "ws_1")
	text, isErr := callTool(t, p, "task_create", `{"workspace":"ws_1","mintPrefix":"BASE","titleMirror":"phase(03-infrastructure)","brief":"Land the data plane.","epic":"infra-baselining","milestone":"mls_EF56GH78","assignee":"me","idempotencyKey":"bt-03","contract":{"goal":"data plane live","affects":["infra"],"doneWhen":["WIRING_* published"],"gates":[]}}`)
	if isErr {
		t.Fatalf("task_create: %s", text)
	}
	if !strings.HasPrefix(text, "task BASE-3 created (derived in milestone 03 — infrastructure; contract attached (merge alone finishes it); branch grammar: orun/BASE-3-<slug>)\n") {
		t.Fatalf("summary: %s", text)
	}
	if api.calls[0] != `CreateTask org=ws_1 body={"mintPrefix":"BASE","titleMirror":"phase(03-infrastructure)","brief":"Land the data plane.","epic":"infra-baselining","milestone":"mls_EF56GH78","assignee":"me"}` {
		t.Fatalf("create call = %s", api.calls[0])
	}
	// gatesDefined rides explicitly; the attach shares the attempt.
	if !strings.Contains(api.calls[1], `"gatesDefined":true`) || !strings.Contains(api.calls[1], "ref=tsk_3KF9TQ2P") {
		t.Fatalf("attach call = %s", api.calls[1])
	}
	if api.keys[0] != "bt-03" || api.keys[1] != "bt-03:contract" {
		t.Fatalf("keys = %v", api.keys)
	}
	// A contract missing its gates is refused before the allocator is asked.
	api = taskAPI(&taskFixtures{})
	text, isErr = callTool(t, granted(&Provider{API: api}, "ws_1"), "task_create", `{"workspace":"ws_1","contract":{"goal":"g","affects":["a"],"doneWhen":["d"]}}`)
	if !isErr || len(api.calls) != 0 || !strings.Contains(text, "gates") {
		t.Fatalf("gateless contract: %v %v %s", isErr, api.calls, text)
	}
	// Without a contract: nothing attached, and the summary says only where.
	api = taskAPI(&taskFixtures{})
	text, _ = callTool(t, granted(&Provider{API: api}, "ws_1"), "task_create", `{"workspace":"ws_1","adoptKey":"ENG-7","epic":"infra-baselining"}`)
	if !strings.HasPrefix(text, "task ENG-7 created (adopted under epic infra-baselining; branch grammar: orun/ENG-7-<slug>)") || len(api.calls) != 1 {
		t.Fatalf("plain create: %s %v", text, api.calls)
	}
	checkAutoKey(t, api.keys[0])
}

func TestEpicCreateAdoptsTakenSlug(t *testing.T) {
	api := taskAPI(&taskFixtures{})
	p := granted(&Provider{API: api}, "ws_1")
	text, isErr := callTool(t, p, "epic_create", `{"workspace":"ws_1","name":"Infra baselining","slug":"infra-baselining","owner":"me"}`)
	if isErr || !strings.HasPrefix(text, "epic infra-baselining created (EP-1); club tasks with task_create epic=infra-baselining\n") || !strings.Contains(text, `"existed":false`) {
		t.Fatalf("epic_create: %v %s", isErr, text)
	}
	taken := &remotestate.APIError{Status: http.StatusConflict, Code: "conflict", Message: "slug is already taken", Details: json.RawMessage(`{"existing":{"id":"epc_AB12CD34","slug":"infra-baselining","key":"EP-1"}}`)}
	api = taskAPI(&taskFixtures{createErr: taken})
	text, isErr = callTool(t, granted(&Provider{API: api}, "ws_1"), "epic_create", `{"workspace":"ws_1","name":"Infra baselining","slug":"infra-baselining"}`)
	if isErr || !strings.HasPrefix(text, "epic infra-baselining already exists (EP-1) — reusing it\n") || !strings.Contains(text, `"existed":true`) {
		t.Fatalf("adopt: %v %s", isErr, text)
	}
	other := &remotestate.APIError{Status: http.StatusConflict, Code: "conflict", Message: "something else"}
	api = taskAPI(&taskFixtures{createErr: other})
	if text, isErr := callTool(t, granted(&Provider{API: api}, "ws_1"), "epic_create", `{"workspace":"ws_1","name":"X"}`); !isErr {
		t.Fatalf("a conflict without a holder must stay an error: %s", text)
	}
}

func TestMilestoneCreatePositions(t *testing.T) {
	for args, wantBody := range map[string]string{
		`{"workspace":"ws_1","epic":"infra-baselining","name":"03 — infrastructure","exitCriteria":["WIRING_* published"],"after":"mls_02FOUND1"}`: `{"after":"mls_02FOUND1","exitCriteria":["WIRING_* published"],"name":"03 — infrastructure"}`,
		`{"workspace":"ws_1","epic":"infra-baselining","name":"01 — scaffold","after":null}`:                                                       `{"after":null,"name":"01 — scaffold"}`,
		`{"workspace":"ws_1","epic":"infra-baselining","name":"08 — docs"}`:                                                                        `{"name":"08 — docs"}`,
	} {
		api := taskAPI(&taskFixtures{})
		text, isErr := callTool(t, granted(&Provider{API: api}, "ws_1"), "milestone_create", args)
		if isErr || !strings.Contains(text, "(mls_EF56GH78) added to epic infra-baselining; place tasks with task_create milestone=mls_EF56GH78") {
			t.Fatalf("milestone_create: %v %s", isErr, text)
		}
		if want := "CreateMilestone org=ws_1 epic=infra-baselining body=" + wantBody; api.calls[0] != want {
			t.Errorf("call = %s\nwant %s", api.calls[0], want)
		}
		checkAutoKey(t, api.keys[0])
	}
}

func TestTaskWritesBlockedReadOnly(t *testing.T) {
	api := taskAPI(&taskFixtures{})
	p := granted(&Provider{API: api, ReadOnly: true}, "ws_1")
	for _, tool := range []string{"task_create", "epic_create", "milestone_create"} {
		if text, isErr := callTool(t, p, tool, `{"workspace":"ws_1","name":"x","epic":"e"}`); !isErr || !strings.Contains(text, "--read-only") {
			t.Errorf("%s under read-only: %v %s", tool, isErr, text)
		}
	}
	if len(api.calls) != 0 {
		t.Fatalf("read-only server reached the seam: %v", api.calls)
	}
}
