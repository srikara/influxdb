package pkger

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/influxdata/influxdb/v2"
	ierrors "github.com/influxdata/influxdb/v2/kit/errors"
	"github.com/influxdata/influxdb/v2/notification/rule"
	"github.com/influxdata/influxdb/v2/snowflake"
	"github.com/influxdata/influxdb/v2/task/options"
	"go.uber.org/zap"
)

// APIVersion marks the current APIVersion for influx packages.
const APIVersion = "influxdata.com/v2alpha1"

type (
	// Stack is an identifier for stateful application of a package(s). This stack
	// will map created resources from the pkg(s) to existing resources on the
	// platform. This stack is updated only after side effects of applying a pkg.
	// If the pkg is applied, and no changes are had, then the stack is not updated.
	Stack struct {
		ID          influxdb.ID     `json:"id"`
		OrgID       influxdb.ID     `json:"orgID"`
		Name        string          `json:"name"`
		Description string          `json:"description"`
		URLs        []string        `json:"urls"`
		Resources   []StackResource `json:"resources"`

		influxdb.CRUDLog
	}

	// StackResource is a record for an individual resource side effect genereated from
	// applying a pkg.
	StackResource struct {
		APIVersion   string                     `json:"apiVersion"`
		ID           influxdb.ID                `json:"resourceID"`
		Kind         Kind                       `json:"kind"`
		PkgName      string                     `json:"pkgName"`
		Associations []StackResourceAssociation `json:"associations"`
	}

	// StackResourceAssociation associates a stack resource with another stack resource.
	StackResourceAssociation struct {
		Kind    Kind   `json:"kind"`
		PkgName string `json:"pkgName"`
	}
)

const ResourceTypeStack influxdb.ResourceType = "stack"

// SVC is the packages service interface.
type SVC interface {
	InitStack(ctx context.Context, userID influxdb.ID, stack Stack) (Stack, error)
	CreatePkg(ctx context.Context, setters ...CreatePkgSetFn) (*Pkg, error)
	DryRun(ctx context.Context, orgID, userID influxdb.ID, pkg *Pkg, opts ...ApplyOptFn) (Summary, Diff, error)
	Apply(ctx context.Context, orgID, userID influxdb.ID, pkg *Pkg, opts ...ApplyOptFn) (Summary, Diff, error)
}

// SVCMiddleware is a service middleware func.
type SVCMiddleware func(SVC) SVC

type serviceOpt struct {
	logger *zap.Logger

	applyReqLimit int
	idGen         influxdb.IDGenerator
	timeGen       influxdb.TimeGenerator
	store         Store

	bucketSVC   influxdb.BucketService
	checkSVC    influxdb.CheckService
	dashSVC     influxdb.DashboardService
	labelSVC    influxdb.LabelService
	endpointSVC influxdb.NotificationEndpointService
	orgSVC      influxdb.OrganizationService
	ruleSVC     influxdb.NotificationRuleStore
	secretSVC   influxdb.SecretService
	taskSVC     influxdb.TaskService
	teleSVC     influxdb.TelegrafConfigStore
	varSVC      influxdb.VariableService
}

// ServiceSetterFn is a means of setting dependencies on the Service type.
type ServiceSetterFn func(opt *serviceOpt)

// WithLogger sets the logger for the service.
func WithLogger(log *zap.Logger) ServiceSetterFn {
	return func(o *serviceOpt) {
		o.logger = log
	}
}

// WithIDGenerator sets the id generator for the service.
func WithIDGenerator(idGen influxdb.IDGenerator) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.idGen = idGen
	}
}

// WithTimeGenerator sets the time generator for the service.
func WithTimeGenerator(timeGen influxdb.TimeGenerator) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.timeGen = timeGen
	}
}

// WithStore sets the store for the service.
func WithStore(store Store) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.store = store
	}
}

// WithBucketSVC sets the bucket service.
func WithBucketSVC(bktSVC influxdb.BucketService) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.bucketSVC = bktSVC
	}
}

// WithCheckSVC sets the check service.
func WithCheckSVC(checkSVC influxdb.CheckService) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.checkSVC = checkSVC
	}
}

// WithDashboardSVC sets the dashboard service.
func WithDashboardSVC(dashSVC influxdb.DashboardService) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.dashSVC = dashSVC
	}
}

// WithNotificationEndpointSVC sets the endpoint notification service.
func WithNotificationEndpointSVC(endpointSVC influxdb.NotificationEndpointService) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.endpointSVC = endpointSVC
	}
}

// WithNotificationRuleSVC sets the endpoint rule service.
func WithNotificationRuleSVC(ruleSVC influxdb.NotificationRuleStore) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.ruleSVC = ruleSVC
	}
}

// WithOrganizationService sets the organization service for the service.
func WithOrganizationService(orgSVC influxdb.OrganizationService) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.orgSVC = orgSVC
	}
}

// WithLabelSVC sets the label service.
func WithLabelSVC(labelSVC influxdb.LabelService) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.labelSVC = labelSVC
	}
}

// WithSecretSVC sets the secret service.
func WithSecretSVC(secretSVC influxdb.SecretService) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.secretSVC = secretSVC
	}
}

// WithTaskSVC sets the task service.
func WithTaskSVC(taskSVC influxdb.TaskService) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.taskSVC = taskSVC
	}
}

// WithTelegrafSVC sets the telegraf service.
func WithTelegrafSVC(telegrafSVC influxdb.TelegrafConfigStore) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.teleSVC = telegrafSVC
	}
}

// WithVariableSVC sets the variable service.
func WithVariableSVC(varSVC influxdb.VariableService) ServiceSetterFn {
	return func(opt *serviceOpt) {
		opt.varSVC = varSVC
	}
}

// Store is the storage behavior the Service depends on.
type Store interface {
	CreateStack(ctx context.Context, stack Stack) error
	ReadStackByID(ctx context.Context, id influxdb.ID) (Stack, error)
	UpdateStack(ctx context.Context, stack Stack) error
	DeleteStack(ctx context.Context, id influxdb.ID) error
}

// Service provides the pkger business logic including all the dependencies to make
// this resource sausage.
type Service struct {
	log *zap.Logger

	// internal dependencies
	applyReqLimit int
	idGen         influxdb.IDGenerator
	store         Store
	timeGen       influxdb.TimeGenerator

	// external service dependencies
	bucketSVC   influxdb.BucketService
	checkSVC    influxdb.CheckService
	dashSVC     influxdb.DashboardService
	labelSVC    influxdb.LabelService
	endpointSVC influxdb.NotificationEndpointService
	orgSVC      influxdb.OrganizationService
	ruleSVC     influxdb.NotificationRuleStore
	secretSVC   influxdb.SecretService
	taskSVC     influxdb.TaskService
	teleSVC     influxdb.TelegrafConfigStore
	varSVC      influxdb.VariableService
}

var _ SVC = (*Service)(nil)

// NewService is a constructor for a pkger Service.
func NewService(opts ...ServiceSetterFn) *Service {
	opt := &serviceOpt{
		logger:        zap.NewNop(),
		applyReqLimit: 5,
		idGen:         snowflake.NewDefaultIDGenerator(),
		timeGen:       influxdb.RealTimeGenerator{},
	}
	for _, o := range opts {
		o(opt)
	}

	return &Service{
		log: opt.logger,

		applyReqLimit: opt.applyReqLimit,
		idGen:         opt.idGen,
		store:         opt.store,
		timeGen:       opt.timeGen,

		bucketSVC:   opt.bucketSVC,
		checkSVC:    opt.checkSVC,
		labelSVC:    opt.labelSVC,
		dashSVC:     opt.dashSVC,
		endpointSVC: opt.endpointSVC,
		orgSVC:      opt.orgSVC,
		ruleSVC:     opt.ruleSVC,
		secretSVC:   opt.secretSVC,
		taskSVC:     opt.taskSVC,
		teleSVC:     opt.teleSVC,
		varSVC:      opt.varSVC,
	}
}

