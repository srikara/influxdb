package pkger

import (
	"reflect"
	"sort"

	"github.com/influxdata/influxdb/v2"
	"github.com/influxdata/influxdb/v2/notification/rule"
)

type stateCoordinator struct {
	mBuckets    map[string]*stateBucket
	mChecks     map[string]*stateCheck
	mDashboards map[string]*stateDashboard
	mEndpoints  map[string]*stateEndpoint
	mLabels     map[string]*stateLabel
	mRules      map[string]*stateRule
	mTasks      map[string]*stateTask
	mTelegrafs  map[string]*stateTelegraf
	mVariables  map[string]*stateVariable

	labelMappings         []stateLabelMapping
	labelMappingsToRemove []stateLabelMappingForRemoval
}

func newStateCoordinator(pkg *Pkg) *stateCoordinator {
	state := stateCoordinator{
		mBuckets:    make(map[string]*stateBucket),
		mChecks:     make(map[string]*stateCheck),
		mDashboards: make(map[string]*stateDashboard),
		mEndpoints:  make(map[string]*stateEndpoint),
		mLabels:     make(map[string]*stateLabel),
		mRules:      make(map[string]*stateRule),
		mTasks:      make(map[string]*stateTask),
		mTelegrafs:  make(map[string]*stateTelegraf),
		mVariables:  make(map[string]*stateVariable),
	}

	for _, pkgBkt := range pkg.buckets() {
		state.mBuckets[pkgBkt.PkgName()] = &stateBucket{
			parserBkt:   pkgBkt,
			stateStatus: StateStatusNew,
		}
	}
	for _, pkgCheck := range pkg.checks() {
		state.mChecks[pkgCheck.PkgName()] = &stateCheck{
			parserCheck: pkgCheck,
			stateStatus: StateStatusNew,
		}
	}
	for _, pkgDash := range pkg.dashboards() {
		state.mDashboards[pkgDash.PkgName()] = &stateDashboard{
			parserDash:  pkgDash,
			stateStatus: StateStatusNew,
		}
	}
	for _, pkgEndpoint := range pkg.notificationEndpoints() {
		state.mEndpoints[pkgEndpoint.PkgName()] = &stateEndpoint{
			parserEndpoint: pkgEndpoint,
			stateStatus:    StateStatusNew,
		}
	}
	for _, pkgLabel := range pkg.labels() {
		state.mLabels[pkgLabel.PkgName()] = &stateLabel{
			parserLabel: pkgLabel,
			stateStatus: StateStatusNew,
		}
	}
	for _, pkgRule := range pkg.notificationRules() {
		state.mRules[pkgRule.PkgName()] = &stateRule{
			parserRule:  pkgRule,
			stateStatus: StateStatusNew,
		}
	}
	for _, pkgTask := range pkg.tasks() {
		state.mTasks[pkgTask.PkgName()] = &stateTask{
			parserTask:  pkgTask,
			stateStatus: StateStatusNew,
		}
	}
	for _, pkgTele := range pkg.telegrafs() {
		state.mTelegrafs[pkgTele.PkgName()] = &stateTelegraf{
			parserTelegraf: pkgTele,
			stateStatus:    StateStatusNew,
		}
	}
	for _, pkgVar := range pkg.variables() {
		state.mVariables[pkgVar.PkgName()] = &stateVariable{
			parserVar:   pkgVar,
			stateStatus: StateStatusNew,
		}
	}

	return &state
}

func (s *stateCoordinator) buckets() []*stateBucket {
	out := make([]*stateBucket, 0, len(s.mBuckets))
	for _, v := range s.mBuckets {
		out = append(out, v)
	}
	return out
}

func (s *stateCoordinator) checks() []*stateCheck {
	out := make([]*stateCheck, 0, len(s.mChecks))
	for _, v := range s.mChecks {
		out = append(out, v)
	}
	return out
}

func (s *stateCoordinator) dashboards() []*stateDashboard {
	out := make([]*stateDashboard, 0, len(s.mDashboards))
	for _, d := range s.mDashboards {
		out = append(out, d)
	}
	return out
}

func (s *stateCoordinator) endpoints() []*stateEndpoint {
	out := make([]*stateEndpoint, 0, len(s.mEndpoints))
	for _, e := range s.mEndpoints {
		out = append(out, e)
	}
	return out
}

func (s *stateCoordinator) labels() []*stateLabel {
	out := make([]*stateLabel, 0, len(s.mLabels))
	for _, v := range s.mLabels {
		out = append(out, v)
	}
	return out
}

func (s *stateCoordinator) rules() []*stateRule {
	out := make([]*stateRule, 0, len(s.mRules))
	for _, r := range s.mRules {
		out = append(out, r)
	}
	return out
}

func (s *stateCoordinator) tasks() []*stateTask {
	out := make([]*stateTask, 0, len(s.mTasks))
	for _, t := range s.mTasks {
		out = append(out, t)
	}
	return out
}

func (s *stateCoordinator) telegrafConfigs() []*stateTelegraf {
	out := make([]*stateTelegraf, 0, len(s.mTelegrafs))
	for _, t := range s.mTelegrafs {
		out = append(out, t)
	}
	return out
}

func (s *stateCoordinator) variables() []*stateVariable {
	out := make([]*stateVariable, 0, len(s.mVariables))
	for _, v := range s.mVariables {
		out = append(out, v)
	}
	return out
}

