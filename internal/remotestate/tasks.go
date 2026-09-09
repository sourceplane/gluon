package remotestate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
)

// Tasks client (orun-tasks O2) — the CLI's door to the task plane:
// `/v1/organizations/{org}/tasks…` through the api-edge facade. Create is
// the ONE identity write (the cloud allocator is the single writer of task
// identity, TK-I; the CLI never invents a key), attach uploads a contract
// the author sealed locally (the server recomputes the hash and refuses a
// mismatch, TK-J), and every read mirrors the wire types in
// orun-cloud packages/contracts/src/tasks.ts. The verdict is derived at
// read on the server (TK-M) — nothing here caches or stores one.

// PublicTask is a task as every read surface sees it (PublicTask on the
// wire): identity plus the R4 display mirror. Nullable wire fields decode
// to "" — absence, not a value.
type PublicTask struct {
	// ID is the durable tsk_… handle.
	ID        string `json:"id"`
	Key       string `json:"key"`
	KeyOrigin string `json:"keyOrigin"`
	// TaskRef is the object-store ref, once synced from the CLI side.
	TaskRef string `json:"taskRef"`
	// TitleMirror mirrors the tracker's title — never ours (R4).
	TitleMirror string `json:"titleMirror"`
	SyncedAt    string `json:"syncedAt"`
	CreatedBy   string `json:"createdBy"`
	CreatedAt   string `json:"createdAt"`
	CanceledAt  string `json:"canceledAt"`
	// ContractHash is present when a contract is attached — the identity
	// every gate decision cites.
	ContractHash string `json:"contractHash"`
	// Brief is orun's own "what done looks like" on a native task (W3);
	// Assignee is who in orun took it up — a member (usr_…) or an agent
	// principal (sp_…) — orun's fact, never the tracker's (TV4).
	Brief    string `json:"brief"`
	Assignee string `json:"assignee"`
	// Epic and Milestone are where the task belongs (E1/W3), when clubbed.
	Epic      *TaskEpicRef      `json:"epic,omitempty"`
	Milestone *TaskMilestoneRef `json:"milestone,omitempty"`
}

// TaskEpicRef is the epic membership as PublicTask carries it: the durable
// handle, the human slug and the minted key (EP-n; empty before backfill).
type TaskEpicRef struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Key  string `json:"key"`
}

// TaskMilestoneRef is the milestone (phase) membership: handle plus name.
type TaskMilestoneRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TaskDerive mirrors CreateTaskRequest.derive: repo prefix + issue number
// for DERIVED keys (`web#123` → `WEB-123`).
type TaskDerive struct {
	RepoPrefix  string `json:"repoPrefix"`
	IssueNumber int    `json:"issueNumber"`
}

// TaskCreateRequest mirrors CreateTaskRequest. The allocator is the single
// writer: adopt only if free, else derive, else the sequence — never a
// client-side choice.
type TaskCreateRequest struct {
	AdoptKey    string      `json:"adoptKey,omitempty"`
	Derive      *TaskDerive `json:"derive,omitempty"`
	MintPrefix  string      `json:"mintPrefix,omitempty"`
	TitleMirror string      `json:"titleMirror,omitempty"`
	// Brief is the W3 brief (at most 4000 chars server-side).
	Brief string `json:"brief,omitempty"`
	// Epic / Milestone club the task in the same create (an epc_… id, an
	// EP-n key or a slug; an mls_… id, which implies its epic). Resolved
	// before the key is minted — a bad ref is a 422, never a half-made task.
	Epic      string `json:"epic,omitempty"`
	Milestone string `json:"milestone,omitempty"`
	// Assignee is a subject ref (usr_… / sp_…) or "me" for the caller.
	Assignee string `json:"assignee,omitempty"`
}

// TaskListFilter mirrors ListTasksFilter's membership and assignment
// filters (E4 / TV4): an epic ref (epc_… or slug), a milestone id (mls_…),
// and an assignee — a subject ref, "me" (the caller) or "agents" (any
// sp_… assignee). Empty fields do not filter.
type TaskListFilter struct {
	Epic      string
	Milestone string
	Assignee  string
}