// InitStack will create a new stack for the given user and its given org. The stack can be created
// with urls that point to the location of packages that are included as part of the stack when
// it is applied.
func (s *Service) InitStack(ctx context.Context, userID influxdb.ID, stack Stack) (Stack, error) {
	if err := validURLs(stack.URLs); err != nil {
		return Stack{}, err
	}

	if _, err := s.orgSVC.FindOrganizationByID(ctx, stack.OrgID); err != nil {
		if influxdb.ErrorCode(err) == influxdb.ENotFound {
			msg := fmt.Sprintf("organization dependency does not exist for id[%q]", stack.OrgID.String())
			return Stack{}, toInfluxError(influxdb.EConflict, msg)
		}
		return Stack{}, internalErr(err)
	}

	stack.ID = s.idGen.ID()
	now := s.timeGen.Now()
	stack.CRUDLog = influxdb.CRUDLog{
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.CreateStack(ctx, stack); err != nil {
		return Stack{}, internalErr(err)
	}

	return stack, nil
}

type (
	// CreatePkgSetFn is a functional input for setting the pkg fields.
	CreatePkgSetFn func(opt *CreateOpt) error

	// CreateOpt are the options for creating a new package.
	CreateOpt struct {
		OrgIDs    []CreateByOrgIDOpt
		Resources []ResourceToClone
	}

	// CreateByOrgIDOpt identifies an org to export resources for and provides
	// multiple filtering options.
	CreateByOrgIDOpt struct {
		OrgID         influxdb.ID
		LabelNames    []string
		ResourceKinds []Kind
	}
)

// CreateWithExistingResources allows the create method to clone existing resources.
func CreateWithExistingResources(resources ...ResourceToClone) CreatePkgSetFn {
	return func(opt *CreateOpt) error {
		for _, r := range resources {
			if err := r.OK(); err != nil {
				return err
			}
		}
		opt.Resources = append(opt.Resources, resources...)
		return nil
	}
}

// CreateWithAllOrgResources allows the create method to clone all existing resources
// for the given organization.
func CreateWithAllOrgResources(orgIDOpt CreateByOrgIDOpt) CreatePkgSetFn {
	return func(opt *CreateOpt) error {
		if orgIDOpt.OrgID == 0 {
			return errors.New("orgID provided must not be zero")
		}
		for _, k := range orgIDOpt.ResourceKinds {
			if err := k.OK(); err != nil {
				return err
			}
		}
		opt.OrgIDs = append(opt.OrgIDs, orgIDOpt)
		return nil
	}
}

// CreatePkg will produce a pkg from the parameters provided.
func (s *Service) CreatePkg(ctx context.Context, setters ...CreatePkgSetFn) (*Pkg, error) {
	opt := new(CreateOpt)
	for _, setter := range setters {
		if err := setter(opt); err != nil {
			return nil, err
		}
	}

	exporter := newResourceExporter(s)

	for _, orgIDOpt := range opt.OrgIDs {
		resourcesToClone, err := s.cloneOrgResources(ctx, orgIDOpt.OrgID, orgIDOpt.ResourceKinds)
		if err != nil {
			return nil, internalErr(err)
		}

		if err := exporter.Export(ctx, resourcesToClone, orgIDOpt.LabelNames...); err != nil {
			return nil, internalErr(err)
		}
	}

	if err := exporter.Export(ctx, opt.Resources); err != nil {
		return nil, internalErr(err)
	}

	pkg := &Pkg{Objects: exporter.Objects()}
	if err := pkg.Validate(ValidWithoutResources()); err != nil {
		return nil, failedValidationErr(err)
	}

	return pkg, nil
}

func (s *Service) cloneOrgResources(ctx context.Context, orgID influxdb.ID, resourceKinds []Kind) ([]ResourceToClone, error) {
	var resources []ResourceToClone
	for _, resGen := range s.filterOrgResourceKinds(resourceKinds) {
		existingResources, err := resGen.cloneFn(ctx, orgID)
		if err != nil {
			return nil, ierrors.Wrap(err, "finding "+string(resGen.resType))
		}
		resources = append(resources, existingResources...)
	}

	return resources, nil
}

func (s *Service) cloneOrgBuckets(ctx context.Context, orgID influxdb.ID) ([]ResourceToClone, error) {
	buckets, _, err := s.bucketSVC.FindBuckets(ctx, influxdb.BucketFilter{
		OrganizationID: &orgID,
	})
	if err != nil {
		return nil, err
	}

	resources := make([]ResourceToClone, 0, len(buckets))
	for _, b := range buckets {
		if b.Type == influxdb.BucketTypeSystem {
			continue
		}
		resources = append(resources, ResourceToClone{
			Kind: KindBucket,
			ID:   b.ID,
		})
	}
	return resources, nil
}

func (s *Service) cloneOrgChecks(ctx context.Context, orgID influxdb.ID) ([]ResourceToClone, error) {
	checks, _, err := s.checkSVC.FindChecks(ctx, influxdb.CheckFilter{
		OrgID: &orgID,
	})
	if err != nil {
		return nil, err
	}

	resources := make([]ResourceToClone, 0, len(checks))
	for _, c := range checks {
		resources = append(resources, ResourceToClone{
			Kind: KindCheck,
			ID:   c.GetID(),
		})
	}
	return resources, nil
}

func (s *Service) cloneOrgDashboards(ctx context.Context, orgID influxdb.ID) ([]ResourceToClone, error) {
	dashs, _, err := s.dashSVC.FindDashboards(ctx, influxdb.DashboardFilter{
		OrganizationID: &orgID,
	}, influxdb.FindOptions{Limit: 100})
	if err != nil {
		return nil, err
	}

	resources := make([]ResourceToClone, 0, len(dashs))
	for _, d := range dashs {
		resources = append(resources, ResourceToClone{
			Kind: KindDashboard,
			ID:   d.ID,
		})
	}
	return resources, nil
}

func (s *Service) cloneOrgLabels(ctx context.Context, orgID influxdb.ID) ([]ResourceToClone, error) {
	labels, err := s.labelSVC.FindLabels(ctx, influxdb.LabelFilter{
		OrgID: &orgID,
	}, influxdb.FindOptions{Limit: 10000})
	if err != nil {
		return nil, ierrors.Wrap(err, "finding labels")
	}

	resources := make([]ResourceToClone, 0, len(labels))
	for _, l := range labels {
		resources = append(resources, ResourceToClone{
			Kind: KindLabel,
			ID:   l.ID,
		})
	}
	return resources, nil
}

func (s *Service) cloneOrgNotificationEndpoints(ctx context.Context, orgID influxdb.ID) ([]ResourceToClone, error) {
	endpoints, _, err := s.endpointSVC.FindNotificationEndpoints(ctx, influxdb.NotificationEndpointFilter{
		OrgID: &orgID,
	})
	if err != nil {
		return nil, err
	}

	resources := make([]ResourceToClone, 0, len(endpoints))
	for _, e := range endpoints {
		resources = append(resources, ResourceToClone{
			Kind: KindNotificationEndpoint,
			ID:   e.GetID(),
		})
	}
	return resources, nil
}

func (s *Service) cloneOrgNotificationRules(ctx context.Context, orgID influxdb.ID) ([]ResourceToClone, error) {
	rules, _, err := s.ruleSVC.FindNotificationRules(ctx, influxdb.NotificationRuleFilter{
		OrgID: &orgID,
	})
	if err != nil {
		return nil, err
	}

	resources := make([]ResourceToClone, 0, len(rules))
	for _, r := range rules {
		resources = append(resources, ResourceToClone{
			Kind: KindNotificationRule,
			ID:   r.GetID(),
		})
	}
	return resources, nil
}

func (s *Service) cloneOrgTasks(ctx context.Context, orgID influxdb.ID) ([]ResourceToClone, error) {
	tasks, _, err := s.taskSVC.FindTasks(ctx, influxdb.TaskFilter{OrganizationID: &orgID})
	if err != nil {
		return nil, err
	}

	if len(tasks) == 0 {
		return nil, nil
	}

	checks, _, err := s.checkSVC.FindChecks(ctx, influxdb.CheckFilter{
		OrgID: &orgID,
	})
	if err != nil {
		return nil, err
	}

	rules, _, err := s.ruleSVC.FindNotificationRules(ctx, influxdb.NotificationRuleFilter{
		OrgID: &orgID,
	})
	if err != nil {
		return nil, err
	}

	mTasks := make(map[influxdb.ID]*influxdb.Task)
	for i := range tasks {
		t := tasks[i]
		if t.Type != influxdb.TaskSystemType {
			continue
		}
		mTasks[t.ID] = t
	}
	for _, c := range checks {
		delete(mTasks, c.GetTaskID())
	}
	for _, r := range rules {
		delete(mTasks, r.GetTaskID())
	}

	resources := make([]ResourceToClone, 0, len(mTasks))
	for _, t := range mTasks {
		resources = append(resources, ResourceToClone{
			Kind: KindTask,
			ID:   t.ID,
		})
	}
	return resources, nil
}

func (s *Service) cloneOrgTelegrafs(ctx context.Context, orgID influxdb.ID) ([]ResourceToClone, error) {
	teles, _, err := s.teleSVC.FindTelegrafConfigs(ctx, influxdb.TelegrafConfigFilter{OrgID: &orgID})
	if err != nil {
		return nil, err
	}

	resources := make([]ResourceToClone, 0, len(teles))
	for _, t := range teles {
		resources = append(resources, ResourceToClone{
			Kind: KindTelegraf,
			ID:   t.ID,
		})
	}
	return resources, nil
}

func (s *Service) cloneOrgVariables(ctx context.Context, orgID influxdb.ID) ([]ResourceToClone, error) {
	vars, err := s.varSVC.FindVariables(ctx, influxdb.VariableFilter{
		OrganizationID: &orgID,
	}, influxdb.FindOptions{Limit: 10000})
	if err != nil {
		return nil, err
	}

	resources := make([]ResourceToClone, 0, len(vars))
	for _, v := range vars {
		resources = append(resources, ResourceToClone{
			Kind: KindVariable,
			ID:   v.ID,
		})
	}

	return resources, nil
}

type cloneResFn func(context.Context, influxdb.ID) ([]ResourceToClone, error)

func (s *Service) filterOrgResourceKinds(resourceKindFilters []Kind) []struct {
	resType influxdb.ResourceType
	cloneFn cloneResFn
} {
	mKinds := map[Kind]cloneResFn{
		KindBucket:               s.cloneOrgBuckets,
		KindCheck:                s.cloneOrgChecks,
		KindDashboard:            s.cloneOrgDashboards,
		KindLabel:                s.cloneOrgLabels,
		KindNotificationEndpoint: s.cloneOrgNotificationEndpoints,
		KindNotificationRule:     s.cloneOrgNotificationRules,
		KindTask:                 s.cloneOrgTasks,
		KindTelegraf:             s.cloneOrgTelegrafs,
		KindVariable:             s.cloneOrgVariables,
	}

	newResGen := func(resType influxdb.ResourceType, cloneFn cloneResFn) struct {
		resType influxdb.ResourceType
		cloneFn cloneResFn
	} {
		return struct {
			resType influxdb.ResourceType
			cloneFn cloneResFn
		}{
			resType: resType,
			cloneFn: cloneFn,
		}
	}

	var resourceTypeGens []struct {
		resType influxdb.ResourceType
		cloneFn cloneResFn
	}
	if len(resourceKindFilters) == 0 {
		for k, cloneFn := range mKinds {
			resourceTypeGens = append(resourceTypeGens, newResGen(k.ResourceType(), cloneFn))
		}
		return resourceTypeGens
	}

	seenKinds := make(map[Kind]bool)
	for _, k := range resourceKindFilters {
		cloneFn, ok := mKinds[k]
		if !ok || seenKinds[k] {
			continue
		}
		seenKinds[k] = true
		resourceTypeGens = append(resourceTypeGens, newResGen(k.ResourceType(), cloneFn))
	}

	return resourceTypeGens
}

// DryRun provides a dry run of the pkg application. The pkg will be marked verified
// for later calls to Apply. This func will be run on an Apply if it has not been run
// already.
func (s *Service) DryRun(ctx context.Context, orgID, userID influxdb.ID, pkg *Pkg, opts ...ApplyOptFn) (Summary, Diff, error) {
	state, err := s.dryRun(ctx, orgID, pkg, opts...)
	if err != nil {
		return Summary{}, Diff{}, err
	}
	return newSummaryFromStatePkg(state, pkg), state.diff(), nil
}

func (s *Service) dryRun(ctx context.Context, orgID influxdb.ID, pkg *Pkg, opts ...ApplyOptFn) (*stateCoordinator, error) {
	// so here's the deal, when we have issues with the parsing validation, we
	// continue to do the diff anyhow. any resource that does not have a name
	// will be skipped, and won't bleed into the dry run here. We can now return
	// a error (parseErr) and valid diff/summary.
	var parseErr error
	if !pkg.isParsed {
		err := pkg.Validate()
		if err != nil && !IsParseErr(err) {
			return nil, internalErr(err)
		}
		parseErr = err
	}

	opt := applyOptFromOptFns(opts...)

	if len(opt.EnvRefs) > 0 {
		err := pkg.applyEnvRefs(opt.EnvRefs)
		if err != nil && !IsParseErr(err) {
			return nil, internalErr(err)
		}
		parseErr = err
	}

	state := newStateCoordinator(pkg)

	if opt.StackID > 0 {
		if err := s.addStackState(ctx, opt.StackID, state); err != nil {
			return nil, internalErr(err)
		}
	}

	if err := s.dryRunSecrets(ctx, orgID, pkg); err != nil {
		return nil, err
	}

	s.dryRunBuckets(ctx, orgID, state.mBuckets)
	s.dryRunChecks(ctx, orgID, state.mChecks)
	s.dryRunDashboards(ctx, orgID, state.mDashboards)
	s.dryRunLabels(ctx, orgID, state.mLabels)
	s.dryRunTasks(ctx, orgID, state.mTasks)
	s.dryRunTelegrafConfigs(ctx, orgID, state.mTelegrafs)
	s.dryRunVariables(ctx, orgID, state.mVariables)
	err := s.dryRunNotificationEndpoints(ctx, orgID, state.mEndpoints)
	if err != nil {
		return nil, ierrors.Wrap(err, "failed to dry run notification endpoints")
	}

	err = s.dryRunNotificationRules(ctx, orgID, state.mRules, state.mEndpoints)
	if err != nil {
		return nil, err
	}

	stateLabelMappings, err := s.dryRunLabelMappings(ctx, state)
	if err != nil {
		return nil, err
	}
	state.labelMappings = stateLabelMappings

	return state, parseErr
}

func (s *Service) dryRunBuckets(ctx context.Context, orgID influxdb.ID, bkts map[string]*stateBucket) {
	for _, stateBkt := range bkts {
		stateBkt.orgID = orgID
		var existing *influxdb.Bucket
		if stateBkt.ID() != 0 {
			existing, _ = s.bucketSVC.FindBucketByID(ctx, stateBkt.ID())
		} else {
			existing, _ = s.bucketSVC.FindBucketByName(ctx, orgID, stateBkt.parserBkt.Name())
		}
		if IsNew(stateBkt.stateStatus) && existing != nil {
			stateBkt.stateStatus = StateStatusExists
		}
		stateBkt.existing = existing
	}
}

func (s *Service) dryRunChecks(ctx context.Context, orgID influxdb.ID, checks map[string]*stateCheck) {
	for _, c := range checks {
		c.orgID = orgID

		var existing influxdb.Check
		if c.ID() != 0 {
			existing, _ = s.checkSVC.FindCheckByID(ctx, c.ID())
		} else {
			name := c.parserCheck.Name()
			existing, _ = s.checkSVC.FindCheck(ctx, influxdb.CheckFilter{
				Name:  &name,
				OrgID: &orgID,
			})
		}
		if IsNew(c.stateStatus) && existing != nil {
			c.stateStatus = StateStatusExists
		}
		c.existing = existing
	}
}

func (s *Service) dryRunDashboards(ctx context.Context, orgID influxdb.ID, dashs map[string]*stateDashboard) {
	for _, stateDash := range dashs {
		stateDash.orgID = orgID
		var existing *influxdb.Dashboard
		if stateDash.ID() != 0 {
			existing, _ = s.dashSVC.FindDashboardByID(ctx, stateDash.ID())
		}
		if IsNew(stateDash.stateStatus) && existing != nil {
			stateDash.stateStatus = StateStatusExists
		}
		stateDash.existing = existing
	}
}

func (s *Service) dryRunLabels(ctx context.Context, orgID influxdb.ID, labels map[string]*stateLabel) {
	for _, pkgLabel := range labels {
		pkgLabel.orgID = orgID
		existingLabel, _ := s.findLabel(ctx, orgID, pkgLabel)
		if IsNew(pkgLabel.stateStatus) && existingLabel != nil {
			pkgLabel.stateStatus = StateStatusExists
		}
		pkgLabel.existing = existingLabel
	}
}

func (s *Service) dryRunNotificationEndpoints(ctx context.Context, orgID influxdb.ID, endpoints map[string]*stateEndpoint) error {
	existingEndpoints, _, err := s.endpointSVC.FindNotificationEndpoints(ctx, influxdb.NotificationEndpointFilter{
		OrgID: &orgID,
	}) // grab em all
	if err != nil {
		return internalErr(err)
	}

	mExistingByName := make(map[string]influxdb.NotificationEndpoint)
	mExistingByID := make(map[influxdb.ID]influxdb.NotificationEndpoint)
	for i := range existingEndpoints {
		e := existingEndpoints[i]
		mExistingByName[e.GetName()] = e
		mExistingByID[e.GetID()] = e
	}

	findEndpoint := func(e *stateEndpoint) influxdb.NotificationEndpoint {
		if iExisting, ok := mExistingByID[e.ID()]; ok {
			return iExisting
		}
		if iExisting, ok := mExistingByName[e.parserEndpoint.Name()]; ok {
			return iExisting
		}
		return nil
	}

	for _, newEndpoint := range endpoints {
		existing := findEndpoint(newEndpoint)
		if IsNew(newEndpoint.stateStatus) && existing != nil {
			newEndpoint.stateStatus = StateStatusExists
		}
		newEndpoint.existing = existing
	}

	return nil
}

func (s *Service) dryRunNotificationRules(ctx context.Context, orgID influxdb.ID, rules map[string]*stateRule, endpoints map[string]*stateEndpoint) error {
	for _, rule := range rules {
		rule.orgID = orgID
		var existing influxdb.NotificationRule
		if rule.ID() != 0 {
			existing, _ = s.ruleSVC.FindNotificationRuleByID(ctx, rule.ID())
		}
		rule.existing = existing
	}

	for _, r := range rules {
		if r.associatedEndpoint != nil {
			continue
		}

		e, ok := endpoints[r.parserRule.endpointPkgName()]
		if !IsRemoval(r.stateStatus) && !ok {
			err := fmt.Errorf("failed to find notification endpoint %q dependency for notification rule %q", r.parserRule.endpointName, r.parserRule.PkgName())
			return &influxdb.Error{
				Code: influxdb.EUnprocessableEntity,
				Err:  err,
			}
		}
		r.associatedEndpoint = e
	}

	return nil
}

func (s *Service) dryRunSecrets(ctx context.Context, orgID influxdb.ID, pkg *Pkg) error {
	pkgSecrets := pkg.mSecrets
	if len(pkgSecrets) == 0 {
		return nil
	}

	existingSecrets, err := s.secretSVC.GetSecretKeys(ctx, orgID)
	if err != nil {
		return &influxdb.Error{Code: influxdb.EInternal, Err: err}
	}

	for _, secret := range existingSecrets {
		pkgSecrets[secret] = true // marked true since it exists in the platform
	}

	return nil
}

func (s *Service) dryRunTasks(ctx context.Context, orgID influxdb.ID, tasks map[string]*stateTask) {
	for _, stateTask := range tasks {
		stateTask.orgID = orgID
		var existing *influxdb.Task
		if stateTask.ID() != 0 {
			existing, _ = s.taskSVC.FindTaskByID(ctx, stateTask.ID())
		}
		if IsNew(stateTask.stateStatus) && existing != nil {
			stateTask.stateStatus = StateStatusExists
		}
		stateTask.existing = existing
	}
}

func (s *Service) dryRunTelegrafConfigs(ctx context.Context, orgID influxdb.ID, teleConfigs map[string]*stateTelegraf) {
	for _, stateTele := range teleConfigs {
		stateTele.orgID = orgID
		var existing *influxdb.TelegrafConfig
		if stateTele.ID() != 0 {
			existing, _ = s.teleSVC.FindTelegrafConfigByID(ctx, stateTele.ID())
		}
		if IsNew(stateTele.stateStatus) && existing != nil {
			stateTele.stateStatus = StateStatusExists
		}
		stateTele.existing = existing
	}
}

func (s *Service) dryRunVariables(ctx context.Context, orgID influxdb.ID, vars map[string]*stateVariable) {
	existingVars, _ := s.getAllPlatformVariables(ctx, orgID)

	mIDs := make(map[influxdb.ID]*influxdb.Variable)
	mNames := make(map[string]*influxdb.Variable)
	for _, v := range existingVars {
		mIDs[v.ID] = v
		mNames[v.Name] = v
	}

	for _, v := range vars {
		var existing *influxdb.Variable
		if v.ID() != 0 {
			existing = mIDs[v.ID()]
		} else {
			existing = mNames[v.parserVar.Name()]
		}
		if IsNew(v.stateStatus) && existing != nil {
			v.stateStatus = StateStatusExists
		}
		v.existing = existing
	}
}

func (s *Service) dryRunLabelMappings(ctx context.Context, state *stateCoordinator) ([]stateLabelMapping, error) {
	stateLabelsByResName := make(map[string]*stateLabel)
	for _, l := range state.mLabels {
		if IsRemoval(l.stateStatus) {
			continue
		}
		stateLabelsByResName[l.parserLabel.Name()] = l
	}

	var mappings []stateLabelMapping
	for _, b := range state.mBuckets {
		if IsRemoval(b.stateStatus) {
			continue
		}
		mm, err := s.dryRunResourceLabelMapping(ctx, state, stateLabelsByResName, b)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mm...)
	}

	for _, c := range state.mChecks {
		if IsRemoval(c.stateStatus) {
			continue
		}
		mm, err := s.dryRunResourceLabelMapping(ctx, state, stateLabelsByResName, c)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mm...)
	}

	for _, d := range state.mDashboards {
		if IsRemoval(d.stateStatus) {
			continue
		}
		mm, err := s.dryRunResourceLabelMapping(ctx, state, stateLabelsByResName, d)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mm...)
	}

	for _, e := range state.mEndpoints {
		if IsRemoval(e.stateStatus) {
			continue
		}
		mm, err := s.dryRunResourceLabelMapping(ctx, state, stateLabelsByResName, e)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mm...)
	}

	for _, r := range state.mRules {
		if IsRemoval(r.stateStatus) {
			continue
		}
		mm, err := s.dryRunResourceLabelMapping(ctx, state, stateLabelsByResName, r)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mm...)
	}

	for _, t := range state.mTasks {
		if IsRemoval(t.stateStatus) {
			continue
		}
		mm, err := s.dryRunResourceLabelMapping(ctx, state, stateLabelsByResName, t)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mm...)
	}

	for _, t := range state.mTelegrafs {
		if IsRemoval(t.stateStatus) {
			continue
		}
		mm, err := s.dryRunResourceLabelMapping(ctx, state, stateLabelsByResName, t)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mm...)
	}

	for _, v := range state.mVariables {
		if IsRemoval(v.stateStatus) {
			continue
		}
		mm, err := s.dryRunResourceLabelMapping(ctx, state, stateLabelsByResName, v)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, mm...)
	}

	return mappings, nil
}