func (s *stateCoordinator) diff() Diff {
	var diff Diff
	for _, b := range s.mBuckets {
		diff.Buckets = append(diff.Buckets, b.diffBucket())
	}
	sort.Slice(diff.Buckets, func(i, j int) bool {
		return diff.Buckets[i].PkgName < diff.Buckets[j].PkgName
	})

	for _, c := range s.mChecks {
		diff.Checks = append(diff.Checks, c.diffCheck())
	}
	sort.Slice(diff.Checks, func(i, j int) bool {
		return diff.Checks[i].PkgName < diff.Checks[j].PkgName
	})

	for _, d := range s.mDashboards {
		diff.Dashboards = append(diff.Dashboards, d.diffDashboard())
	}
	sort.Slice(diff.Dashboards, func(i, j int) bool {
		return diff.Dashboards[i].PkgName < diff.Dashboards[j].PkgName
	})

	for _, e := range s.mEndpoints {
		diff.NotificationEndpoints = append(diff.NotificationEndpoints, e.diffEndpoint())
	}
	sort.Slice(diff.NotificationEndpoints, func(i, j int) bool {
		return diff.NotificationEndpoints[i].PkgName < diff.NotificationEndpoints[j].PkgName
	})

	for _, l := range s.mLabels {
		diff.Labels = append(diff.Labels, l.diffLabel())
	}
	sort.Slice(diff.Labels, func(i, j int) bool {
		return diff.Labels[i].PkgName < diff.Labels[j].PkgName
	})

	for _, r := range s.mRules {
		diff.NotificationRules = append(diff.NotificationRules, r.diffRule())
	}
	sort.Slice(diff.NotificationRules, func(i, j int) bool {
		return diff.NotificationRules[i].PkgName < diff.NotificationRules[j].PkgName
	})

	for _, t := range s.mTasks {
		diff.Tasks = append(diff.Tasks, t.diffTask())
	}
	sort.Slice(diff.Tasks, func(i, j int) bool {
		return diff.Tasks[i].PkgName < diff.Tasks[j].PkgName
	})

	for _, t := range s.mTelegrafs {
		diff.Telegrafs = append(diff.Telegrafs, t.diffTelegraf())
	}
	sort.Slice(diff.Telegrafs, func(i, j int) bool {
		return diff.Telegrafs[i].PkgName < diff.Telegrafs[j].PkgName
	})

	for _, v := range s.mVariables {
		diff.Variables = append(diff.Variables, v.diffVariable())
	}
	sort.Slice(diff.Variables, func(i, j int) bool {
		return diff.Variables[i].PkgName < diff.Variables[j].PkgName
	})

	for _, m := range s.labelMappings {
		diff.LabelMappings = append(diff.LabelMappings, m.diffLabelMapping())
	}
	for _, m := range s.labelMappingsToRemove {
		diff.LabelMappings = append(diff.LabelMappings, m.diffLabelMapping())
	}

	sort.Slice(diff.LabelMappings, func(i, j int) bool {
		n, m := diff.LabelMappings[i], diff.LabelMappings[j]
		if n.ResType < m.ResType {
			return true
		}
		if n.ResType > m.ResType {
			return false
		}
		if n.ResPkgName < m.ResPkgName {
			return true
		}
		if n.ResPkgName > m.ResPkgName {
			return false
		}
		return n.LabelName < m.LabelName
	})

	return diff
}

func (s *stateCoordinator) summary() Summary {
	var sum Summary
	for _, v := range s.mBuckets {
		if IsRemoval(v.stateStatus) {
			continue
		}
		sum.Buckets = append(sum.Buckets, v.summarize())
	}
	sort.Slice(sum.Buckets, func(i, j int) bool {
		return sum.Buckets[i].PkgName < sum.Buckets[j].PkgName
	})

	for _, c := range s.mChecks {
		if IsRemoval(c.stateStatus) {
			continue
		}
		sum.Checks = append(sum.Checks, c.summarize())
	}
	sort.Slice(sum.Checks, func(i, j int) bool {
		return sum.Checks[i].PkgName < sum.Checks[j].PkgName
	})

	for _, d := range s.mDashboards {
		if IsRemoval(d.stateStatus) {
			continue
		}
		sum.Dashboards = append(sum.Dashboards, d.summarize())
	}
	sort.Slice(sum.Dashboards, func(i, j int) bool {
		return sum.Dashboards[i].PkgName < sum.Dashboards[j].PkgName
	})

	for _, e := range s.mEndpoints {
		if IsRemoval(e.stateStatus) {
			continue
		}
		sum.NotificationEndpoints = append(sum.NotificationEndpoints, e.summarize())
	}
	sort.Slice(sum.NotificationEndpoints, func(i, j int) bool {
		return sum.NotificationEndpoints[i].PkgName < sum.NotificationEndpoints[j].PkgName
	})

	for _, v := range s.mLabels {
		if IsRemoval(v.stateStatus) {
			continue
		}
		sum.Labels = append(sum.Labels, v.summarize())
	}
	sort.Slice(sum.Labels, func(i, j int) bool {
		return sum.Labels[i].PkgName < sum.Labels[j].PkgName
	})

	for _, v := range s.mRules {
		if IsRemoval(v.stateStatus) {
			continue
		}
		sum.NotificationRules = append(sum.NotificationRules, v.summarize())
	}
	sort.Slice(sum.NotificationRules, func(i, j int) bool {
		return sum.NotificationRules[i].PkgName < sum.NotificationRules[j].PkgName
	})

	for _, t := range s.mTasks {
		if IsRemoval(t.stateStatus) {
			continue
		}
		sum.Tasks = append(sum.Tasks, t.summarize())
	}
	sort.Slice(sum.Tasks, func(i, j int) bool {
		return sum.Tasks[i].PkgName < sum.Tasks[j].PkgName
	})

	for _, t := range s.mTelegrafs {
		if IsRemoval(t.stateStatus) {
			continue
		}
		sum.TelegrafConfigs = append(sum.TelegrafConfigs, t.summarize())
	}
	sort.Slice(sum.TelegrafConfigs, func(i, j int) bool {
		return sum.TelegrafConfigs[i].PkgName < sum.TelegrafConfigs[j].PkgName
	})

	for _, v := range s.mVariables {
		if IsRemoval(v.stateStatus) {
			continue
		}
		sum.Variables = append(sum.Variables, v.summarize())
	}
	sort.Slice(sum.Variables, func(i, j int) bool {
		return sum.Variables[i].PkgName < sum.Variables[j].PkgName
	})

	for _, v := range s.labelMappings {
		sum.LabelMappings = append(sum.LabelMappings, v.summarize())
	}
	sort.Slice(sum.LabelMappings, func(i, j int) bool {
		n, m := sum.LabelMappings[i], sum.LabelMappings[j]
		if n.ResourceType != m.ResourceType {
			return n.ResourceType < m.ResourceType
		}
		if n.ResourcePkgName != m.ResourcePkgName {
			return n.ResourcePkgName < m.ResourcePkgName
		}
		return n.LabelName < m.LabelName
	})

	return sum
}

