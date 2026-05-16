package quotaawarepreempt

import (
	"github.com/kaschnit/custom-scheduler/internal/fwkutil"
	"github.com/kaschnit/custom-scheduler/plugin/quotaawarepreempt/queue"
	fwk "k8s.io/kube-scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

const stateKeyPreFilter fwk.StateKey = "PreFilter" + PluginName

var _ fwk.StateData = (*RequstedResourceState)(nil)

// RequstedResourceState is shared scheduling state related to requested resources.
type RequstedResourceState struct {
	// Request is the requested resources for the pod this cycle.
	Request framework.Resource
	// NominatedReqInQuota is the sum of requests in the quota for pods
	// that have a nominated node in the current cycle.
	NominatedReqInQuota framework.Resource
}

// Clone implements [fwk.StateData].
func (s *RequstedResourceState) Clone() fwk.StateData {
	return &RequstedResourceState{
		Request:             *s.Request.Clone(),
		NominatedReqInQuota: *s.NominatedReqInQuota.Clone(),
	}
}

const stateKeyQuotaSnapshot fwk.StateKey = "QuotaSnapshot" + PluginName

var _ fwk.StateData = (*QueueSnapshotState)(nil)

// QueueSnapshotState is shared scheduling state related to quota usage.
type QueueSnapshotState struct {
	QueueMgr *queue.Manager
}

// Clone implements [fwk.StateData].
func (s *QueueSnapshotState) Clone() fwk.StateData {
	return &QueueSnapshotState{
		QueueMgr: s.QueueMgr.Clone(),
	}
}

// StateManager manages the scheduling cycle state for the quota-aware preemption plugin.
type StateManager struct {
	cycleState fwk.CycleState
}

// NewStateManager creates a new [StateManager].
func NewStateManager(cycleState fwk.CycleState) *StateManager {
	return &StateManager{
		cycleState: cycleState,
	}
}

// ReadRequstedResource reads the requested resource data from the scheduling cycle state.
func (mgr *StateManager) ReadRequstedResource() (*RequstedResourceState, error) {
	return fwkutil.ReadState[*RequstedResourceState](mgr.cycleState, stateKeyPreFilter)
}

// WriteRequestedResource writes the requested resource data to the scheduling cycle state.
func (mgr *StateManager) WriteRequestedResource(data *RequstedResourceState) {
	mgr.cycleState.Write(stateKeyPreFilter, data)
}

// ReadQueueSnapshot reads the queue snapshot from the scheduling cycle state.
func (mgr *StateManager) ReadQueueSnapshot() (*QueueSnapshotState, error) {
	return fwkutil.ReadState[*QueueSnapshotState](mgr.cycleState, stateKeyQuotaSnapshot)
}

// WriteQueueSnapshot writes the queue snapshot to the scheduling cycle state.
func (mgr *StateManager) WriteQueueSnapshot(data *QueueSnapshotState) {
	mgr.cycleState.Write(stateKeyQuotaSnapshot, data)
}