func (s *Service) dryRunResourceLabelMapping(ctx context.Context, state *stateCoordinator, stateLabelsByResName map[string]*stateLabel, associatedResource interface {
	labels() []*label
	stateIdentity() stateIdentity
}) ([]stateLabelMapping, error) {

	ident := associatedResource.stateIdentity()
	pkgResourceLabels := associatedResource.labels()

	var mappings []stateLabelMapping
	if !ident.exists() {
		for _, l := range pkgResourceLabels {
			mappings = append(mappings, stateLabelMapping{
				status:   StateStatusNew,
				resource: associatedResource,
				label:    state.getLabelByPkgName(l.PkgName()),
			})
		}
		return mappings, nil
	}

	existingLabels, err := s.labelSVC.FindResourceLabels(ctx, influxdb.LabelMappingFilter{
		ResourceID:   ident.id,
		ResourceType: ident.resourceType,
	})
	if err != nil {
		return nil, err
	}

	pkgLabels := labelSlcToMap(pkgResourceLabels)
	for _, l := range existingLabels {
		// if label is found in state then we track the mapping and mark it existing
		// otherwise we continue on
		delete(pkgLabels, l.Name)
		if sLabel, ok := stateLabelsByResName[l.Name]; ok {
			mappings = append(mappings, stateLabelMapping{
				status:   StateStatusExists,
				resource: associatedResource,
				label:    sLabel,
			})
		}
	}

	// now we add labels that do not exist
	for _, l := range pkgLabels {
		mappings = append(mappings, stateLabelMapping{
			status:   StateStatusNew,
			resource: associatedResource,
			label:    state.getLabelByPkgName(l.PkgName()),
		})
	}

	return mappings, nil
}

func (s *Service) addStackState(ctx context.Context, stackID influxdb.ID, state *stateCoordinator) error {
	stack, err := s.store.ReadStackByID(ctx, stackID)
	if err != nil {
		return ierrors.Wrap(internalErr(err), "reading stack")
	}

	state.addStackState(stack)
	return nil
}

// ApplyOpt is an option for applying a package.
type ApplyOpt struct {
	EnvRefs        map[string]string
	MissingSecrets map[string]string
	StackID        influxdb.ID
}

// ApplyOptFn updates the ApplyOpt per the functional option.
type ApplyOptFn func(opt *ApplyOpt)

// ApplyWithEnvRefs provides env refs to saturate the missing reference fields in the pkg.
func ApplyWithEnvRefs(envRefs map[string]string) ApplyOptFn {
	return func(o *ApplyOpt) {
		o.EnvRefs = envRefs
	}
}

// ApplyWithSecrets provides secrets to the platform that the pkg will need.
func ApplyWithSecrets(secrets map[string]string) ApplyOptFn {
	return func(o *ApplyOpt) {
		o.MissingSecrets = secrets
	}
}

// ApplyWithStackID associates the application of a pkg with a stack.
func ApplyWithStackID(stackID influxdb.ID) ApplyOptFn {
	return func(o *ApplyOpt) {
		o.StackID = stackID
	}
}

func applyOptFromOptFns(opts ...ApplyOptFn) ApplyOpt {
	var opt ApplyOpt
	for _, o := range opts {
		o(&opt)
	}
	return opt
}