func (s *stateCoordinator) getLabelByPkgName(pkgName string) *stateLabel {
	return s.mLabels[pkgName]
}

func (s *stateCoordinator) addStackState(stack Stack) {
	reconcilers := []func([]StackResource){
		s.reconcileStackResources,
		s.reconcileLabelMappings,
		s.reconcileNotificationDependencies,
	}
	for _, reconcileFn := range reconcilers {
		reconcileFn(stack.Resources)
	}
}

func (s *stateCoordinator) reconcileStackResources(stackResources []StackResource) {
	for _, r := range stackResources {
		if !s.Contains(r.Kind, r.PkgName) {
			s.addObjectForRemoval(r.Kind, r.PkgName, r.ID)
			continue
		}
		s.setObjectID(r.Kind, r.PkgName, r.ID)
	}
}

func (s *stateCoordinator) reconcileLabelMappings(stackResources []StackResource) {
	mLabelPkgNameToID := make(map[string]influxdb.ID)
	for _, r := range stackResources {
		if r.Kind.is(KindLabel) {
			mLabelPkgNameToID[r.PkgName] = r.ID
		}
	}

	for _, r := range stackResources {
		labels := s.labelAssociations(r.Kind, r.PkgName)
		if len(r.Associations) == 0 {
			continue
		}

		// if associations agree => do nothing
		// if associations are new (in state not in stack) => do nothing
		// if associations are not in state and in stack => add them for removal
		mStackAss := make(map[StackResourceAssociation]struct{})
		for _, ass := range r.Associations {
			if ass.Kind.is(KindLabel) {
				mStackAss[ass] = struct{}{}
			}
		}

		for _, l := range labels {
			// we want to keep associations that are from previous application and are not changing
			delete(mStackAss, StackResourceAssociation{
				Kind:    KindLabel,
				PkgName: l.parserLabel.PkgName(),
			})
		}

		// all associations that are in the stack but not in the
		// state fall into here and are marked for removal.
		for assForRemoval := range mStackAss {
			s.labelMappingsToRemove = append(s.labelMappingsToRemove, stateLabelMappingForRemoval{
				LabelPkgName:    assForRemoval.PkgName,
				LabelID:         mLabelPkgNameToID[assForRemoval.PkgName],
				ResourceID:      r.ID,
				ResourcePkgName: r.PkgName,
				ResourceType:    r.Kind.ResourceType(),
			})
		}
	}
}

func (s *stateCoordinator) reconcileNotificationDependencies(stackResources []StackResource) {
	for _, r := range stackResources {
		if r.Kind.is(KindNotificationRule) {
			for _, ass := range r.Associations {
				if ass.Kind.is(KindNotificationEndpoint) {
					s.mRules[r.PkgName].associatedEndpoint = s.mEndpoints[ass.PkgName]
					break
				}
			}
		}
	}
}

func (s *stateCoordinator) get(k Kind, pkgName string) (interface{}, bool) {
	switch k {
	case KindBucket:
		v, ok := s.mBuckets[pkgName]
		return v, ok
	case KindCheck, KindCheckDeadman, KindCheckThreshold:
		v, ok := s.mChecks[pkgName]
		return v, ok
	case KindDashboard:
		v, ok := s.mDashboards[pkgName]
		return v, ok
	case KindLabel:
		v, ok := s.mLabels[pkgName]
		return v, ok
	case KindNotificationEndpoint,
		KindNotificationEndpointHTTP,
		KindNotificationEndpointPagerDuty,
		KindNotificationEndpointSlack:
		v, ok := s.mEndpoints[pkgName]
		return v, ok
	case KindNotificationRule:
		v, ok := s.mRules[pkgName]
		return v, ok
	case KindTask:
		v, ok := s.mTasks[pkgName]
		return v, ok
	case KindTelegraf:
		v, ok := s.mTelegrafs[pkgName]
		return v, ok
	case KindVariable:
		v, ok := s.mVariables[pkgName]
		return v, ok
	default:
		return nil, false
	}
}

func (s *stateCoordinator) labelAssociations(k Kind, pkgName string) []*stateLabel {
	type labelAssociater interface {
		labels() []*label
	}

	v, _ := s.get(k, pkgName)
	labeler, ok := v.(labelAssociater)
	if !ok {
		return nil
	}

	var out []*stateLabel
	for _, l := range labeler.labels() {
		out = append(out, s.mLabels[l.PkgName()])
	}
	return out
}

func (s *stateCoordinator) Contains(k Kind, pkgName string) bool {
	_, ok := s.get(k, pkgName)
	return ok
}

// setObjectID sets the id for the resource graphed from the object the key identifies.
func (s *stateCoordinator) setObjectID(k Kind, pkgName string, id influxdb.ID) {
	idSetFn, ok := s.getObjectIDSetter(k, pkgName)
	if !ok {
		return
	}
	idSetFn(id)
}