func (f TaskListFilter) query() string {
	q := url.Values{}
	if f.Epic != "" {
		q.Set("epic", f.Epic)
	}
	if f.Milestone != "" {
		q.Set("milestone", f.Milestone)
	}
	if f.Assignee != "" {
		q.Set("assignee", f.Assignee)
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// TasksList mirrors ListTasksResponse.
type TasksList struct {
	Tasks []PublicTask `json:"tasks"`
}

// TaskContractSeal mirrors AttachContractResponse: the stored identity —
// always the server's own recomputation.
type TaskContractSeal struct {
	ContractHash string `json:"contractHash"`
	SyncedAt     string `json:"syncedAt"`
}

// TaskContractView mirrors GetContractResponse. The body stays raw bytes on
// this seam: remotestate carries the wire, internal/contract owns the type.
type TaskContractView struct {
	ContractHash string          `json:"contractHash"`
	Contract     json.RawMessage `json:"contract"`
	SyncedAt     string          `json:"syncedAt"`
}

// TaskVerdict mirrors TaskVerdictWire — the derived rung plus the evidence
// that produced it (a rung without a reason is a status column in costume).
type TaskVerdict struct {
	Rung     string `json:"rung"`
	Evidence struct {
		ObservationID string `json:"observationId"`
		Reason        string `json:"reason"`
	} `json:"evidence"`
	Pin *struct {
		Rung   string `json:"rung"`
		Active bool   `json:"active"`
	} `json:"pin"`
	Dissent *struct {
		Asserted string `json:"asserted"`
	} `json:"dissent"`
	Blocked   bool     `json:"blocked"`
	BlockedBy []string `json:"blockedBy"`
}

// TaskDependency is one resolved contract dep (GetVerdictResponse).
type TaskDependency struct {
	Ref   string `json:"ref"`
	State string `json:"state"` // open | done | canceled | unknown
}

// TaskObservation is one observed fact (TaskObservationWire) — evidence,
// never status.
type TaskObservation struct {
	ID         string                 `json:"id"`
	Kind       string                 `json:"kind"`
	OccurredAt string                 `json:"occurredAt"`
	Payload    map[string]interface{} `json:"payload"`
}

// TaskVerdictView mirrors GetVerdictResponse: the verdict plus the evidence
// it cites, resolved in the same read.
type TaskVerdictView struct {
	Verdict      TaskVerdict       `json:"verdict"`
	ContractHash string            `json:"contractHash"`
	Dependencies []TaskDependency  `json:"dependencies"`
	Observations []TaskObservation `json:"observations"`
}

func tasksPathFor(org, suffix string) string {
	return orgPath(org, "/tasks"+suffix)
}

// CreateTask asks the allocator for a task (adopt > derive > mint — the
// server's ladder, not ours). POSTs are not retried here: the caller decides
// what a second attempt means.
func (c *Client) CreateTask(ctx context.Context, org string, req TaskCreateRequest) (*PublicTask, error) {
	var resp struct {
		Task PublicTask `json:"task"`
	}
	if err := c.doJSON(ctx, http.MethodPost, tasksPathFor(org, ""), req, &resp, false); err != nil {
		return nil, err
	}
	return &resp.Task, nil
}

// ListTasks fetches the org's tasks (newest first, unfiltered).
func (c *Client) ListTasks(ctx context.Context, org string) (*TasksList, error) {
	return c.ListTasksWhere(ctx, org, TaskListFilter{})
}

// ListTasksWhere fetches the org's tasks narrowed by membership and/or
// assignment — the read a find-or-create loop keys on (`epic` + title).
func (c *Client) ListTasksWhere(ctx context.Context, org string, filter TaskListFilter) (*TasksList, error) {
	var resp TasksList
	if err := c.doJSON(ctx, http.MethodGet, tasksPathFor(org, filter.query()), nil, &resp, true); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetTask fetches one task by key (ENG-42) or durable id (tsk_…).
func (c *Client) GetTask(ctx context.Context, org, keyOrID string) (*PublicTask, error) {
	var resp struct {
		Task PublicTask `json:"task"`
	}
	if err := c.doJSON(ctx, http.MethodGet, tasksPathFor(org, "/"+urlSegment(keyOrID)), nil, &resp, true); err != nil {
		return nil, err
	}
	return &resp.Task, nil
}

// AttachTaskContract uploads a contract's canonical wire bytes plus the hash
// the author sealed; the server recomputes and refuses a mismatch, so a
// corrupted body never becomes the projection. Retried freely: attaching
// identical content is idempotent by construction.
func (c *Client) AttachTaskContract(ctx context.Context, org, keyOrID string, contractWire json.RawMessage, contractHash string) (*TaskContractSeal, error) {
	req := struct {
		Contract     json.RawMessage `json:"contract"`
		ContractHash string          `json:"contractHash,omitempty"`
	}{Contract: contractWire, ContractHash: contractHash}
	var resp TaskContractSeal
	if err := c.doJSON(ctx, http.MethodPut, tasksPathFor(org, "/"+urlSegment(keyOrID)+"/contract"), req, &resp, true); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetTaskContract fetches the attached contract.
func (c *Client) GetTaskContract(ctx context.Context, org, keyOrID string) (*TaskContractView, error) {
	var resp TaskContractView
	if err := c.doJSON(ctx, http.MethodGet, tasksPathFor(org, "/"+urlSegment(keyOrID)+"/contract"), nil, &resp, true); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetTaskVerdict fetches the derived verdict plus its evidence.
func (c *Client) GetTaskVerdict(ctx context.Context, org, keyOrID string) (*TaskVerdictView, error) {
	var resp TaskVerdictView
	if err := c.doJSON(ctx, http.MethodGet, tasksPathFor(org, "/"+urlSegment(keyOrID)+"/verdict"), nil, &resp, true); err != nil {
		return nil, err
	}
	return &resp, nil
}
