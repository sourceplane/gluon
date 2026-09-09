package platformmcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/sourceplane/orun/internal/contract"
	"github.com/sourceplane/orun/internal/remotestate"
)

// The task plane's six tools (orun-tasks M1/M2 + BT0, served natively here
// at orun-baseline-tracking BT-O3): three reads — task_list, task_get (the
// consolidated read: contract + derived verdict + evidence in ONE call, and
// the widened ref that answers for epics and milestones too), policy_preview
// (a contract's blast radius BEFORE it is attached) — and three writes —
// task_create (born clubbed, briefed, assigned and contracted), epic_create,
// milestone_create. Summaries mirror the TS plane's (orun-cloud
// packages/mcp/src/tools/tasks.ts) so prompts and docs stay portable.

var (
	taskKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,5}-[A-Z]?[0-9]+$`)
	epicKeyRe = regexp.MustCompile(`^EP-[0-9]+$`)
)

// Inline budgets for an epic's spec docs on task_get (epics-and-docs.md §6):
// content rides inline while it fits; a larger doc travels as its pointer.
const (
	inlineDocBytes   = 16 * 1024
	inlineDocsBudget = 48 * 1024
)

func (p *Provider) callTaskRead(ctx context.Context, name string, a argmap) (string, error) {
	ws := a.str("workspace")
	switch name {
	case "task_list":
		filter := remotestate.TaskListFilter{Epic: a.str("epic"), Milestone: a.str("milestone"), Assignee: a.str("assignee")}
		list, err := p.API.ListTasksWhere(ctx, ws, filter)
		if err != nil {
			return "", err
		}
		contracted := 0
		for _, t := range list.Tasks {
			if t.ContractHash != "" {
				contracted++
			}
		}
		scope := ""
		switch {
		case filter.Milestone != "":
			scope = " in milestone " + filter.Milestone
		case filter.Epic != "":
			scope = " under epic " + filter.Epic
		}
		who := ""
		if filter.Assignee != "" {
			who = " assigned to " + filter.Assignee
		}
		return emit(fmt.Sprintf("%d task(s)%s%s, %d with a contract attached", len(list.Tasks), scope, who, contracted),
			map[string]interface{}{"tasks": list.Tasks})

	case "task_get":
		if err := requireStr(a, name, "task"); err != nil {
			return "", err
		}
		return p.taskGet(ctx, ws, a.str("task"))

	case "policy_preview":
		return p.policyPreview(ctx, a, ws)
	}
	return "", fmt.Errorf("unknown tool %s", name)
}

// taskGet is the widened read: mls_ → milestone; epc_ / EP-n / anything
// that is neither a task key nor a tsk_ id → epic (the slug is the epic's
// human handle); else the task with its contract, verdict and evidence.
func (p *Provider) taskGet(ctx context.Context, ws, ref string) (string, error) {
	if strings.HasPrefix(ref, "mls_") {
		got, err := p.API.GetMilestone(ctx, ws, ref)
		if err != nil {
			if remotestate.IsNotFound(err) {
				return emit(fmt.Sprintf("no milestone %s here", ref), map[string]interface{}{"milestone": nil})
			}
			return "", err
		}
		cc, err := p.containerContract(ctx, ws, "milestones", ref)
		if err != nil {
			return "", err
		}
		label := got.Milestone.Name
		if label == "" {
			label = got.Milestone.ID
		}
		summary := "milestone " + label
		if got.Epic != nil {
			summary += " in epic " + got.Epic.Slug
		}
		if cc != nil {
			summary += " — carries a container contract"
		}
		return emit(summary, map[string]interface{}{"milestone": got.Milestone, "epic": got.Epic, "contract": cc})
	}

	looksLikeEpic := strings.HasPrefix(ref, "epc_") || epicKeyRe.MatchString(ref) ||
		(!strings.HasPrefix(ref, "tsk_") && !taskKeyRe.MatchString(ref))
	if looksLikeEpic {
		view, err := p.API.GetEpic(ctx, ws, ref)
		if err != nil {
			if remotestate.IsNotFound(err) {
				return emit(fmt.Sprintf("nothing named %s — not an epic slug, epc_/mls_/tsk_ id, or task key here", ref),
					map[string]interface{}{"epic": nil})
			}
			return "", err
		}
		cc, err := p.containerContract(ctx, ws, "epics", ref)
		if err != nil {
			return "", err
		}
		specs, byPointer, err := p.epicSpecs(ctx, ws, ref)
		if err != nil {
			return "", err
		}
		e := view.Epic
		voice := e.Provider
		if voice == "" {
			voice = "orun"
		}
		observed := e.State
		if observed == "" {
			observed = "no tracker state"
		}
		rollup := view.Rollup
		if rollup == nil {
			rollup = &remotestate.EpicRollup{}
		}
		g := rollup.Governance
		summary := fmt.Sprintf("epic %s: %s: %s · orun: %d/%d done · governance own %d / inherited %d / none %d",
			e.Slug, voice, observed, rollup.Done, rollup.Total, g.Own, g.Inherited, g.None)
		if len(specs) > 0 {
			summary += fmt.Sprintf(" · %d spec(s)", len(specs))
			if byPointer {
				summary += " (large ones by pointer)"
			}
		}
		return emit(summary, map[string]interface{}{"epic": e, "rollup": rollup, "contract": cc, "specs": specs})
	}

	task, err := p.API.GetTask(ctx, ws, ref)
	if err != nil {
		if remotestate.IsNotFound(err) {
			return emit(fmt.Sprintf("no task %s here — pass a key (ENG-42) or a tsk_… id", ref), map[string]interface{}{"task": nil})
		}
		return "", err
	}
	verdict, err := p.API.GetTaskVerdict(ctx, ws, task.ID)
	if err != nil {
		return "", err
	}
	var cont interface{}
	if task.ContractHash != "" {
		view, err := p.API.GetTaskContract(ctx, ws, task.ID)
		if err != nil {
			return "", err
		}
		cont = view.Contract
	}
	v := verdict.Verdict
	summary := fmt.Sprintf("%s: %s — %s", task.Key, v.Rung, v.Evidence.Reason)
	if v.Blocked {
		summary += " (blocked by " + strings.Join(v.BlockedBy, ", ") + ")"
	}
	return emit(summary, map[string]interface{}{
		"task": task, "contract": cont, "verdict": v,
		"dependencies": verdict.Dependencies, "observations": verdict.Observations,
	})
}

// containerContract reads the E3/E4 governing document on a container, or
// nil when there is none — a 404 here is an answer ("ungoverned"), never an
// error.
func (p *Provider) containerContract(ctx context.Context, ws, container, ref string) (interface{}, error) {
	view, err := p.API.GetContainerContract(ctx, ws, container, ref)
	if err != nil {
		if remotestate.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return view.Contract, nil
}

// epicSpec is one spec doc as task_get renders it: the pointer, and the
// content when it fit the inline budget (nil = by pointer).
type epicSpec struct {
	Slug      string  `json:"slug"`
	Title     string  `json:"title"`
	Repo      string  `json:"repo"`
	Path      string  `json:"path"`
	Sha       string  `json:"sha"`
	SizeBytes int     `json:"sizeBytes"`
	PushedAt  string  `json:"pushedAt"`
	Content   *string `json:"content"`
}

func (p *Provider) epicSpecs(ctx context.Context, ws, ref string) ([]epicSpec, bool, error) {
	listed, err := p.API.ListEpicDocs(ctx, ws, ref)
	if err != nil {
		if remotestate.IsNotFound(err) {
			return []epicSpec{}, false, nil
		}
		return nil, false, err
	}
	specs := make([]epicSpec, 0, len(listed.Docs))
	budget := inlineDocsBudget
	byPointer := false
	for _, d := range listed.Docs {
		spec := epicSpec{Slug: d.Slug, Title: d.Title, Repo: d.Repo, Path: d.Path, Sha: d.GitSha, SizeBytes: d.SizeBytes, PushedAt: d.PushedAt}
		if d.SizeBytes <= inlineDocBytes && d.SizeBytes <= budget {
			view, err := p.API.GetEpicDoc(ctx, ws, ref, d.Slug)
			if err != nil {
				return nil, false, err
			}
			content := view.Content
			spec.Content = &content
			budget -= d.SizeBytes
		} else {
			byPointer = true
		}
		specs = append(specs, spec)
	}
	return specs, byPointer, nil
}

// policyPreview evaluates a draft contract's `secrets` globs and `envs`
// list against the secret keys visible at one project environment, with
// the exact narrow-only semantics enforcement uses (effective = policy ∩
// contract), in the same order: the env gate first (it denies the whole
// environment at once), then the key globs.
func (p *Provider) policyPreview(ctx context.Context, a argmap, ws string) (string, error) {
	if err := requireStr(a, "policy_preview", "project", "environment"); err != nil {
		return "", err
	}
	env := a.str("environment")
	page, err := p.API.ListSecretsMetadata(ctx, remotestate.ConfigScope{Org: ws, Project: a.str("project"), Environment: env})
	if err != nil {
		return "", err
	}
	keys := secretKeysOf(page)
	secretsGlobs, secretsGiven := a.optList("contractSecrets")
	envsGlobs, envsGiven := a.optList("contractEnvs")

	envDenied := envsGiven && !anyGlob(envsGlobs, env)
	type denial struct {
		Key      string `json:"key"`
		DeniedBy string `json:"deniedBy"`
	}
	wouldDeny := []denial{}
	still := []string{}
	for _, k := range keys {
		switch {
		case envDenied:
			wouldDeny = append(wouldDeny, denial{k, "envs"})
		case secretsGiven && !anyGlob(secretsGlobs, k):
			wouldDeny = append(wouldDeny, denial{k, "secrets"})
		default:
			still = append(still, k)
		}
	}
	narrowing := secretsGiven || envsGiven
	data := map[string]interface{}{"environment": env, "narrowing": narrowing, "wouldDeny": wouldDeny, "stillResolving": still}
	if narrowing {
		return emit(fmt.Sprintf("%d of %d secret(s) would stop resolving in %s; %d unaffected", len(wouldDeny), len(keys), env, len(still)), data)
	}
	return emit(fmt.Sprintf("no narrowing fields given — all %d secret(s) keep resolving (a contract without secrets/envs does not narrow)", len(keys)), data)
}

// secretKeysOf reads the secret keys off a secrets-metadata page: rows under
// "secrets", "items", or the array itself, each keyed by secretKey (or key).
func secretKeysOf(page *remotestate.PlatformPage) []string {
	if page == nil || len(page.Data) == 0 {
		return nil
	}
	var rows []map[string]interface{}
	if json.Unmarshal(page.Data, &rows) != nil {
		var obj map[string]json.RawMessage
		if json.Unmarshal(page.Data, &obj) != nil {
			return nil
		}
		for _, k := range []string{"secrets", "items", "data"} {
			if raw, ok := obj[k]; ok && json.Unmarshal(raw, &rows) == nil {
				break
			}
		}
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		for _, k := range []string{"secretKey", "key"} {
			if v, ok := r[k].(string); ok && v != "" {
				out = append(out, v)
				break
			}
		}
	}
	return out
}

// globMatch is the policy engine's matcher (packages/policy-engine
// predicates.ts): `*` matches anything; otherwise `*` is the only
// metacharacter, and the pattern is anchored.
func globMatch(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	re, err := regexp.Compile("^" + strings.Join(parts, ".*") + "$")
	if err != nil {
		return false
	}
	return re.MatchString(value)
}

func anyGlob(globs []string, value string) bool {
	for _, g := range globs {
		if globMatch(g, value) {
			return true
		}
	}
	return false
}

// optList reads an optional string-array argument: (values, present).
func (a argmap) optList(k string) ([]string, bool) {
	v, ok := a[k]
	if !ok || v == nil {
		return nil, false
	}
	return a.strList(k), true
}

// ── Writes ──────────────────────────────────────────────────────────────

func (p *Provider) callTaskWrite(ctx context.Context, name string, a argmap, ws, key string) (string, error) {
	switch name {
	case "task_create":
		return p.taskCreate(ctx, a, ws, key)

	case "epic_create":
		if err := requireStr(a, name, "name"); err != nil {
			return "", err
		}
		req := remotestate.EpicCreateRequest{
			Name: a.str("name"), Slug: a.str("slug"), Description: a.str("description"),
			TargetDate: a.str("targetDate"), Owner: a.str("owner"),
		}
		epic, err := p.API.CreateEpicWithKey(ctx, ws, req, key)
		if err != nil {
			// The server refuses a taken slug with the holder attached —
			// the collision is the caller's decision, and for an idempotent
			// caller the decision is "that one".
			if existing := remotestate.ExistingEpicOf(err); existing != nil {
				return emit(fmt.Sprintf("epic %s already exists (%s) — reusing it", existing.Slug, orID(existing.Key, existing.ID)),
					map[string]interface{}{"epic": existing, "existed": true})
			}
			return "", err
		}
		return emit(fmt.Sprintf("epic %s created (%s); club tasks with task_create epic=%s", epic.Slug, orID(epic.Key, epic.ID), epic.Slug),
			map[string]interface{}{"epic": epic, "existed": false})

	case "milestone_create":
		if err := requireStr(a, name, "epic", "name"); err != nil {
			return "", err
		}
		req := remotestate.MilestoneCreateRequest{Name: a.str("name"), TargetDate: a.str("targetDate")}
		if crit, ok := a.optList("exitCriteria"); ok {
			req.ExitCriteria = crit
		}
		// The three positions: a sibling, an explicit null (first), absent
		// (last) — the wire distinguishes them and so does the argument.
		if after, present := a["after"]; present {
			if s, _ := after.(string); s != "" {
				req.After = s
			} else {
				req.First = true
			}
		}
		m, err := p.API.CreateMilestoneWithKey(ctx, ws, a.str("epic"), req, key)
		if err != nil {
			return "", err
		}
		label := m.Name
		if label == "" {
			label = m.ID
		}
		return emit(fmt.Sprintf("milestone %s (%s) added to epic %s; place tasks with task_create milestone=%s", label, m.ID, a.str("epic"), m.ID),
			map[string]interface{}{"milestone": m})
	}
	return "", fmt.Errorf("unknown tool %s", name)
}

func (p *Provider) taskCreate(ctx context.Context, a argmap, ws, key string) (string, error) {
	req := remotestate.TaskCreateRequest{
		AdoptKey: a.str("adoptKey"), MintPrefix: a.str("mintPrefix"), TitleMirror: a.str("titleMirror"),
		Brief: a.str("brief"), Epic: a.str("epic"), Milestone: a.str("milestone"), Assignee: a.str("assignee"),
	}
	if d, ok := a["derive"].(map[string]interface{}); ok {
		prefix, _ := d["repoPrefix"].(string)
		n, _ := d["issueNumber"].(float64)
		if prefix == "" || n <= 0 {
			return "", fmt.Errorf("task_create: derive wants repoPrefix and a positive issueNumber")
		}
		req.Derive = &remotestate.TaskDerive{RepoPrefix: prefix, IssueNumber: int(n)}
	}
	// Validate the contract BEFORE the allocator is asked: a malformed
	// contract must not cost a minted key.
	var draft *contract.Contract
	if raw, ok := a["contract"].(map[string]interface{}); ok {
		c, err := contractFromArgs(raw)
		if err != nil {
			return "", err
		}
		draft = c
	}
	task, err := p.API.CreateTaskWithKey(ctx, ws, req, key)
	if err != nil {
		return "", err
	}
	var attached interface{}
	gates := ""
	if draft != nil {
		hash, wire, err := contract.ContractID(draft)
		if err != nil {
			return "", err
		}
		seal, err := p.API.AttachTaskContractWithKey(ctx, ws, task.ID, wire, hash, key+":contract")
		if err != nil {
			return "", err
		}
		task.ContractHash = seal.ContractHash
		attached = map[string]interface{}{"contractHash": seal.ContractHash}
		if len(draft.Gates) == 0 {
			gates = "; contract attached (merge alone finishes it)"
		} else {
			gates = fmt.Sprintf("; contract attached (%d gate(s))", len(draft.Gates))
		}
	}
	where := ""
	switch {
	case task.Milestone != nil:
		where = " in milestone " + orID(task.Milestone.Name, task.Milestone.ID)
	case task.Epic != nil:
		where = " under epic " + task.Epic.Slug
	}
	return emit(fmt.Sprintf("task %s created (%s%s%s; branch grammar: orun/%s-<slug>)", task.Key, task.KeyOrigin, where, gates, task.Key),
		map[string]interface{}{"task": task, "contract": attached})
}

// contractFromArgs builds the sealed contract from task_create's `contract`
// argument. `gates` is always DECLARED here (gatesDefined: true) — an empty
// list means "merge alone finishes it", the fold's one authored fact.
func contractFromArgs(raw map[string]interface{}) (*contract.Contract, error) {
	list := func(k string) []string {
		out := []string{}
		if arr, ok := raw[k].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok && s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	}
	goal, _ := raw["goal"].(string)
	c := &contract.Contract{
		Goal: goal, Affects: list("affects"), DoneWhen: list("doneWhen"), Gates: list("gates"),
		DesignRefs: list("designRefs"), Deps: list("deps"), Secrets: list("secrets"), Envs: list("envs"),
		GatesDefined: true,
	}
	if c.Goal == "" || len(c.Affects) == 0 || len(c.DoneWhen) == 0 {
		return nil, fmt.Errorf("task_create: contract wants goal, affects (≥1) and doneWhen (≥1)")
	}
	if _, ok := raw["gates"]; !ok {
		return nil, fmt.Errorf("task_create: contract wants gates — an empty list means merge alone finishes the work")
	}
	return c, nil
}

// orID prefers the human handle and falls back to the durable id.
func orID(handle, id string) string {
	if handle != "" {
		return handle
	}
	return id
}