// addObjectForRemoval sets the id for the resource graphed from the object the key identifies.
// The pkgName and kind are used as the unique identifier, when calling this it will
// overwrite any existing value if one exists. If desired, check for the value by using
// the Contains method.
func (s *stateCoordinator) addObjectForRemoval(k Kind, pkgName string, id influxdb.ID) {
	newIdentity := identity{
		name: &references{val: pkgName},
	}

	switch k {
	case KindBucket:
		s.mBuckets[pkgName] = &stateBucket{
			id:          id,
			parserBkt:   &bucket{identity: newIdentity},
			stateStatus: StateStatusRemove,
		}
	case KindCheck, KindCheckDeadman, KindCheckThreshold:
		s.mChecks[pkgName] = &stateCheck{
			id:          id,
			parserCheck: &check{identity: newIdentity},
			stateStatus: StateStatusRemove,
		}
	case KindDashboard:
		s.mDashboards[pkgName] = &stateDashboard{
			id:          id,
			parserDash:  &dashboard{identity: newIdentity},
			stateStatus: StateStatusRemove,
		}
	case KindLabel:
		s.mLabels[pkgName] = &stateLabel{
			id:          id,
			parserLabel: &label{identity: newIdentity},
			stateStatus: StateStatusRemove,
		}
	case KindNotificationEndpoint,
		KindNotificationEndpointHTTP,
		KindNotificationEndpointPagerDuty,
		KindNotificationEndpointSlack:
		s.mEndpoints[pkgName] = &stateEndpoint{
			id:             id,
			parserEndpoint: &notificationEndpoint{identity: newIdentity},
			stateStatus:    StateStatusRemove,
		}
	case KindNotificationRule:
		s.mRules[pkgName] = &stateRule{
			id:          id,
			parserRule:  &notificationRule{identity: newIdentity},
			stateStatus: StateStatusRemove,
		}
	case KindTask:
		s.mTasks[pkgName] = &stateTask{
			id:          id,
			parserTask:  &task{identity: newIdentity},
			stateStatus: StateStatusRemove,
		}
	case KindTelegraf:
		s.mTelegrafs[pkgName] = &stateTelegraf{
			id:             id,
			parserTelegraf: &telegraf{identity: newIdentity},
			stateStatus:    StateStatusRemove,
		}
	case KindVariable:
		s.mVariables[pkgName] = &stateVariable{
			id:          id,
			parserVar:   &variable{identity: newIdentity},
			stateStatus: StateStatusRemove,
		}
	}
}

func (s *stateCoordinator) getObjectIDSetter(k Kind, pkgName string) (func(influxdb.ID), bool) {
	switch k {
	case KindBucket:
		r, ok := s.mBuckets[pkgName]
		return func(id influxdb.ID) {
			r.id = id
			r.stateStatus = StateStatusExists
		}, ok
	case KindCheck, KindCheckDeadman, KindCheckThreshold:
		r, ok := s.mChecks[pkgName]
		return func(id influxdb.ID) {
			r.id = id
			r.stateStatus = StateStatusExists
		}, ok
	case KindDashboard:
		r, ok := s.mDashboards[pkgName]
		return func(id influxdb.ID) {
			r.id = id
			r.stateStatus = StateStatusExists
		}, ok
	case KindLabel:
		r, ok := s.mLabels[pkgName]
		return func(id influxdb.ID) {
			r.id = id
			r.stateStatus = StateStatusExists
		}, ok
	case KindNotificationEndpoint,
		KindNotificationEndpointHTTP,
		KindNotificationEndpointPagerDuty,
		KindNotificationEndpointSlack:
		r, ok := s.mEndpoints[pkgName]
		return func(id influxdb.ID) {
			r.id = id
			r.stateStatus = StateStatusExists
		}, ok
	case KindNotificationRule:
		r, ok := s.mRules[pkgName]
		return func(id influxdb.ID) {
			r.id = id
			r.stateStatus = StateStatusExists
		}, ok
	case KindTask:
		r, ok := s.mTasks[pkgName]
		return func(id influxdb.ID) {
			r.id = id
			r.stateStatus = StateStatusExists
		}, ok
	case KindTelegraf:
		r, ok := s.mTelegrafs[pkgName]
		return func(id influxdb.ID) {
			r.id = id
			r.stateStatus = StateStatusExists
		}, ok
	case KindVariable:
		r, ok := s.mVariables[pkgName]
		return func(id influxdb.ID) {
			r.id = id
			r.stateStatus = StateStatusExists
		}, ok
	default:
		return nil, false
	}
}

type stateIdentity struct {
	id           influxdb.ID
	name         string
	pkgName      string
	resourceType influxdb.ResourceType
	stateStatus  StateStatus
}

func (s stateIdentity) exists() bool {
	return IsExisting(s.stateStatus)
}

type stateBucket struct {
	id, orgID   influxdb.ID
	stateStatus StateStatus

	parserBkt *bucket
	existing  *influxdb.Bucket
}

func (b *stateBucket) diffBucket() DiffBucket {
	diff := DiffBucket{
		DiffIdentifier: DiffIdentifier{
			ID:          SafeID(b.ID()),
			Remove:      IsRemoval(b.stateStatus),
			StateStatus: b.stateStatus,
			PkgName:     b.parserBkt.PkgName(),
		},
		New: DiffBucketValues{
			Name:           b.parserBkt.Name(),
			Description:    b.parserBkt.Description,
			RetentionRules: b.parserBkt.RetentionRules,
		},
	}
	if e := b.existing; e != nil {
		diff.Old = &DiffBucketValues{
			Name:        e.Name,
			Description: e.Description,
		}
		if e.RetentionPeriod > 0 {
			diff.Old.RetentionRules = retentionRules{newRetentionRule(e.RetentionPeriod)}
		}
	}
	return diff
}