// Apply will apply all the resources identified in the provided pkg. The entire pkg will be applied
// in its entirety. If a failure happens midway then the entire pkg will be rolled back to the state
// from before the pkg were applied.
func (s *Service) Apply(ctx context.Context, orgID, userID influxdb.ID, pkg *Pkg, opts ...ApplyOptFn) (sum Summary, diff Diff, e error) {
	if !pkg.isParsed {
		if err := pkg.Validate(); err != nil {
			return Summary{}, Diff{}, failedValidationErr(err)
		}
	}

	opt := applyOptFromOptFns(opts...)

	if err := pkg.applyEnvRefs(opt.EnvRefs); err != nil {
		return Summary{}, Diff{}, failedValidationErr(err)
	}

	state, err := s.dryRun(ctx, orgID, pkg, opts...)
	if err != nil {
		return Summary{}, Diff{}, err
	}

	defer func(stackID influxdb.ID) {
		if stackID == 0 {
			return
		}

		updateStackFn := s.updateStackAfterSuccess
		if e != nil {
			updateStackFn = s.updateStackAfterRollback
		}
		if err := updateStackFn(ctx, stackID, state); err != nil {
			s.log.Error("failed to update stack", zap.Error(err))
		}
	}(opt.StackID)

	coordinator := &rollbackCoordinator{sem: make(chan struct{}, s.applyReqLimit)}
	defer coordinator.rollback(s.log, &e, orgID)

	err = s.applyState(ctx, coordinator, orgID, userID, state, opt.MissingSecrets)
	if err != nil {
		return Summary{}, Diff{}, err
	}

	pkg.applySecrets(opt.MissingSecrets)

	return newSummaryFromStatePkg(state, pkg), state.diff(), err
}

func (s *Service) applyState(ctx context.Context, coordinator *rollbackCoordinator, orgID, userID influxdb.ID, state *stateCoordinator, missingSecrets map[string]string) (e error) {
	endpointApp, ruleApp, err := s.applyNotificationGenerator(ctx, userID, state.rules(), state.endpoints())
	if err != nil {
		return err
	}

	// each grouping here runs for its entirety, then returns an error that
	// is indicative of running all appliers provided. For instance, the labels
	// may have 1 variable fail and one of the buckets fails. The errors aggregate so
	// the caller will be informed of both the failed label variable the failed bucket.
	// the groupings here allow for steps to occur before exiting. The first step is
	// adding the dependencies, resources that are associated by other resources. Then the
	// primary resources. Here we get all the errors associated with them.
	// If those are all good, then we run the secondary(dependent) resources which
	// rely on the primary resources having been created.
	appliers := [][]applier{
		{
			// adds secrets that are referenced it the pkg, this allows user to
			// provide data that does not rest in the pkg.
			s.applySecrets(missingSecrets),
		},
		{
			// deps for primary resources
			s.applyLabels(ctx, state.labels()),
		},
		{
			// primary resources, can have relationships to labels
			s.applyVariables(ctx, state.variables()),
			s.applyBuckets(ctx, state.buckets()),
			s.applyChecks(ctx, state.checks()),
			s.applyDashboards(ctx, state.dashboards()),
			endpointApp,
			s.applyTasks(ctx, state.tasks()),
			s.applyTelegrafs(ctx, userID, state.telegrafConfigs()),
		},
	}

	for _, group := range appliers {
		if err := coordinator.runTilEnd(ctx, orgID, userID, group...); err != nil {
			return internalErr(err)
		}
	}

	// this has to be run after the above primary resources, because it relies on
	// notification endpoints already being applied.
	if err := coordinator.runTilEnd(ctx, orgID, userID, ruleApp); err != nil {
		return err
	}

	// secondary resources
	// this last grouping relies on the above 2 steps having completely successfully
	secondary := []applier{
		s.applyLabelMappings(ctx, state.labelMappings),
		s.removeLabelMappings(ctx, state.labelMappingsToRemove),
	}
	if err := coordinator.runTilEnd(ctx, orgID, userID, secondary...); err != nil {
		return internalErr(err)
	}

	return nil
}

func (s *Service) applyBuckets(ctx context.Context, buckets []*stateBucket) applier {
	const resource = "bucket"

	mutex := new(doMutex)
	rollbackBuckets := make([]*stateBucket, 0, len(buckets))

	createFn := func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody {
		var b *stateBucket
		mutex.Do(func() {
			buckets[i].orgID = orgID
			b = buckets[i]
		})
		if !b.shouldApply() {
			return nil
		}

		influxBucket, err := s.applyBucket(ctx, b)
		if err != nil {
			return &applyErrBody{
				name: b.parserBkt.PkgName(),
				msg:  err.Error(),
			}
		}

		mutex.Do(func() {
			buckets[i].id = influxBucket.ID
			rollbackBuckets = append(rollbackBuckets, buckets[i])
		})

		return nil
	}

	return applier{
		creater: creater{
			entries: len(buckets),
			fn:      createFn,
		},
		rollbacker: rollbacker{
			resource: resource,
			fn:       func(_ influxdb.ID) error { return s.rollbackBuckets(ctx, rollbackBuckets) },
		},
	}
}

