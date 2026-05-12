package quotaawarepreempt

import (
	"github.com/kaschnit/custom-scheduler/internal/fwkutil"
	"github.com/kaschnit/custom-scheduler/quotaawarepreempt/queue"
	fwk "k8s.io/kube-scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

const stateKeyPreFilter fwk.StateKey = "PreFilter" + PluginName

var _ fwk.StateData = (*RequstedResourceState)(nil)

// RequstedResourceState is shared scheduling state related to requested resources.
type RequstedResourceState struct {
	request             framework.Resource
	nominatedReqInQuota framework.Resource
}

// Clone implements [fwk.StateData].
func (s *RequstedResourceState) Clone() fwk.StateData {
	return &RequstedResourceState{
		request:             *s.request.Clone(),
		nominatedReqInQuota: *s.nominatedReqInQuota.Clone(),
	}
}

const stateKeyQuotaSnapshot fwk.StateKey = "QuotaSnapshot" + PluginName

var _ fwk.StateData = (*QuotaSnapshotState)(nil)

// QuotaSnapshotState is shared scheduling state related to quota usage.
type QuotaSnapshotState struct {
	QuotaMgr *queue.QuotaManager
}

// Clone implements [fwk.StateData].
func (s *QuotaSnapshotState) Clone() fwk.StateData {
	return &QuotaSnapshotState{
		QuotaMgr: s.QuotaMgr.Clone(),
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

// ReadQuotaSnapshot reads the quota usage snapshot data from the scheduling cycle state.
func (mgr *StateManager) ReadQuotaSnapshot() (*QuotaSnapshotState, error) {
	return fwkutil.ReadState[*QuotaSnapshotState](mgr.cycleState, stateKeyQuotaSnapshot)
}

// WriteQuotaSnapshot writes the quota usage snapshot data to the scheduling cycle state.
func (mgr *StateManager) WriteQuotaSnapshot(data *QuotaSnapshotState) {
	mgr.cycleState.Write(stateKeyQuotaSnapshot, data)
}