func (b *stateBucket) summarize() SummaryBucket {
	sum := b.parserBkt.summarize()
	sum.ID = SafeID(b.ID())
	sum.OrgID = SafeID(b.orgID)
	return sum
}

func (b *stateBucket) ID() influxdb.ID {
	if !IsNew(b.stateStatus) && b.existing != nil {
		return b.existing.ID
	}
	return b.id
}

func (b *stateBucket) resourceType() influxdb.ResourceType {
	return KindBucket.ResourceType()
}

func (b *stateBucket) labels() []*label {
	return b.parserBkt.labels
}

func (b *stateBucket) stateIdentity() stateIdentity {
	return stateIdentity{
		id:           b.ID(),
		name:         b.parserBkt.Name(),
		pkgName:      b.parserBkt.PkgName(),
		resourceType: b.resourceType(),
		stateStatus:  b.stateStatus,
	}
}

func (b *stateBucket) shouldApply() bool {
	return IsRemoval(b.stateStatus) ||
		b.existing == nil ||
		b.parserBkt.Description != b.existing.Description ||
		b.parserBkt.Name() != b.existing.Name ||
		b.parserBkt.RetentionRules.RP() != b.existing.RetentionPeriod
}

type stateCheck struct {
	id, orgID   influxdb.ID
	stateStatus StateStatus

	parserCheck *check
	existing    influxdb.Check
}

func (c *stateCheck) ID() influxdb.ID {
	if !IsNew(c.stateStatus) && c.existing != nil {
		return c.existing.GetID()
	}
	return c.id
}

func (c *stateCheck) labels() []*label {
	return c.parserCheck.labels
}

func (c *stateCheck) resourceType() influxdb.ResourceType {
	return KindCheck.ResourceType()
}

func (c *stateCheck) stateIdentity() stateIdentity {
	return stateIdentity{
		id:           c.ID(),
		name:         c.parserCheck.Name(),
		pkgName:      c.parserCheck.PkgName(),
		resourceType: c.resourceType(),
		stateStatus:  c.stateStatus,
	}
}

func (c *stateCheck) diffCheck() DiffCheck {
	diff := DiffCheck{
		DiffIdentifier: DiffIdentifier{
			ID:          SafeID(c.ID()),
			Remove:      IsRemoval(c.stateStatus),
			StateStatus: c.stateStatus,
			PkgName:     c.parserCheck.PkgName(),
		},
	}
	if newCheck := c.summarize(); newCheck.Check != nil {
		diff.New.Check = newCheck.Check
	}
	if c.existing != nil {
		diff.Old = &DiffCheckValues{
			Check: c.existing,
		}
	}
	return diff
}

func (c *stateCheck) summarize() SummaryCheck {
	sum := c.parserCheck.summarize()
	if sum.Check == nil {
		return sum
	}
	sum.Check.SetID(c.id)
	sum.Check.SetOrgID(c.orgID)
	return sum
}

type stateDashboard struct {
	id, orgID   influxdb.ID
	stateStatus StateStatus

	parserDash *dashboard
	existing   *influxdb.Dashboard
}

func (d *stateDashboard) ID() influxdb.ID {
	if !IsNew(d.stateStatus) && d.existing != nil {
		return d.existing.ID
	}
	return d.id
}

func (d *stateDashboard) labels() []*label {
	return d.parserDash.labels
}

func (d *stateDashboard) resourceType() influxdb.ResourceType {
	return KindDashboard.ResourceType()
}

func (d *stateDashboard) stateIdentity() stateIdentity {
	return stateIdentity{
		id:           d.ID(),
		name:         d.parserDash.Name(),
		pkgName:      d.parserDash.PkgName(),
		resourceType: d.resourceType(),
		stateStatus:  d.stateStatus,
	}
}

func (d *stateDashboard) diffDashboard() DiffDashboard {
	diff := DiffDashboard{
		DiffIdentifier: DiffIdentifier{
			ID:          SafeID(d.ID()),
			Remove:      IsRemoval(d.stateStatus),
			StateStatus: d.stateStatus,
			PkgName:     d.parserDash.PkgName(),
		},
		New: DiffDashboardValues{
			Name:   d.parserDash.Name(),
			Desc:   d.parserDash.Description,
			Charts: make([]DiffChart, 0, len(d.parserDash.Charts)),
		},
	}

	for _, c := range d.parserDash.Charts {
		diff.New.Charts = append(diff.New.Charts, DiffChart{
			Properties: c.properties(),
			Height:     c.Height,
			Width:      c.Width,
		})
	}

	if d.existing == nil {
		return diff
	}

	oldDiff := DiffDashboardValues{
		Name:   d.existing.Name,
		Desc:   d.existing.Description,
		Charts: make([]DiffChart, 0, len(d.existing.Cells)),
	}

	for _, c := range d.existing.Cells {
		var props influxdb.ViewProperties
		if c.View != nil {
			props = c.View.Properties
		}

		oldDiff.Charts = append(oldDiff.Charts, DiffChart{
			Properties: props,
			XPosition:  int(c.X),
			YPosition:  int(c.Y),
			Height:     int(c.H),
			Width:      int(c.W),
		})
	}

	diff.Old = &oldDiff

	return diff
}

func (d *stateDashboard) summarize() SummaryDashboard {
	sum := d.parserDash.summarize()
	sum.ID = SafeID(d.ID())
	sum.OrgID = SafeID(d.orgID)
	return sum
}

type stateLabel struct {
	id, orgID   influxdb.ID
	stateStatus StateStatus

	parserLabel *label
	existing    *influxdb.Label
}