func (s *Service) rollbackBuckets(ctx context.Context, buckets []*stateBucket) error {
	rollbackFn := func(b *stateBucket) error {
		var err error
		switch b.stateStatus {
		case StateStatusRemove:
			err = ierrors.Wrap(s.bucketSVC.CreateBucket(ctx, b.existing), "rolling back removed bucket")
		case StateStatusExists:
			rp := b.parserBkt.RetentionRules.RP()
			_, err = s.bucketSVC.UpdateBucket(ctx, b.ID(), influxdb.BucketUpdate{
				Description:     &b.parserBkt.Description,
				RetentionPeriod: &rp,
			})
			err = ierrors.Wrap(err, "rolling back existing bucket to previous state")
		default:
			err = ierrors.Wrap(s.bucketSVC.DeleteBucket(ctx, b.ID()), "rolling back new bucket")
		}
		return err
	}

	var errs []string
	for _, b := range buckets {
		if err := rollbackFn(b); err != nil {
			errs = append(errs, fmt.Sprintf("error for bucket[%q]: %s", b.ID(), err))
		}
	}

	if len(errs) > 0 {
		// TODO: fixup error
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (s *Service) applyBucket(ctx context.Context, b *stateBucket) (influxdb.Bucket, error) {
	switch b.stateStatus {
	case StateStatusRemove:
		if err := s.bucketSVC.DeleteBucket(ctx, b.ID()); err != nil {
			return influxdb.Bucket{}, fmt.Errorf("failed to delete bucket[%q]: %w", b.ID(), err)
		}
		return *b.existing, nil
	case StateStatusExists:
		rp := b.parserBkt.RetentionRules.RP()
		newName := b.parserBkt.Name()
		influxBucket, err := s.bucketSVC.UpdateBucket(ctx, b.ID(), influxdb.BucketUpdate{
			Description:     &b.parserBkt.Description,
			Name:            &newName,
			RetentionPeriod: &rp,
		})
		if err != nil {
			return influxdb.Bucket{}, fmt.Errorf("failed to updated bucket[%q]: %w", b.ID(), err)
		}
		return *influxBucket, nil
	default:
		rp := b.parserBkt.RetentionRules.RP()
		influxBucket := influxdb.Bucket{
			OrgID:           b.orgID,
			Description:     b.parserBkt.Description,
			Name:            b.parserBkt.Name(),
			RetentionPeriod: rp,
		}
		err := s.bucketSVC.CreateBucket(ctx, &influxBucket)
		if err != nil {
			return influxdb.Bucket{}, fmt.Errorf("failed to create bucket[%q]: %w", b.ID(), err)
		}
		return influxBucket, nil
	}
}

func (s *Service) applyChecks(ctx context.Context, checks []*stateCheck) applier {
	const resource = "check"

	mutex := new(doMutex)
	rollbackChecks := make([]*stateCheck, 0, len(checks))

	createFn := func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody {
		var c *stateCheck
		mutex.Do(func() {
			checks[i].orgID = orgID
			c = checks[i]
		})

		influxCheck, err := s.applyCheck(ctx, c, userID)
		if err != nil {
			return &applyErrBody{
				name: c.parserCheck.Name(),
				msg:  err.Error(),
			}
		}

		mutex.Do(func() {
			checks[i].id = influxCheck.GetID()
			rollbackChecks = append(rollbackChecks, checks[i])
		})

		return nil
	}

	return applier{
		creater: creater{
			entries: len(checks),
			fn:      createFn,
		},
		rollbacker: rollbacker{
			resource: resource,
			fn:       func(_ influxdb.ID) error { return s.rollbackChecks(ctx, rollbackChecks) },
		},
	}
}

func (s *Service) rollbackChecks(ctx context.Context, checks []*stateCheck) error {
	rollbackFn := func(c *stateCheck) error {
		var err error
		switch c.stateStatus {
		case StateStatusRemove:
			err = s.checkSVC.CreateCheck(
				ctx,
				influxdb.CheckCreate{
					Check:  c.existing,
					Status: c.parserCheck.Status(),
				},
				c.existing.GetOwnerID(),
			)
			c.id = c.existing.GetID()
		case StateStatusNew:
			err = s.checkSVC.DeleteCheck(ctx, c.ID())
		default:
			_, err = s.checkSVC.UpdateCheck(ctx, c.ID(), influxdb.CheckCreate{
				Check:  c.summarize().Check,
				Status: influxdb.Status(c.parserCheck.status),
			})
		}
		return err
	}

	var errs []string
	for _, c := range checks {
		if err := rollbackFn(c); err != nil {
			errs = append(errs, fmt.Sprintf("error for check[%q]: %s", c.ID(), err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func (s *Service) applyCheck(ctx context.Context, c *stateCheck, userID influxdb.ID) (influxdb.Check, error) {
	switch c.stateStatus {
	case StateStatusRemove:
		if err := s.checkSVC.DeleteCheck(ctx, c.ID()); err != nil {
			return nil, fmt.Errorf("failed to delete check[%q]: %w", c.ID(), err)
		}
		return c.existing, nil
	case StateStatusExists:
		influxCheck, err := s.checkSVC.UpdateCheck(ctx, c.ID(), influxdb.CheckCreate{
			Check:  c.summarize().Check,
			Status: c.parserCheck.Status(),
		})
		if err != nil {
			return nil, err
		}
		return influxCheck, nil
	default:
		checkStub := influxdb.CheckCreate{
			Check:  c.summarize().Check,
			Status: c.parserCheck.Status(),
		}
		err := s.checkSVC.CreateCheck(ctx, checkStub, userID)
		if err != nil {
			return nil, err
		}
		return checkStub.Check, nil
	}
}

func (s *Service) applyDashboards(ctx context.Context, dashboards []*stateDashboard) applier {
	const resource = "dashboard"

	mutex := new(doMutex)
	rollbackDashboards := make([]*stateDashboard, 0, len(dashboards))

	createFn := func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody {
		var d *stateDashboard
		mutex.Do(func() {
			dashboards[i].orgID = orgID
			d = dashboards[i]
		})

		influxBucket, err := s.applyDashboard(ctx, d)
		if err != nil {
			return &applyErrBody{
				name: d.parserDash.Name(),
				msg:  err.Error(),
			}
		}

		mutex.Do(func() {
			dashboards[i].id = influxBucket.ID
			rollbackDashboards = append(rollbackDashboards, dashboards[i])
		})
		return nil
	}

	return applier{
		creater: creater{
			entries: len(dashboards),
			fn:      createFn,
		},
		rollbacker: rollbacker{
			resource: resource,
			fn: func(_ influxdb.ID) error {
				return s.rollbackDashboards(ctx, rollbackDashboards)
			},
		},
	}
}

func (s *Service) applyDashboard(ctx context.Context, d *stateDashboard) (influxdb.Dashboard, error) {
	switch d.stateStatus {
	case StateStatusRemove:
		if err := s.dashSVC.DeleteDashboard(ctx, d.ID()); err != nil {
			return influxdb.Dashboard{}, fmt.Errorf("failed to delete dashboard[%q]: %w", d.ID(), err)
		}
		return *d.existing, nil
	case StateStatusExists:
		name := d.parserDash.Name()
		cells := convertChartsToCells(d.parserDash.Charts)
		dash, err := s.dashSVC.UpdateDashboard(ctx, d.ID(), influxdb.DashboardUpdate{
			Name:        &name,
			Description: &d.parserDash.Description,
			Cells:       &cells,
		})
		if err != nil {
			return influxdb.Dashboard{}, ierrors.Wrap(err, "failed to update dashboard")
		}
		return *dash, nil
	default:
		cells := convertChartsToCells(d.parserDash.Charts)
		influxDashboard := influxdb.Dashboard{
			OrganizationID: d.orgID,
			Description:    d.parserDash.Description,
			Name:           d.parserDash.Name(),
			Cells:          cells,
		}
		err := s.dashSVC.CreateDashboard(ctx, &influxDashboard)
		if err != nil {
			return influxdb.Dashboard{}, ierrors.Wrap(err, "failed to create dashboard")
		}
		return influxDashboard, nil
	}
}

func (s *Service) rollbackDashboards(ctx context.Context, dashs []*stateDashboard) error {
	rollbackFn := func(d *stateDashboard) error {
		var err error
		switch d.stateStatus {
		case StateStatusRemove:
			err = ierrors.Wrap(s.dashSVC.CreateDashboard(ctx, d.existing), "rolling back removed dashboard")
		case StateStatusExists:
			_, err := s.dashSVC.UpdateDashboard(ctx, d.ID(), influxdb.DashboardUpdate{
				Name:        &d.existing.Name,
				Description: &d.existing.Description,
				Cells:       &d.existing.Cells,
			})
			return ierrors.Wrap(err, "failed to update dashboard")
		default:
			err = ierrors.Wrap(s.dashSVC.DeleteDashboard(ctx, d.ID()), "rolling back new dashboard")
		}
		return err
	}

	var errs []string
	for _, d := range dashs {
		if err := rollbackFn(d); err != nil {
			errs = append(errs, fmt.Sprintf("error for dashboard[%q]: %s", d.ID(), err))
		}
	}

	if len(errs) > 0 {
		// TODO: fixup error
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func convertChartsToCells(ch []chart) []*influxdb.Cell {
	icells := make([]*influxdb.Cell, 0, len(ch))
	for _, c := range ch {
		icell := &influxdb.Cell{
			CellProperty: influxdb.CellProperty{
				X: int32(c.XPos),
				Y: int32(c.YPos),
				H: int32(c.Height),
				W: int32(c.Width),
			},
			View: &influxdb.View{
				ViewContents: influxdb.ViewContents{Name: c.Name},
				Properties:   c.properties(),
			},
		}
		icells = append(icells, icell)
	}
	return icells
}

func (s *Service) applyLabels(ctx context.Context, labels []*stateLabel) applier {
	const resource = "label"

	mutex := new(doMutex)
	rollBackLabels := make([]*stateLabel, 0, len(labels))

	createFn := func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody {
		var l *stateLabel
		mutex.Do(func() {
			labels[i].orgID = orgID
			l = labels[i]
		})
		if !l.shouldApply() {
			return nil
		}

		influxLabel, err := s.applyLabel(ctx, l)
		if err != nil {
			return &applyErrBody{
				name: l.parserLabel.PkgName(),
				msg:  err.Error(),
			}
		}

		mutex.Do(func() {
			labels[i].id = influxLabel.ID
			rollBackLabels = append(rollBackLabels, labels[i])
		})

		return nil
	}

	return applier{
		creater: creater{
			entries: len(labels),
			fn:      createFn,
		},
		rollbacker: rollbacker{
			resource: resource,
			fn:       func(_ influxdb.ID) error { return s.rollbackLabels(ctx, rollBackLabels) },
		},
	}
}

func (s *Service) rollbackLabels(ctx context.Context, labels []*stateLabel) error {
	rollbackFn := func(l *stateLabel) error {
		var err error
		switch l.stateStatus {
		case StateStatusRemove:
			err = s.labelSVC.CreateLabel(ctx, l.existing)
		case StateStatusExists:
			_, err = s.labelSVC.UpdateLabel(ctx, l.ID(), influxdb.LabelUpdate{
				Name:       l.parserLabel.Name(),
				Properties: l.existing.Properties,
			})
		default:
			err = s.labelSVC.DeleteLabel(ctx, l.ID())
		}
		return err
	}

	var errs []string
	for _, l := range labels {
		if err := rollbackFn(l); err != nil {
			errs = append(errs, fmt.Sprintf("error for label[%q]: %s", l.ID(), err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (s *Service) applyLabel(ctx context.Context, l *stateLabel) (influxdb.Label, error) {
	var (
		influxLabel *influxdb.Label
		err         error
	)
	switch l.stateStatus {
	case StateStatusRemove:
		influxLabel, err = l.existing, s.labelSVC.DeleteLabel(ctx, l.ID())
	case StateStatusExists:
		influxLabel, err = s.labelSVC.UpdateLabel(ctx, l.ID(), influxdb.LabelUpdate{
			Name:       l.parserLabel.Name(),
			Properties: l.properties(),
		})
		err = ierrors.Wrap(err, "updating")
	default:
		creatLabel := l.toInfluxLabel()
		influxLabel = &creatLabel
		err = ierrors.Wrap(s.labelSVC.CreateLabel(ctx, &creatLabel), "creating")
	}
	if err != nil || influxLabel == nil {
		return influxdb.Label{}, err
	}

	return *influxLabel, nil
}

func (s *Service) applyNotificationEndpoints(ctx context.Context, userID influxdb.ID, endpoints []*stateEndpoint) (applier, func(influxdb.ID) error) {
	mutex := new(doMutex)
	rollbackEndpoints := make([]*stateEndpoint, 0, len(endpoints))

	createFn := func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody {
		var endpoint *stateEndpoint
		mutex.Do(func() {
			endpoints[i].orgID = orgID
			endpoint = endpoints[i]
		})

		influxEndpoint, err := s.applyNotificationEndpoint(ctx, endpoint, userID)
		if err != nil {
			return &applyErrBody{
				name: endpoint.parserEndpoint.Name(),
				msg:  err.Error(),
			}
		}

		mutex.Do(func() {
			endpoints[i].id = influxEndpoint.GetID()
			for _, secret := range influxEndpoint.SecretFields() {
				switch {
				case strings.HasSuffix(secret.Key, "-routing-key"):
					if endpoints[i].parserEndpoint.routingKey == nil {
						endpoints[i].parserEndpoint.routingKey = new(references)
					}
					endpoints[i].parserEndpoint.routingKey.Secret = secret.Key
				case strings.HasSuffix(secret.Key, "-token"):
					if endpoints[i].parserEndpoint.token == nil {
						endpoints[i].parserEndpoint.token = new(references)
					}
					endpoints[i].parserEndpoint.token.Secret = secret.Key
				case strings.HasSuffix(secret.Key, "-username"):
					if endpoints[i].parserEndpoint.username == nil {
						endpoints[i].parserEndpoint.username = new(references)
					}
					endpoints[i].parserEndpoint.username.Secret = secret.Key
				case strings.HasSuffix(secret.Key, "-password"):
					if endpoints[i].parserEndpoint.password == nil {
						endpoints[i].parserEndpoint.password = new(references)
					}
					endpoints[i].parserEndpoint.password.Secret = secret.Key
				}
			}
			rollbackEndpoints = append(rollbackEndpoints, endpoints[i])
		})

		return nil
	}

	rollbackFn := func(_ influxdb.ID) error {
		return s.rollbackNotificationEndpoints(ctx, userID, rollbackEndpoints)
	}

	return applier{
		creater: creater{
			entries: len(endpoints),
			fn:      createFn,
		},
		rollbacker: rollbacker{
			fn: func(_ influxdb.ID) error {
				return nil
			},
		},
	}, rollbackFn
}

func (s *Service) applyNotificationEndpoint(ctx context.Context, e *stateEndpoint, userID influxdb.ID) (influxdb.NotificationEndpoint, error) {
	switch e.stateStatus {
	case StateStatusRemove:
		_, _, err := s.endpointSVC.DeleteNotificationEndpoint(ctx, e.ID())
		if err != nil {
			return nil, err
		}
		return e.existing, nil
	case StateStatusExists:
		// stub out userID since we're always using hte http client which will fill it in for us with the token
		// feels a bit broken that is required.
		// TODO: look into this userID requirement
		return s.endpointSVC.UpdateNotificationEndpoint(
			ctx,
			e.ID(),
			e.summarize().NotificationEndpoint,
			userID,
		)
	default:
		actual := e.summarize().NotificationEndpoint
		err := s.endpointSVC.CreateNotificationEndpoint(ctx, actual, userID)
		if err != nil {
			return nil, err
		}

		return actual, nil
	}
}

func (s *Service) rollbackNotificationEndpoints(ctx context.Context, userID influxdb.ID, endpoints []*stateEndpoint) error {
	rollbackFn := func(e *stateEndpoint) error {
		var err error
		switch e.stateStatus {
		case StateStatusRemove:
			err = s.endpointSVC.CreateNotificationEndpoint(ctx, e.existing, userID)
			err = ierrors.Wrap(err, "failed to rollback removed endpoint")
		case StateStatusExists:
			_, err = s.endpointSVC.UpdateNotificationEndpoint(ctx, e.ID(), e.existing, userID)
			err = ierrors.Wrap(err, "failed to rollback updated endpoint")
		default:
			_, _, err = s.endpointSVC.DeleteNotificationEndpoint(ctx, e.ID())
			err = ierrors.Wrap(err, "failed to rollback created endpoint")
		}
		return err
	}

	var errs []string
	for _, e := range endpoints {
		if err := rollbackFn(e); err != nil {
			errs = append(errs, fmt.Sprintf("error for notification endpoint[%q]: %s", e.ID(), err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func (s *Service) applyNotificationGenerator(ctx context.Context, userID influxdb.ID, rules []*stateRule, stateEndpoints []*stateEndpoint) (endpointApplier applier, ruleApplier applier, err error) {
	mEndpoints := make(map[string]*stateEndpoint)
	for _, e := range stateEndpoints {
		mEndpoints[e.parserEndpoint.PkgName()] = e
	}

	var errs applyErrs
	for _, r := range rules {
		if IsRemoval(r.stateStatus) {
			continue
		}
		v, ok := mEndpoints[r.endpointPkgName()]
		if !ok {
			errs = append(errs, &applyErrBody{
				name: r.parserRule.Name(),
				msg:  fmt.Sprintf("notification rule endpoint dependency does not exist; endpointName=%q", r.parserRule.associatedEndpoint.PkgName()),
			})
			continue
		}
		r.associatedEndpoint = v
	}

	err = errs.toError("notification_rules", "failed to find dependency")
	if err != nil {
		return applier{}, applier{}, err
	}

	endpointApp, endpointRollbackFn := s.applyNotificationEndpoints(ctx, userID, stateEndpoints)
	ruleApp, ruleRollbackFn := s.applyNotificationRules(ctx, userID, rules)

	// here we have to couple the endpoints to rules b/c of the dependency here when rolling back
	// a deleted endpoint and rule. This forces the endpoints to be rolled back first so the
	// reference for the rule has settled. The dependency has to be available before rolling back
	// notification rules.
	endpointApp.rollbacker = rollbacker{
		fn: func(orgID influxdb.ID) error {
			if err := endpointRollbackFn(orgID); err != nil {
				s.log.Error("failed to roll back endpoints", zap.Error(err))
			}
			return ruleRollbackFn(orgID)
		},
	}

	return endpointApp, ruleApp, nil
}

func (s *Service) applyNotificationRules(ctx context.Context, userID influxdb.ID, rules []*stateRule) (applier, func(influxdb.ID) error) {
	mutex := new(doMutex)
	rollbackEndpoints := make([]*stateRule, 0, len(rules))

	createFn := func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody {
		var rule *stateRule
		mutex.Do(func() {
			rules[i].orgID = orgID
			rule = rules[i]
		})

		influxRule, err := s.applyNotificationRule(ctx, rule, userID)
		if err != nil {
			return &applyErrBody{
				name: rule.parserRule.PkgName(),
				msg:  err.Error(),
			}
		}

		mutex.Do(func() {
			rules[i].id = influxRule.GetID()
			rollbackEndpoints = append(rollbackEndpoints, rules[i])
		})

		return nil
	}

	rollbackFn := func(_ influxdb.ID) error {
		return s.rollbackNotificationRules(ctx, userID, rollbackEndpoints)
	}

	return applier{
		creater: creater{
			entries: len(rules),
			fn:      createFn,
		},
		rollbacker: rollbacker{
			fn: func(_ influxdb.ID) error { return nil },
		},
	}, rollbackFn
}

func (s *Service) applyNotificationRule(ctx context.Context, r *stateRule, userID influxdb.ID) (influxdb.NotificationRule, error) {
	switch r.stateStatus {
	case StateStatusRemove:
		if err := s.ruleSVC.DeleteNotificationRule(ctx, r.ID()); err != nil {
			return nil, ierrors.Wrap(err, "failed to remove notification rule")
		}
		return r.existing, nil
	case StateStatusExists:
		ruleCreate := influxdb.NotificationRuleCreate{
			NotificationRule: r.toInfluxRule(),
			Status:           r.parserRule.Status(),
		}
		influxRule, err := s.ruleSVC.UpdateNotificationRule(ctx, r.ID(), ruleCreate, userID)
		if err != nil {
			return nil, ierrors.Wrap(err, "failed to update notification rule")
		}
		return influxRule, nil
	default:
		influxRule := influxdb.NotificationRuleCreate{
			NotificationRule: r.toInfluxRule(),
			Status:           r.parserRule.Status(),
		}
		err := s.ruleSVC.CreateNotificationRule(ctx, influxRule, userID)
		if err != nil {
			return nil, err
		}
		return influxRule.NotificationRule, nil
	}
}

func (s *Service) rollbackNotificationRules(ctx context.Context, userID influxdb.ID, rules []*stateRule) error {
	rollbackFn := func(r *stateRule) error {
		existingRuleFn := func(endpointID influxdb.ID) influxdb.NotificationRule {
			switch rr := r.existing.(type) {
			case *rule.HTTP:
				rr.EndpointID = endpointID
			case *rule.PagerDuty:
				rr.EndpointID = endpointID
			case *rule.Slack:
				rr.EndpointID = endpointID
			}
			return r.existing
		}

		// setting status to unknown b/c these resources for two reasons:
		//	1. we have no ability to find status via the Service, only to set it...
		//	2. we have no way of inspecting an existing rule and pulling status from it
		//	3. since this is a fallback condition, we set things to inactive as a user
		//		is likely to follow up this failure by fixing their pkg up then reapplying
		unknownStatus := influxdb.Inactive

		var err error
		switch r.stateStatus {
		case StateStatusRemove:
			if r.associatedEndpoint == nil {
				return internalErr(errors.New("failed to find endpoint dependency to rollback existing notification rule"))
			}
			influxRule := influxdb.NotificationRuleCreate{
				NotificationRule: existingRuleFn(r.endpointID()),
				Status:           unknownStatus,
			}
			err = s.ruleSVC.CreateNotificationRule(ctx, influxRule, userID)
			err = ierrors.Wrap(err, "failed to rollback created notification rule")
		case StateStatusExists:
			if r.associatedEndpoint == nil {
				return internalErr(errors.New("failed to find endpoint dependency to rollback existing notification rule"))
			}

			influxRule := influxdb.NotificationRuleCreate{
				NotificationRule: existingRuleFn(r.endpointID()),
				Status:           unknownStatus,
			}
			_, err = s.ruleSVC.UpdateNotificationRule(ctx, r.ID(), influxRule, r.existing.GetOwnerID())
			err = ierrors.Wrap(err, "failed to rollback updated notification rule")
		default:
			err = s.ruleSVC.DeleteNotificationRule(ctx, r.ID())
			err = ierrors.Wrap(err, "failed to rollback created notification rule")
		}
		return err
	}

	var errs []string
	for _, r := range rules {
		if err := rollbackFn(r); err != nil {
			errs = append(errs, fmt.Sprintf("error for notification rule[%q]: %s", r.ID(), err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (s *Service) applySecrets(secrets map[string]string) applier {
	const resource = "secrets"

	if len(secrets) == 0 {
		return applier{
			rollbacker: rollbacker{fn: func(orgID influxdb.ID) error { return nil }},
		}
	}

	mutex := new(doMutex)
	rollbackSecrets := make([]string, 0)

	createFn := func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody {
		err := s.secretSVC.PutSecrets(ctx, orgID, secrets)
		if err != nil {
			return &applyErrBody{name: "secrets", msg: err.Error()}
		}

		mutex.Do(func() {
			for key := range secrets {
				rollbackSecrets = append(rollbackSecrets, key)
			}
		})

		return nil
	}

	return applier{
		creater: creater{
			entries: 1,
			fn:      createFn,
		},
		rollbacker: rollbacker{
			resource: resource,
			fn: func(orgID influxdb.ID) error {
				return s.secretSVC.DeleteSecret(context.Background(), orgID)
			},
		},
	}
}

func (s *Service) applyTasks(ctx context.Context, tasks []*stateTask) applier {
	const resource = "tasks"

	mutex := new(doMutex)
	rollbackTasks := make([]*stateTask, 0, len(tasks))

	createFn := func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody {
		var t *stateTask
		mutex.Do(func() {
			tasks[i].orgID = orgID
			t = tasks[i]
		})

		newTask, err := s.applyTask(ctx, userID, t)
		if err != nil {
			return &applyErrBody{
				name: t.parserTask.Name(),
				msg:  err.Error(),
			}
		}

		mutex.Do(func() {
			tasks[i].id = newTask.ID
			rollbackTasks = append(rollbackTasks, tasks[i])
		})

		return nil
	}

	return applier{
		creater: creater{
			entries: len(tasks),
			fn:      createFn,
		},
		rollbacker: rollbacker{
			resource: resource,
			fn: func(_ influxdb.ID) error {
				return s.rollbackTasks(ctx, rollbackTasks)
			},
		},
	}
}

func (s *Service) applyTask(ctx context.Context, userID influxdb.ID, t *stateTask) (influxdb.Task, error) {
	switch t.stateStatus {
	case StateStatusRemove:
		if err := s.taskSVC.DeleteTask(ctx, t.ID()); err != nil {
			return influxdb.Task{}, ierrors.Wrap(err, "failed to delete task")
		}
		return *t.existing, nil
	case StateStatusExists:
		newFlux := t.parserTask.flux()
		newStatus := string(t.parserTask.Status())
		opt := options.Options{
			Name: t.parserTask.Name(),
			Cron: t.parserTask.cron,
		}
		if every := t.parserTask.every; every > 0 {
			opt.Every.Parse(every.String())
		}
		if offset := t.parserTask.offset; offset > 0 {
			opt.Offset.Parse(offset.String())
		}

		updatedTask, err := s.taskSVC.UpdateTask(ctx, t.ID(), influxdb.TaskUpdate{
			Flux:        &newFlux,
			Status:      &newStatus,
			Description: &t.parserTask.description,
			Options:     opt,
		})
		if err != nil {
			return influxdb.Task{}, ierrors.Wrap(err, "failed to update task")
		}
		return *updatedTask, nil
	default:
		newTask, err := s.taskSVC.CreateTask(ctx, influxdb.TaskCreate{
			Type:           influxdb.TaskSystemType,
			Flux:           t.parserTask.flux(),
			OwnerID:        userID,
			Description:    t.parserTask.description,
			Status:         string(t.parserTask.Status()),
			OrganizationID: t.orgID,
		})
		if err != nil {
			return influxdb.Task{}, ierrors.Wrap(err, "failed to create task")
		}
		return *newTask, nil
	}
}

func (s *Service) rollbackTasks(ctx context.Context, tasks []*stateTask) error {
	rollbackFn := func(t *stateTask) error {
		var err error
		switch t.stateStatus {
		case StateStatusRemove:
			newTask, err := s.taskSVC.CreateTask(ctx, influxdb.TaskCreate{
				Type:           t.existing.Type,
				Flux:           t.existing.Flux,
				OwnerID:        t.existing.OwnerID,
				Description:    t.existing.Description,
				Status:         t.existing.Status,
				OrganizationID: t.orgID,
				Metadata:       t.existing.Metadata,
			})
			if err != nil {
				return ierrors.Wrap(err, "failed to rollback removed task")
			}
			t.existing = newTask
		case StateStatusExists:
			opt := options.Options{
				Name: t.existing.Name,
				Cron: t.existing.Cron,
			}
			if every := t.existing.Every; every != "" {
				opt.Every.Parse(every)
			}
			if offset := t.existing.Offset; offset > 0 {
				opt.Offset.Parse(offset.String())
			}

			_, err = s.taskSVC.UpdateTask(ctx, t.ID(), influxdb.TaskUpdate{
				Flux:        &t.existing.Flux,
				Status:      &t.existing.Status,
				Description: &t.existing.Description,
				Metadata:    t.existing.Metadata,
				Options:     opt,
			})
			err = ierrors.Wrap(err, "failed to rollback updated task")
		default:
			err = s.taskSVC.DeleteTask(ctx, t.ID())
			err = ierrors.Wrap(err, "failed to rollback created task")
		}
		return err
	}

	var errs []string
	for _, d := range tasks {
		if err := rollbackFn(d); err != nil {
			errs = append(errs, fmt.Sprintf("error for task[%q]: %s", d.ID(), err))
		}
	}

	if len(errs) > 0 {
		// TODO: fixup error
		return errors.New(strings.Join(errs, ", "))
	}

	return nil
}

func (s *Service) applyTelegrafs(ctx context.Context, userID influxdb.ID, teles []*stateTelegraf) applier {
	const resource = "telegrafs"

	mutex := new(doMutex)
	rollbackTelegrafs := make([]*stateTelegraf, 0, len(teles))

	createFn := func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody {
		var t *stateTelegraf
		mutex.Do(func() {
			teles[i].orgID = orgID
			t = teles[i]
		})

		existing, err := s.applyTelegrafConfig(ctx, userID, t)
		if err != nil {
			return &applyErrBody{
				name: t.parserTelegraf.Name(),
				msg:  err.Error(),
			}
		}

		mutex.Do(func() {
			teles[i].id = existing.ID
			rollbackTelegrafs = append(rollbackTelegrafs, teles[i])
		})

		return nil
	}

	return applier{
		creater: creater{
			entries: len(teles),
			fn:      createFn,
		},
		rollbacker: rollbacker{
			resource: resource,
			fn: func(_ influxdb.ID) error {
				return s.rollbackTelegrafConfigs(ctx, userID, rollbackTelegrafs)
			},
		},
	}
}

func (s *Service) applyTelegrafConfig(ctx context.Context, userID influxdb.ID, t *stateTelegraf) (influxdb.TelegrafConfig, error) {
	switch t.stateStatus {
	case StateStatusRemove:
		if err := s.teleSVC.DeleteTelegrafConfig(ctx, t.ID()); err != nil {
			return influxdb.TelegrafConfig{}, ierrors.Wrap(err, "failed to delete config")
		}
		return *t.existing, nil
	case StateStatusExists:
		cfg := t.summarize().TelegrafConfig
		updatedConfig, err := s.teleSVC.UpdateTelegrafConfig(ctx, t.ID(), &cfg, userID)
		if err != nil {
			return influxdb.TelegrafConfig{}, ierrors.Wrap(err, "failed to update config")
		}
		return *updatedConfig, nil
	default:
		cfg := t.summarize().TelegrafConfig
		err := s.teleSVC.CreateTelegrafConfig(ctx, &cfg, userID)
		if err != nil {
			return influxdb.TelegrafConfig{}, ierrors.Wrap(err, "failed to create telegraf config")
		}
		return cfg, nil
	}
}

func (s *Service) rollbackTelegrafConfigs(ctx context.Context, userID influxdb.ID, cfgs []*stateTelegraf) error {
	rollbackFn := func(t *stateTelegraf) error {
		var err error
		switch t.stateStatus {
		case StateStatusRemove:
			err = ierrors.Wrap(s.teleSVC.CreateTelegrafConfig(ctx, t.existing, userID), "rolling back removed telegraf config")
		case StateStatusExists:
			_, err = s.teleSVC.UpdateTelegrafConfig(ctx, t.ID(), t.existing, userID)
			err = ierrors.Wrap(err, "rolling back updated telegraf config")
		default:
			err = ierrors.Wrap(s.teleSVC.DeleteTelegrafConfig(ctx, t.ID()), "rolling back created telegraf config")
		}
		return err
	}

	var errs []string
	for _, v := range cfgs {
		if err := rollbackFn(v); err != nil {
			errs = append(errs, fmt.Sprintf("error for variable[%q]: %s", v.ID(), err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func (s *Service) applyVariables(ctx context.Context, vars []*stateVariable) applier {
	const resource = "variable"

	mutex := new(doMutex)
	rollBackVars := make([]*stateVariable, 0, len(vars))

	createFn := func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody {
		var v *stateVariable
		mutex.Do(func() {
			vars[i].orgID = orgID
			v = vars[i]
		})
		if !v.shouldApply() {
			return nil
		}
		influxVar, err := s.applyVariable(ctx, v)
		if err != nil {
			return &applyErrBody{
				name: v.parserVar.Name(),
				msg:  err.Error(),
			}
		}

		mutex.Do(func() {
			vars[i].id = influxVar.ID
			rollBackVars = append(rollBackVars, vars[i])
		})
		return nil
	}

	return applier{
		creater: creater{
			entries: len(vars),
			fn:      createFn,
		},
		rollbacker: rollbacker{
			resource: resource,
			fn:       func(_ influxdb.ID) error { return s.rollbackVariables(ctx, rollBackVars) },
		},
	}
}

func (s *Service) rollbackVariables(ctx context.Context, variables []*stateVariable) error {
	rollbackFn := func(v *stateVariable) error {
		var err error
		switch v.stateStatus {
		case StateStatusRemove:
			err = ierrors.Wrap(s.varSVC.CreateVariable(ctx, v.existing), "rolling back removed variable")
		case StateStatusExists:
			_, err = s.varSVC.UpdateVariable(ctx, v.ID(), &influxdb.VariableUpdate{
				Name:        v.parserVar.Name(),
				Description: v.parserVar.Description,
				Arguments:   v.parserVar.influxVarArgs(),
			})
			err = ierrors.Wrap(err, "rolling back updated variable")
		default:
			err = ierrors.Wrap(s.varSVC.DeleteVariable(ctx, v.ID()), "rolling back created variable")
		}
		return err
	}

	var errs []string
	for _, v := range variables {
		if err := rollbackFn(v); err != nil {
			errs = append(errs, fmt.Sprintf("error for variable[%q]: %s", v.ID(), err))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func (s *Service) applyVariable(ctx context.Context, v *stateVariable) (influxdb.Variable, error) {
	switch v.stateStatus {
	case StateStatusRemove:
		if err := s.varSVC.DeleteVariable(ctx, v.id); err != nil {
			return influxdb.Variable{}, err
		}
		return *v.existing, nil
	case StateStatusExists:
		updatedVar, err := s.varSVC.UpdateVariable(ctx, v.ID(), &influxdb.VariableUpdate{
			Name:        v.parserVar.Name(),
			Description: v.parserVar.Description,
			Arguments:   v.parserVar.influxVarArgs(),
		})
		if err != nil {
			return influxdb.Variable{}, err
		}
		return *updatedVar, nil
	default:
		influxVar := influxdb.Variable{
			OrganizationID: v.orgID,
			Name:           v.parserVar.Name(),
			Description:    v.parserVar.Description,
			Arguments:      v.parserVar.influxVarArgs(),
		}
		err := s.varSVC.CreateVariable(ctx, &influxVar)
		if err != nil {
			return influxdb.Variable{}, err
		}

		return influxVar, nil
	}
}

func (s *Service) removeLabelMappings(ctx context.Context, labelMappings []stateLabelMappingForRemoval) applier {
	const resource = "removed_label_mapping"

	var rollbackMappings []stateLabelMappingForRemoval

	mutex := new(doMutex)
	createFn := func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody {
		var mapping stateLabelMappingForRemoval
		mutex.Do(func() {
			mapping = labelMappings[i]
		})

		err := s.labelSVC.DeleteLabelMapping(ctx, &influxdb.LabelMapping{
			LabelID:      mapping.LabelID,
			ResourceID:   mapping.ResourceID,
			ResourceType: mapping.ResourceType,
		})
		if err != nil && influxdb.ErrorCode(err) != influxdb.ENotFound {
			return &applyErrBody{
				name: fmt.Sprintf("%s:%s:%s", mapping.ResourceType, mapping.ResourceID, mapping.LabelID),
				msg:  err.Error(),
			}
		}

		mutex.Do(func() {
			rollbackMappings = append(rollbackMappings, mapping)
		})
		return nil
	}

	return applier{
		creater: creater{
			entries: len(labelMappings),
			fn:      createFn,
		},
		rollbacker: rollbacker{
			resource: resource,
			fn:       func(_ influxdb.ID) error { return s.rollbackRemoveLabelMappings(ctx, rollbackMappings) },
		},
	}
}

func (s *Service) rollbackRemoveLabelMappings(ctx context.Context, mappings []stateLabelMappingForRemoval) error {
	var errs []string
	for _, m := range mappings {
		err := s.labelSVC.CreateLabelMapping(ctx, &influxdb.LabelMapping{
			LabelID:      m.LabelID,
			ResourceID:   m.ResourceID,
			ResourceType: m.ResourceType,
		})
		if err != nil {
			errs = append(errs,
				fmt.Sprintf(
					"error for label mapping: resource_type=%s resource_id=%s label_id=%s err=%s",
					m.ResourceType,
					m.ResourceID,
					m.LabelID,
					err,
				))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func (s *Service) applyLabelMappings(ctx context.Context, labelMappings []stateLabelMapping) applier {
	const resource = "label_mapping"

	mutex := new(doMutex)
	rollbackMappings := make([]stateLabelMapping, 0, len(labelMappings))

	createFn := func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody {
		var mapping stateLabelMapping
		mutex.Do(func() {
			mapping = labelMappings[i]
		})

		ident := mapping.resource.stateIdentity()
		if IsExisting(mapping.status) || mapping.label.ID() == 0 || ident.id == 0 {
			// this block here does 2 things, it does not write a
			// mapping when one exists. it also avoids having to worry
			// about deleting an existing mapping since it will not be
			// passed to the delete function below b/c it is never added
			// to the list of mappings that is referenced in the delete
			// call.
			return nil
		}

		m := influxdb.LabelMapping{
			LabelID:      mapping.label.ID(),
			ResourceID:   ident.id,
			ResourceType: ident.resourceType,
		}
		err := s.labelSVC.CreateLabelMapping(ctx, &m)
		if err != nil {
			return &applyErrBody{
				name: fmt.Sprintf("%s:%s:%s", ident.resourceType, ident.id, mapping.label.ID()),
				msg:  err.Error(),
			}
		}

		mutex.Do(func() {
			rollbackMappings = append(rollbackMappings, mapping)
		})

		return nil
	}

	return applier{
		creater: creater{
			entries: len(labelMappings),
			fn:      createFn,
		},
		rollbacker: rollbacker{
			resource: resource,
			fn:       func(_ influxdb.ID) error { return s.rollbackLabelMappings(ctx, rollbackMappings) },
		},
	}
}

func (s *Service) rollbackLabelMappings(ctx context.Context, mappings []stateLabelMapping) error {
	var errs []string
	for _, stateMapping := range mappings {
		influxMapping := stateLabelMappingToInfluxLabelMapping(stateMapping)
		err := s.labelSVC.DeleteLabelMapping(ctx, &influxMapping)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s:%s", stateMapping.label.ID(), stateMapping.resource.stateIdentity().id))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf(`label_resource_id_pairs=[%s] err="unable to delete label"`, strings.Join(errs, ", "))
	}

	return nil
}

func (s *Service) updateStackAfterSuccess(ctx context.Context, stackID influxdb.ID, state *stateCoordinator) error {
	stack, err := s.store.ReadStackByID(ctx, stackID)
	if err != nil {
		return err
	}

	var stackResources []StackResource
	for _, b := range state.mBuckets {
		if IsRemoval(b.stateStatus) {
			continue
		}
		associatedLabels := state.labelAssociations(KindBucket, b.parserBkt.PkgName())
		stackResources = append(stackResources, StackResource{
			APIVersion:   APIVersion,
			ID:           b.ID(),
			Kind:         KindBucket,
			PkgName:      b.parserBkt.PkgName(),
			Associations: stateLabelsToStackAssociations(associatedLabels),
		})
	}
	for _, c := range state.mChecks {
		if IsRemoval(c.stateStatus) {
			continue
		}
		associatedLabels := state.labelAssociations(KindCheck, c.parserCheck.PkgName())
		stackResources = append(stackResources, StackResource{
			APIVersion:   APIVersion,
			ID:           c.ID(),
			Kind:         KindCheck,
			PkgName:      c.parserCheck.PkgName(),
			Associations: stateLabelsToStackAssociations(associatedLabels),
		})
	}
	for _, d := range state.mDashboards {
		if IsRemoval(d.stateStatus) {
			continue
		}
		associatedLabels := state.labelAssociations(KindDashboard, d.parserDash.PkgName())
		stackResources = append(stackResources, StackResource{
			APIVersion:   APIVersion,
			ID:           d.ID(),
			Kind:         KindDashboard,
			PkgName:      d.parserDash.PkgName(),
			Associations: stateLabelsToStackAssociations(associatedLabels),
		})
	}
	for _, n := range state.mEndpoints {
		if IsRemoval(n.stateStatus) {
			continue
		}
		associatedLabels := state.labelAssociations(KindNotificationEndpoint, n.parserEndpoint.PkgName())
		stackResources = append(stackResources, StackResource{
			APIVersion:   APIVersion,
			ID:           n.ID(),
			Kind:         KindNotificationEndpoint,
			PkgName:      n.parserEndpoint.PkgName(),
			Associations: stateLabelsToStackAssociations(associatedLabels),
		})
	}
	for _, l := range state.mLabels {
		if IsRemoval(l.stateStatus) {
			continue
		}
		stackResources = append(stackResources, StackResource{
			APIVersion: APIVersion,
			ID:         l.ID(),
			Kind:       KindLabel,
			PkgName:    l.parserLabel.PkgName(),
		})
	}
	for _, r := range state.mRules {
		if IsRemoval(r.stateStatus) {
			continue
		}
		associatedLabels := state.labelAssociations(KindNotificationRule, r.parserRule.PkgName())
		stackResources = append(stackResources, StackResource{
			APIVersion: APIVersion,
			ID:         r.ID(),
			Kind:       KindNotificationRule,
			PkgName:    r.parserRule.PkgName(),
			Associations: append(
				stateLabelsToStackAssociations(associatedLabels),
				r.endpointAssociation(),
			),
		})
	}
	for _, t := range state.mTasks {
		if IsRemoval(t.stateStatus) {
			continue
		}
		associatedLabels := state.labelAssociations(KindTask, t.parserTask.PkgName())
		stackResources = append(stackResources, StackResource{
			APIVersion:   APIVersion,
			ID:           t.ID(),
			Kind:         KindTask,
			PkgName:      t.parserTask.PkgName(),
			Associations: stateLabelsToStackAssociations(associatedLabels),
		})
	}
	for _, t := range state.mTelegrafs {
		if IsRemoval(t.stateStatus) {
			continue
		}
		associatedLabels := state.labelAssociations(KindTelegraf, t.parserTelegraf.PkgName())
		stackResources = append(stackResources, StackResource{
			APIVersion:   APIVersion,
			ID:           t.ID(),
			Kind:         KindTelegraf,
			PkgName:      t.parserTelegraf.PkgName(),
			Associations: stateLabelsToStackAssociations(associatedLabels),
		})
	}
	for _, v := range state.mVariables {
		if IsRemoval(v.stateStatus) {
			continue
		}
		associatedLabels := state.labelAssociations(KindVariable, v.parserVar.PkgName())
		stackResources = append(stackResources, StackResource{
			APIVersion:   APIVersion,
			ID:           v.ID(),
			Kind:         KindVariable,
			PkgName:      v.parserVar.PkgName(),
			Associations: stateLabelsToStackAssociations(associatedLabels),
		})
	}
	stack.Resources = stackResources

	stack.UpdatedAt = time.Now()
	return s.store.UpdateStack(ctx, stack)
}

func (s *Service) updateStackAfterRollback(ctx context.Context, stackID influxdb.ID, state *stateCoordinator) error {
	stack, err := s.store.ReadStackByID(ctx, stackID)
	if err != nil {
		return err
	}

	type key struct {
		k       Kind
		pkgName string
	}
	newKey := func(k Kind, pkgName string) key {
		return key{k: k, pkgName: pkgName}
	}

	existingResources := make(map[key]*StackResource)
	for i := range stack.Resources {
		res := stack.Resources[i]
		existingResources[newKey(res.Kind, res.PkgName)] = &stack.Resources[i]
	}

	hasChanges := false
	{
		// these are the case where a deletion happens and is rolled back creating a new resource.
		// when resource is not to be removed this is a nothing burger, as it should be
		// rolled back to previous state.
		for _, b := range state.mBuckets {
			res, ok := existingResources[newKey(KindBucket, b.parserBkt.PkgName())]
			if ok && res.ID != b.ID() {
				hasChanges = true
				res.ID = b.existing.ID
			}
		}
		for _, c := range state.mChecks {
			res, ok := existingResources[newKey(KindCheck, c.parserCheck.PkgName())]
			if ok && res.ID != c.ID() {
				hasChanges = true
				res.ID = c.existing.GetID()
			}
		}
		for _, d := range state.mDashboards {
			res, ok := existingResources[newKey(KindDashboard, d.parserDash.PkgName())]
			if ok && res.ID != d.ID() {
				hasChanges = true
				res.ID = d.existing.ID
			}
		}
		for _, e := range state.mEndpoints {
			res, ok := existingResources[newKey(KindNotificationEndpoint, e.parserEndpoint.PkgName())]
			if ok && res.ID != e.ID() {
				hasChanges = true
				res.ID = e.existing.GetID()
			}
		}
		for _, l := range state.mLabels {
			res, ok := existingResources[newKey(KindLabel, l.parserLabel.PkgName())]
			if ok && res.ID != l.ID() {
				hasChanges = true
				res.ID = l.existing.ID
			}
		}
		for _, r := range state.mRules {
			res, ok := existingResources[newKey(KindNotificationRule, r.parserRule.PkgName())]
			if !ok {
				continue
			}

			if res.ID != r.ID() {
				hasChanges = true
				res.ID = r.existing.GetID()
			}

			endpointAssociation := r.endpointAssociation()
			newAss := make([]StackResourceAssociation, 0, len(res.Associations))

			var endpointAssociationChanged bool
			for _, ass := range res.Associations {
				if ass.Kind.is(KindNotificationEndpoint) && ass != endpointAssociation {
					endpointAssociationChanged = true
					ass = endpointAssociation
				}
				newAss = append(newAss, ass)
			}
			if endpointAssociationChanged {
				hasChanges = true
				res.Associations = newAss
			}
		}
		for _, t := range state.mTasks {
			res, ok := existingResources[newKey(KindTask, t.parserTask.PkgName())]
			if ok && res.ID != t.ID() {
				hasChanges = true
				res.ID = t.existing.ID
			}
		}
		for _, t := range state.mTelegrafs {
			res, ok := existingResources[newKey(KindTelegraf, t.parserTelegraf.PkgName())]
			if ok && res.ID != t.ID() {
				hasChanges = true
				res.ID = t.existing.ID
			}
		}
		for _, v := range state.mVariables {
			res, ok := existingResources[newKey(KindVariable, v.parserVar.PkgName())]
			if ok && res.ID != v.ID() {
				hasChanges = true
				res.ID = v.existing.ID
			}
		}
	}
	if !hasChanges {
		return nil
	}

	stack.UpdatedAt = time.Now()
	return s.store.UpdateStack(ctx, stack)
}

func (s *Service) findLabel(ctx context.Context, orgID influxdb.ID, l *stateLabel) (*influxdb.Label, error) {
	if l.ID() != 0 {
		return s.labelSVC.FindLabelByID(ctx, l.ID())
	}

	existingLabels, err := s.labelSVC.FindLabels(ctx, influxdb.LabelFilter{
		Name:  l.parserLabel.Name(),
		OrgID: &orgID,
	}, influxdb.FindOptions{Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(existingLabels) == 0 {
		return nil, errors.New("no labels found for name: " + l.parserLabel.Name())
	}
	return existingLabels[0], nil
}

func (s *Service) getAllPlatformVariables(ctx context.Context, orgID influxdb.ID) ([]*influxdb.Variable, error) {
	const limit = 100

	var (
		existingVars []*influxdb.Variable
		offset       int
	)
	for {
		vars, err := s.varSVC.FindVariables(ctx, influxdb.VariableFilter{
			OrganizationID: &orgID,
			// TODO: would be ideal to extend find variables to allow for a name matcher
			//  since names are unique for vars within an org. In the meanwhile, make large
			//  limit returned vars, should be more than enough for the time being.
		}, influxdb.FindOptions{Limit: limit, Offset: offset})
		if err != nil {
			return nil, err
		}
		existingVars = append(existingVars, vars...)

		if len(vars) < limit {
			break
		}
		offset += len(vars)
	}
	return existingVars, nil
}

func newSummaryFromStatePkg(state *stateCoordinator, pkg *Pkg) Summary {
	stateSum := state.summary()
	stateSum.MissingEnvs = pkg.missingEnvRefs()
	stateSum.MissingSecrets = pkg.missingSecrets()
	return stateSum
}

func stateLabelsToStackAssociations(stateLabels []*stateLabel) []StackResourceAssociation {
	var out []StackResourceAssociation
	for _, l := range stateLabels {
		out = append(out, StackResourceAssociation{
			Kind:    KindLabel,
			PkgName: l.parserLabel.PkgName(),
		})
	}
	return out
}

func getLabelIDMap(ctx context.Context, labelSVC influxdb.LabelService, labelNames []string) (map[influxdb.ID]bool, error) {
	mLabelIDs := make(map[influxdb.ID]bool)
	for _, labelName := range labelNames {
		iLabels, err := labelSVC.FindLabels(ctx, influxdb.LabelFilter{
			Name: labelName,
		})
		if err != nil {
			return nil, err
		}
		if len(iLabels) == 1 {
			mLabelIDs[iLabels[0].ID] = true
		}
	}
	return mLabelIDs, nil
}

type doMutex struct {
	sync.Mutex
}

func (m *doMutex) Do(fn func()) {
	m.Lock()
	defer m.Unlock()
	fn()
}

type (
	applier struct {
		creater    creater
		rollbacker rollbacker
	}

	rollbacker struct {
		resource string
		fn       func(orgID influxdb.ID) error
	}

	creater struct {
		entries int
		fn      func(ctx context.Context, i int, orgID, userID influxdb.ID) *applyErrBody
	}
)

type rollbackCoordinator struct {
	rollbacks []rollbacker

	sem chan struct{}
}

func (r *rollbackCoordinator) runTilEnd(ctx context.Context, orgID, userID influxdb.ID, appliers ...applier) error {
	errStr := newErrStream(ctx)

	wg := new(sync.WaitGroup)
	for i := range appliers {
		// cannot reuse the shared variable from for loop since we're using concurrency b/c
		// that temp var gets recycled between iterations
		app := appliers[i]
		r.rollbacks = append(r.rollbacks, app.rollbacker)
		for idx := range make([]struct{}, app.creater.entries) {
			r.sem <- struct{}{}
			wg.Add(1)

			go func(i int, resource string) {
				defer func() {
					wg.Done()
					<-r.sem
				}()

				ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				defer cancel()

				if err := app.creater.fn(ctx, i, orgID, userID); err != nil {
					errStr.add(errMsg{resource: resource, err: *err})
				}
			}(idx, app.rollbacker.resource)
		}
	}
	wg.Wait()

	return errStr.close()
}

func (r *rollbackCoordinator) rollback(l *zap.Logger, err *error, orgID influxdb.ID) {
	if *err == nil {
		return
	}

	for _, r := range r.rollbacks {
		if err := r.fn(orgID); err != nil {
			l.Error("failed to delete "+r.resource, zap.Error(err))
		}
	}
}

type errMsg struct {
	resource string
	err      applyErrBody
}

type errStream struct {
	msgStream chan errMsg
	err       chan error
	done      <-chan struct{}
}

func newErrStream(ctx context.Context) *errStream {
	e := &errStream{
		msgStream: make(chan errMsg),
		err:       make(chan error),
		done:      ctx.Done(),
	}
	e.do()
	return e
}

func (e *errStream) do() {
	go func() {
		mErrs := func() map[string]applyErrs {
			mErrs := make(map[string]applyErrs)
			for {
				select {
				case <-e.done:
					return nil
				case msg, ok := <-e.msgStream:
					if !ok {
						return mErrs
					}
					mErrs[msg.resource] = append(mErrs[msg.resource], &msg.err)
				}
			}
		}()

		if len(mErrs) == 0 {
			e.err <- nil
			return
		}

		var errs []string
		for resource, err := range mErrs {
			errs = append(errs, err.toError(resource, "failed to apply resource").Error())
		}
		e.err <- errors.New(strings.Join(errs, "\n"))
	}()
}

func (e *errStream) close() error {
	close(e.msgStream)
	return <-e.err
}

func (e *errStream) add(msg errMsg) {
	select {
	case <-e.done:
	case e.msgStream <- msg:
	}
}

// TODO: clean up apply errors to inform the user in an actionable way
type applyErrBody struct {
	name string
	msg  string
}

type applyErrs []*applyErrBody

func (a applyErrs) toError(resType, msg string) error {
	if len(a) == 0 {
		return nil
	}
	errMsg := fmt.Sprintf(`resource_type=%q err=%q`, resType, msg)
	for _, e := range a {
		errMsg += fmt.Sprintf("\n\tpkg_name=%q err_msg=%q", e.name, e.msg)
	}
	return errors.New(errMsg)
}

func validURLs(urls []string) error {
	for _, u := range urls {
		if _, err := url.Parse(u); err != nil {
			msg := fmt.Sprintf("url invalid for entry %q", u)
			return toInfluxError(influxdb.EInvalid, msg)
		}
	}
	return nil
}

func labelSlcToMap(labels []*label) map[string]*label {
	m := make(map[string]*label)
	for i := range labels {
		m[labels[i].Name()] = labels[i]
	}
	return m
}

func failedValidationErr(err error) error {
	if err == nil {
		return nil
	}
	return &influxdb.Error{Code: influxdb.EUnprocessableEntity, Err: err}
}

func internalErr(err error) error {
	if err == nil {
		return nil
	}
	return toInfluxError(influxdb.EInternal, err.Error())
}

func toInfluxError(code string, msg string) *influxdb.Error {
	return &influxdb.Error{
		Code: code,
		Msg:  msg,
	}
}