func (l *stateLabel) diffLabel() DiffLabel {
	diff := DiffLabel{
		DiffIdentifier: DiffIdentifier{
			ID: SafeID(l.ID()),
			// TODO: axe Remove field when StateStatus is adopted
			Remove:      IsRemoval(l.stateStatus),
			StateStatus: l.stateStatus,
			PkgName:     l.parserLabel.PkgName(),
		},
		New: DiffLabelValues{
			Name:        l.parserLabel.Name(),
			Description: l.parserLabel.Description,
			Color:       l.parserLabel.Color,
		},
	}
	if e := l.existing; e != nil {
		diff.Old = &DiffLabelValues{
			Name:        e.Name,
			Description: e.Properties["description"],
			Color:       e.Properties["color"],
		}
	}
	return diff
}

func (l *stateLabel) summarize() SummaryLabel {
	sum := l.parserLabel.summarize()
	sum.ID = SafeID(l.ID())
	sum.OrgID = SafeID(l.orgID)
	return sum
}

func (l *stateLabel) ID() influxdb.ID {
	if !IsNew(l.stateStatus) && l.existing != nil {
		return l.existing.ID
	}
	return l.id
}

func (l *stateLabel) shouldApply() bool {
	return IsRemoval(l.stateStatus) ||
		l.existing == nil ||
		l.parserLabel.Description != l.existing.Properties["description"] ||
		l.parserLabel.Name() != l.existing.Name ||
		l.parserLabel.Color != l.existing.Properties["color"]
}

func (l *stateLabel) toInfluxLabel() influxdb.Label {
	return influxdb.Label{
		ID:         l.ID(),
		OrgID:      l.orgID,
		Name:       l.parserLabel.Name(),
		Properties: l.properties(),
	}
}

func (l *stateLabel) properties() map[string]string {
	return map[string]string{
		"color":       l.parserLabel.Color,
		"description": l.parserLabel.Description,
	}
}

type stateLabelMapping struct {
	status StateStatus

	resource interface {
		stateIdentity() stateIdentity
	}

	label *stateLabel
}

func (lm stateLabelMapping) diffLabelMapping() DiffLabelMapping {
	ident := lm.resource.stateIdentity()
	return DiffLabelMapping{
		StateStatus:  lm.status,
		ResType:      ident.resourceType,
		ResID:        SafeID(ident.id),
		ResPkgName:   ident.pkgName,
		ResName:      ident.name,
		LabelID:      SafeID(lm.label.ID()),
		LabelPkgName: lm.label.parserLabel.PkgName(),
		LabelName:    lm.label.parserLabel.Name(),
	}
}

func (lm stateLabelMapping) summarize() SummaryLabelMapping {
	ident := lm.resource.stateIdentity()
	return SummaryLabelMapping{
		Status:          lm.status,
		ResourceID:      SafeID(ident.id),
		ResourcePkgName: ident.pkgName,
		ResourceName:    ident.name,
		ResourceType:    ident.resourceType,
		LabelPkgName:    lm.label.parserLabel.PkgName(),
		LabelName:       lm.label.parserLabel.Name(),
		LabelID:         SafeID(lm.label.ID()),
	}
}

func stateLabelMappingToInfluxLabelMapping(mapping stateLabelMapping) influxdb.LabelMapping {
	ident := mapping.resource.stateIdentity()
	return influxdb.LabelMapping{
		LabelID:      mapping.label.ID(),
		ResourceID:   ident.id,
		ResourceType: ident.resourceType,
	}
}

type stateLabelMappingForRemoval struct {
	LabelID         influxdb.ID
	LabelPkgName    string
	ResourceID      influxdb.ID
	ResourcePkgName string
	ResourceType    influxdb.ResourceType
}

func (m *stateLabelMappingForRemoval) diffLabelMapping() DiffLabelMapping {
	return DiffLabelMapping{
		StateStatus:  StateStatusRemove,
		ResType:      m.ResourceType,
		ResID:        SafeID(m.ResourceID),
		ResPkgName:   m.ResourcePkgName,
		LabelID:      SafeID(m.LabelID),
		LabelPkgName: m.LabelPkgName,
	}
}

type stateEndpoint struct {
	id, orgID   influxdb.ID
	stateStatus StateStatus

	parserEndpoint *notificationEndpoint
	existing       influxdb.NotificationEndpoint
}

func (e *stateEndpoint) ID() influxdb.ID {
	if !IsNew(e.stateStatus) && e.existing != nil {
		return e.existing.GetID()
	}
	return e.id
}

func (e *stateEndpoint) diffEndpoint() DiffNotificationEndpoint {
	diff := DiffNotificationEndpoint{
		DiffIdentifier: DiffIdentifier{
			ID:          SafeID(e.ID()),
			Remove:      IsRemoval(e.stateStatus),
			StateStatus: e.stateStatus,
			PkgName:     e.parserEndpoint.PkgName(),
		},
	}
	if sum := e.summarize(); sum.NotificationEndpoint != nil {
		diff.New.NotificationEndpoint = sum.NotificationEndpoint
	}
	if e.existing != nil {
		diff.Old = &DiffNotificationEndpointValues{
			NotificationEndpoint: e.existing,
		}
	}
	return diff
}

func (e *stateEndpoint) labels() []*label {
	return e.parserEndpoint.labels
}

func (e *stateEndpoint) resourceType() influxdb.ResourceType {
	return KindNotificationEndpoint.ResourceType()
}

func (e *stateEndpoint) stateIdentity() stateIdentity {
	return stateIdentity{
		id:           e.ID(),
		name:         e.parserEndpoint.Name(),
		pkgName:      e.parserEndpoint.PkgName(),
		resourceType: e.resourceType(),
		stateStatus:  e.stateStatus,
	}
}

func (e *stateEndpoint) summarize() SummaryNotificationEndpoint {
	sum := e.parserEndpoint.summarize()
	if sum.NotificationEndpoint == nil {
		return sum
	}
	if e.ID() != 0 {
		sum.NotificationEndpoint.SetID(e.ID())
	}
	if e.orgID != 0 {
		sum.NotificationEndpoint.SetOrgID(e.orgID)
	}
	return sum
}

type stateRule struct {
	id, orgID   influxdb.ID
	stateStatus StateStatus

	associatedEndpoint *stateEndpoint

	parserRule *notificationRule
	existing   influxdb.NotificationRule
}

func (r *stateRule) ID() influxdb.ID {
	if !IsNew(r.stateStatus) && r.existing != nil {
		return r.existing.GetID()
	}
	return r.id
}

func (r *stateRule) endpointAssociation() StackResourceAssociation {
	if r.associatedEndpoint == nil {
		return StackResourceAssociation{}
	}
	return StackResourceAssociation{
		Kind:    KindNotificationEndpoint,
		PkgName: r.endpointPkgName(),
	}
}

func (r *stateRule) diffRule() DiffNotificationRule {
	sum := DiffNotificationRule{
		DiffIdentifier: DiffIdentifier{
			ID:      SafeID(r.ID()),
			Remove:  r.parserRule.shouldRemove,
			PkgName: r.parserRule.PkgName(),
		},
		New: DiffNotificationRuleValues{
			Name:            r.parserRule.Name(),
			Description:     r.parserRule.description,
			EndpointName:    r.endpointPkgName(),
			EndpointID:      SafeID(r.endpointID()),
			EndpointType:    r.endpointType(),
			Every:           r.parserRule.every.String(),
			Offset:          r.parserRule.offset.String(),
			MessageTemplate: r.parserRule.msgTemplate,
			StatusRules:     toSummaryStatusRules(r.parserRule.statusRules),
			TagRules:        toSummaryTagRules(r.parserRule.tagRules),
		},
	}

	if r.existing == nil {
		return sum
	}

	sum.Old = &DiffNotificationRuleValues{
		Name:         r.existing.GetName(),
		Description:  r.existing.GetDescription(),
		EndpointName: r.existing.GetName(),
		EndpointID:   SafeID(r.existing.GetEndpointID()),
		EndpointType: r.existing.Type(),
	}

	assignBase := func(b rule.Base) {
		if b.Every != nil {
			sum.Old.Every = b.Every.TimeDuration().String()
		}
		if b.Offset != nil {
			sum.Old.Offset = b.Offset.TimeDuration().String()
		}
		for _, tr := range b.TagRules {
			sum.Old.TagRules = append(sum.Old.TagRules, SummaryTagRule{
				Key:      tr.Key,
				Value:    tr.Value,
				Operator: tr.Operator.String(),
			})
		}
		for _, sr := range b.StatusRules {
			sRule := SummaryStatusRule{CurrentLevel: sr.CurrentLevel.String()}
			if sr.PreviousLevel != nil {
				sRule.PreviousLevel = sr.PreviousLevel.String()
			}
			sum.Old.StatusRules = append(sum.Old.StatusRules, sRule)
		}
	}

	switch p := r.existing.(type) {
	case *rule.HTTP:
		assignBase(p.Base)
	case *rule.Slack:
		assignBase(p.Base)
		sum.Old.MessageTemplate = p.MessageTemplate
	case *rule.PagerDuty:
		assignBase(p.Base)
		sum.Old.MessageTemplate = p.MessageTemplate
	}

	return sum
}

func (r *stateRule) endpointID() influxdb.ID {
	if r.associatedEndpoint != nil {
		return r.associatedEndpoint.ID()
	}
	return 0
}

func (r *stateRule) endpointPkgName() string {
	if r.associatedEndpoint != nil && r.associatedEndpoint.parserEndpoint != nil {
		return r.associatedEndpoint.parserEndpoint.PkgName()
	}
	return ""
}

func (r *stateRule) endpointType() string {
	if r.associatedEndpoint != nil {
		return r.associatedEndpoint.parserEndpoint.kind.String()
	}
	return ""
}

func (r *stateRule) labels() []*label {
	return r.parserRule.labels
}

func (r *stateRule) resourceType() influxdb.ResourceType {
	return KindNotificationRule.ResourceType()
}

func (r *stateRule) stateIdentity() stateIdentity {
	return stateIdentity{
		id:           r.ID(),
		name:         r.parserRule.Name(),
		pkgName:      r.parserRule.PkgName(),
		resourceType: r.resourceType(),
		stateStatus:  r.stateStatus,
	}
}

func (r *stateRule) summarize() SummaryNotificationRule {
	sum := r.parserRule.summarize()
	sum.ID = SafeID(r.id)
	sum.EndpointID = SafeID(r.associatedEndpoint.ID())
	sum.EndpointPkgName = r.associatedEndpoint.parserEndpoint.PkgName()
	sum.EndpointType = r.associatedEndpoint.parserEndpoint.kind.String()
	return sum
}

func (r *stateRule) toInfluxRule() influxdb.NotificationRule {
	influxRule := r.parserRule.toInfluxRule()
	if r.ID() > 0 {
		influxRule.SetID(r.ID())
	}
	if r.orgID > 0 {
		influxRule.SetOrgID(r.orgID)
	}
	switch e := influxRule.(type) {
	case *rule.HTTP:
		e.EndpointID = r.associatedEndpoint.ID()
	case *rule.PagerDuty:
		e.EndpointID = r.associatedEndpoint.ID()
	case *rule.Slack:
		e.EndpointID = r.associatedEndpoint.ID()
	}

	return influxRule
}

type stateTask struct {
	id, orgID   influxdb.ID
	stateStatus StateStatus

	parserTask *task
	existing   *influxdb.Task
}

func (t *stateTask) ID() influxdb.ID {
	if !IsNew(t.stateStatus) && t.existing != nil {
		return t.existing.ID
	}
	return t.id
}

func (t *stateTask) diffTask() DiffTask {
	diff := DiffTask{
		DiffIdentifier: DiffIdentifier{
			ID:      SafeID(t.ID()),
			Remove:  IsRemoval(t.stateStatus),
			PkgName: t.parserTask.PkgName(),
		},
		New: DiffTaskValues{
			Name:        t.parserTask.Name(),
			Cron:        t.parserTask.cron,
			Description: t.parserTask.description,
			Every:       durToStr(t.parserTask.every),
			Offset:      durToStr(t.parserTask.offset),
			Query:       t.parserTask.query,
			Status:      t.parserTask.Status(),
		},
	}

	if t.existing == nil {
		return diff
	}

	diff.Old = &DiffTaskValues{
		Name:        t.existing.Name,
		Cron:        t.existing.Cron,
		Description: t.existing.Description,
		Every:       t.existing.Every,
		Offset:      t.existing.Offset.String(),
		Query:       t.existing.Flux,
		Status:      influxdb.Status(t.existing.Status),
	}

	return diff
}

func (t *stateTask) labels() []*label {
	return t.parserTask.labels
}

func (t *stateTask) resourceType() influxdb.ResourceType {
	return influxdb.TasksResourceType
}

func (t *stateTask) stateIdentity() stateIdentity {
	return stateIdentity{
		id:           t.ID(),
		name:         t.parserTask.Name(),
		pkgName:      t.parserTask.PkgName(),
		resourceType: t.resourceType(),
		stateStatus:  t.stateStatus,
	}
}

func (t *stateTask) summarize() SummaryTask {
	sum := t.parserTask.summarize()
	sum.ID = SafeID(t.id)
	return sum
}

type stateTelegraf struct {
	id, orgID   influxdb.ID
	stateStatus StateStatus

	parserTelegraf *telegraf
	existing       *influxdb.TelegrafConfig
}

func (t *stateTelegraf) ID() influxdb.ID {
	if !IsNew(t.stateStatus) && t.existing != nil {
		return t.existing.ID
	}
	return t.id
}

func (t *stateTelegraf) diffTelegraf() DiffTelegraf {
	return DiffTelegraf{
		DiffIdentifier: DiffIdentifier{
			ID:      SafeID(t.ID()),
			Remove:  IsRemoval(t.stateStatus),
			PkgName: t.parserTelegraf.PkgName(),
		},
		New: t.parserTelegraf.config,
		Old: t.existing,
	}
}

func (t *stateTelegraf) labels() []*label {
	return t.parserTelegraf.labels
}

func (t *stateTelegraf) resourceType() influxdb.ResourceType {
	return influxdb.TelegrafsResourceType
}

func (t *stateTelegraf) stateIdentity() stateIdentity {
	return stateIdentity{
		id:           t.ID(),
		name:         t.parserTelegraf.Name(),
		pkgName:      t.parserTelegraf.PkgName(),
		resourceType: t.resourceType(),
		stateStatus:  t.stateStatus,
	}
}

func (t *stateTelegraf) summarize() SummaryTelegraf {
	sum := t.parserTelegraf.summarize()
	sum.TelegrafConfig.ID = t.id
	sum.TelegrafConfig.OrgID = t.orgID
	return sum
}

type stateVariable struct {
	id, orgID   influxdb.ID
	stateStatus StateStatus

	parserVar *variable
	existing  *influxdb.Variable
}

func (v *stateVariable) ID() influxdb.ID {
	if !IsNew(v.stateStatus) && v.existing != nil {
		return v.existing.ID
	}
	return v.id
}

func (v *stateVariable) diffVariable() DiffVariable {
	diff := DiffVariable{
		DiffIdentifier: DiffIdentifier{
			ID:          SafeID(v.ID()),
			Remove:      IsRemoval(v.stateStatus),
			StateStatus: v.stateStatus,
			PkgName:     v.parserVar.PkgName(),
		},
		New: DiffVariableValues{
			Name:        v.parserVar.Name(),
			Description: v.parserVar.Description,
			Args:        v.parserVar.influxVarArgs(),
		},
	}
	if iv := v.existing; iv != nil {
		diff.Old = &DiffVariableValues{
			Name:        iv.Name,
			Description: iv.Description,
			Args:        iv.Arguments,
		}
	}

	return diff
}

func (v *stateVariable) labels() []*label {
	return v.parserVar.labels
}

func (v *stateVariable) resourceType() influxdb.ResourceType {
	return KindVariable.ResourceType()
}

func (v *stateVariable) shouldApply() bool {
	return IsRemoval(v.stateStatus) ||
		v.existing == nil ||
		v.existing.Description != v.parserVar.Description ||
		v.existing.Arguments == nil ||
		!reflect.DeepEqual(v.existing.Arguments, v.parserVar.influxVarArgs())
}

func (v *stateVariable) stateIdentity() stateIdentity {
	return stateIdentity{
		id:           v.ID(),
		name:         v.parserVar.Name(),
		pkgName:      v.parserVar.PkgName(),
		resourceType: v.resourceType(),
		stateStatus:  v.stateStatus,
	}
}

func (v *stateVariable) summarize() SummaryVariable {
	sum := v.parserVar.summarize()
	sum.ID = SafeID(v.ID())
	sum.OrgID = SafeID(v.orgID)
	return sum
}

// IsNew identifies state status as new to the platform.
func IsNew(status StateStatus) bool {
	// defaulting zero value to identify as new
	return status == StateStatusNew || status == ""
}

// IsExisting identifies state status as existing in the platform.
func IsExisting(status StateStatus) bool {
	return status == StateStatusExists
}

// IsRemoval identifies state status as existing resource that will be removed
// from the platform.
func IsRemoval(status StateStatus) bool {
	return status == StateStatusRemove
}
