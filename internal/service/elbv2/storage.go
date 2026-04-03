package elbv2

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sivchari/kumo/internal/storage"
)

const (
	defaultRegion    = "us-east-1"
	defaultAccountID = "000000000000"
)

// Storage defines the storage interface for ELB v2 service.
type Storage interface {
	CreateLoadBalancer(ctx context.Context, req *CreateLoadBalancerRequest) (*LoadBalancer, error)
	DeleteLoadBalancer(ctx context.Context, loadBalancerArn string) error
	DescribeLoadBalancers(ctx context.Context, arns, names []string) ([]*LoadBalancer, error)
	DescribeLoadBalancerAttributes(ctx context.Context, loadBalancerArn string) (map[string]string, error)
	ModifyLoadBalancerAttributes(ctx context.Context, loadBalancerArn string, attributes []Attribute) (map[string]string, error)
	DescribeTags(ctx context.Context, resourceArns []string) ([]*TagDescription, error)
	AddTags(ctx context.Context, resourceArns []string, tags []Tag) error
	RemoveTags(ctx context.Context, resourceArns []string, tagKeys []string) error

	CreateTargetGroup(ctx context.Context, req *CreateTargetGroupRequest) (*TargetGroup, error)
	DeleteTargetGroup(ctx context.Context, targetGroupArn string) error
	DescribeTargetGroups(ctx context.Context, arns, names []string, lbArn string) ([]*TargetGroup, error)
	DescribeTargetGroupAttributes(ctx context.Context, targetGroupArn string) (map[string]string, error)
	DescribeTargetHealth(ctx context.Context, targetGroupArn string, targets []Target) ([]*TargetHealthDescription, error)
	ModifyTargetGroupAttributes(ctx context.Context, targetGroupArn string, attributes []Attribute) (map[string]string, error)

	RegisterTargets(ctx context.Context, targetGroupArn string, targets []Target) error
	DeregisterTargets(ctx context.Context, targetGroupArn string, targets []Target) error

	CreateListener(ctx context.Context, req *CreateListenerRequest) (*Listener, error)
	CreateRule(ctx context.Context, req *CreateRuleRequest) (*Rule, error)
	DescribeListeners(ctx context.Context, listenerArns []string, loadBalancerArn string) ([]*Listener, error)
	DescribeRules(ctx context.Context, listenerArn string, ruleArns []string) ([]*Rule, error)
	DescribeListenerAttributes(ctx context.Context, listenerArn string) (map[string]string, error)
	DeleteListener(ctx context.Context, listenerArn string) error
}

// Option is a configuration option for MemoryStorage.
type Option func(*MemoryStorage)

// WithDataDir enables persistent storage in the specified directory.
func WithDataDir(dir string) Option {
	return func(s *MemoryStorage) {
		s.dataDir = dir
	}
}

// Compile-time interface checks.
var (
	_ json.Marshaler   = (*MemoryStorage)(nil)
	_ json.Unmarshaler = (*MemoryStorage)(nil)
)

// MemoryStorage is an in-memory implementation of Storage.
type MemoryStorage struct {
	mu            sync.RWMutex             `json:"-"`
	LoadBalancers map[string]*LoadBalancer `json:"loadBalancers"` // keyed by ARN
	TargetGroups  map[string]*TargetGroup  `json:"targetGroups"`  // keyed by ARN
	Listeners     map[string]*Listener     `json:"listeners"`     // keyed by ARN
	Rules         map[string]*Rule         `json:"rules"`         // keyed by ARN
	Targets       map[string][]Target      `json:"targets"`       // keyed by targetGroupArn
	dataDir       string
}

// NewMemoryStorage creates a new MemoryStorage.
func NewMemoryStorage(opts ...Option) *MemoryStorage {
	s := &MemoryStorage{
		LoadBalancers: make(map[string]*LoadBalancer),
		TargetGroups:  make(map[string]*TargetGroup),
		Listeners:     make(map[string]*Listener),
		Rules:         make(map[string]*Rule),
		Targets:       make(map[string][]Target),
	}
	for _, o := range opts {
		o(s)
	}

	if s.dataDir != "" {
		_ = storage.Load(s.dataDir, "elbv2", s)
	}

	return s
}

// MarshalJSON serializes the storage state to JSON.
func (m *MemoryStorage) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type Alias MemoryStorage

	data, err := json.Marshal(&struct{ *Alias }{Alias: (*Alias)(m)})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return data, nil
}

// UnmarshalJSON restores the storage state from JSON.
func (m *MemoryStorage) UnmarshalJSON(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	type Alias MemoryStorage

	aux := &struct{ *Alias }{Alias: (*Alias)(m)}

	if err := json.Unmarshal(data, aux); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	if m.LoadBalancers == nil {
		m.LoadBalancers = make(map[string]*LoadBalancer)
	}

	if m.TargetGroups == nil {
		m.TargetGroups = make(map[string]*TargetGroup)
	}

	if m.Listeners == nil {
		m.Listeners = make(map[string]*Listener)
	}

	if m.Rules == nil {
		m.Rules = make(map[string]*Rule)
	}

	if m.Targets == nil {
		m.Targets = make(map[string][]Target)
	}

	return nil
}

// Close saves the storage state to disk if persistence is enabled.
func (m *MemoryStorage) Close() error {
	if m.dataDir == "" {
		return nil
	}

	if err := storage.Save(m.dataDir, "elbv2", m); err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// loadBalancerDefaults holds default values for load balancer creation.
type loadBalancerDefaults struct {
	lbType        string
	scheme        string
	ipAddressType string
}

// getLoadBalancerDefaults returns default values for load balancer fields.
func getLoadBalancerDefaults(req *CreateLoadBalancerRequest) loadBalancerDefaults {
	defaults := loadBalancerDefaults{
		lbType:        req.Type,
		scheme:        req.Scheme,
		ipAddressType: req.IPAddressType,
	}

	if defaults.lbType == "" {
		defaults.lbType = "application"
	}

	if defaults.scheme == "" {
		defaults.scheme = "internet-facing"
	}

	if defaults.ipAddressType == "" {
		defaults.ipAddressType = "ipv4"
	}

	return defaults
}

// CreateLoadBalancer creates a new load balancer.
func (m *MemoryStorage) CreateLoadBalancer(_ context.Context, req *CreateLoadBalancerRequest) (*LoadBalancer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkDuplicateLoadBalancerName(req.Name); err != nil {
		return nil, err
	}

	defaults := getLoadBalancerDefaults(req)
	lb := m.buildLoadBalancer(req, defaults)
	m.LoadBalancers[lb.LoadBalancerArn] = lb

	return lb, nil
}

// checkDuplicateLoadBalancerName checks if a load balancer with the given name already exists.
func (m *MemoryStorage) checkDuplicateLoadBalancerName(name string) error {
	for _, lb := range m.LoadBalancers {
		if lb.LoadBalancerName == name {
			return &Error{
				Code:    "DuplicateLoadBalancerName",
				Message: fmt.Sprintf("A load balancer with the name '%s' already exists", name),
			}
		}
	}

	return nil
}

// buildLoadBalancer constructs a LoadBalancer from request and defaults.
func (m *MemoryStorage) buildLoadBalancer(req *CreateLoadBalancerRequest, defaults loadBalancerDefaults) *LoadBalancer {
	lbID := uuid.New().String()[:17]
	arn := fmt.Sprintf("arn:aws:elasticloadbalancing:%s:%s:loadbalancer/%s/%s/%s",
		defaultRegion, defaultAccountID, defaults.lbType[:3], req.Name, lbID)
	dnsName := fmt.Sprintf("%s-%s.%s.elb.amazonaws.com", req.Name, lbID[:8], defaultRegion)

	azs := make([]AvailabilityZone, 0, len(req.Subnets))
	for i, subnet := range req.Subnets {
		azs = append(azs, AvailabilityZone{
			ZoneName: fmt.Sprintf("%s%c", defaultRegion, 'a'+byte(i%3)),
			SubnetID: subnet,
		})
	}

	return &LoadBalancer{
		LoadBalancerArn:       arn,
		DNSName:               dnsName,
		CanonicalHostedZoneID: "Z35SXDOTRQ7X7K",
		CreatedTime:           time.Now(),
		LoadBalancerName:      req.Name,
		Scheme:                defaults.scheme,
		VpcID:                 "vpc-" + uuid.New().String()[:8],
		State:                 LoadBalancerState{Code: "active"},
		Type:                  defaults.lbType,
		AvailabilityZones:     azs,
		SecurityGroups:        req.SecurityGroups,
		IPAddressType:         defaults.ipAddressType,
		Tags:                  append([]Tag(nil), req.Tags...),
		Attributes:            map[string]string{},
	}
}

// DeleteLoadBalancer deletes a load balancer.
func (m *MemoryStorage) DeleteLoadBalancer(_ context.Context, loadBalancerArn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.LoadBalancers[loadBalancerArn]; !ok {
		return &Error{
			Code:    "LoadBalancerNotFound",
			Message: fmt.Sprintf("Load balancer '%s' not found", loadBalancerArn),
		}
	}

	// Delete associated listeners.
	for arn, listener := range m.Listeners {
		if listener.LoadBalancerArn == loadBalancerArn {
			for ruleArn, rule := range m.Rules {
				if rule.ListenerArn == arn {
					delete(m.Rules, ruleArn)
				}
			}
			delete(m.Listeners, arn)
		}
	}

	delete(m.LoadBalancers, loadBalancerArn)

	return nil
}

// DescribeLoadBalancers describes load balancers.
func (m *MemoryStorage) DescribeLoadBalancers(_ context.Context, arns, names []string) ([]*LoadBalancer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*LoadBalancer, 0)

	if len(arns) == 0 && len(names) == 0 {
		// Return all load balancers.
		for _, lb := range m.LoadBalancers {
			result = append(result, lb)
		}

		return result, nil
	}

	// Filter by ARNs.
	arnSet := make(map[string]bool)
	for _, arn := range arns {
		arnSet[arn] = true
	}

	// Filter by names.
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	for _, lb := range m.LoadBalancers {
		if len(arns) > 0 && arnSet[lb.LoadBalancerArn] {
			result = append(result, lb)

			continue
		}

		if len(names) > 0 && nameSet[lb.LoadBalancerName] {
			result = append(result, lb)
		}
	}

	return result, nil
}

// DescribeLoadBalancerAttributes describes load balancer attributes.
func (m *MemoryStorage) DescribeLoadBalancerAttributes(_ context.Context, loadBalancerArn string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lb, ok := m.LoadBalancers[loadBalancerArn]
	if !ok {
		return nil, &Error{
			Code:    "LoadBalancerNotFound",
			Message: fmt.Sprintf("Load balancer '%s' not found", loadBalancerArn),
		}
	}

	return cloneAttributes(lb.Attributes), nil
}

// ModifyLoadBalancerAttributes modifies load balancer attributes.
func (m *MemoryStorage) ModifyLoadBalancerAttributes(_ context.Context, loadBalancerArn string, attributes []Attribute) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.LoadBalancers[loadBalancerArn]
	if !ok {
		return nil, &Error{
			Code:    "LoadBalancerNotFound",
			Message: fmt.Sprintf("Load balancer '%s' not found", loadBalancerArn),
		}
	}

	if lb.Attributes == nil {
		lb.Attributes = make(map[string]string)
	}

	for _, attribute := range attributes {
		lb.Attributes[attribute.Key] = attribute.Value
	}

	return cloneAttributes(lb.Attributes), nil
}

// DescribeTags describes tags for ELBv2 resources.
func (m *MemoryStorage) DescribeTags(_ context.Context, resourceArns []string) ([]*TagDescription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	descriptions := make([]*TagDescription, 0, len(resourceArns))
	for _, resourceArn := range resourceArns {
		tags, ok := m.getResourceTagsLocked(resourceArn)
		if !ok {
			continue
		}

		descriptions = append(descriptions, &TagDescription{
			ResourceArn: resourceArn,
			Tags:        append([]Tag(nil), tags...),
		})
	}

	return descriptions, nil
}

// AddTags adds or updates tags for ELBv2 resources.
func (m *MemoryStorage) AddTags(_ context.Context, resourceArns []string, tags []Tag) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, resourceArn := range resourceArns {
		resourceTags, ok := m.getMutableResourceTagsLocked(resourceArn)
		if !ok {
			return &Error{
				Code:    "LoadBalancerNotFound",
				Message: fmt.Sprintf("Resource '%s' not found", resourceArn),
			}
		}

		for _, tag := range tags {
			upsertTag(resourceTags, tag)
		}
	}

	return nil
}

// RemoveTags removes tags from ELBv2 resources.
func (m *MemoryStorage) RemoveTags(_ context.Context, resourceArns []string, tagKeys []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, resourceArn := range resourceArns {
		resourceTags, ok := m.getMutableResourceTagsLocked(resourceArn)
		if !ok {
			return &Error{
				Code:    "LoadBalancerNotFound",
				Message: fmt.Sprintf("Resource '%s' not found", resourceArn),
			}
		}

		removeTags(resourceTags, tagKeys)
	}

	return nil
}

// targetGroupDefaults holds default values for target group creation.
type targetGroupDefaults struct {
	targetType          string
	healthCheckPort     string
	healthCheckProtocol string
	healthCheckPath     string
	healthCheckInterval int
	healthCheckTimeout  int
	healthyThreshold    int
	unhealthyThreshold  int
}

// getTargetGroupDefaults returns default values for target group fields.
func getTargetGroupDefaults(req *CreateTargetGroupRequest) targetGroupDefaults {
	defaults := targetGroupDefaults{
		targetType:          req.TargetType,
		healthCheckPort:     req.HealthCheckPort,
		healthCheckProtocol: req.HealthCheckProtocol,
		healthCheckPath:     req.HealthCheckPath,
		healthCheckInterval: req.HealthCheckIntervalSeconds,
		healthCheckTimeout:  req.HealthCheckTimeoutSeconds,
		healthyThreshold:    req.HealthyThresholdCount,
		unhealthyThreshold:  req.UnhealthyThresholdCount,
	}

	if defaults.targetType == "" {
		defaults.targetType = "instance"
	}

	if defaults.healthCheckPort == "" {
		defaults.healthCheckPort = "traffic-port"
	}

	if defaults.healthCheckProtocol == "" {
		defaults.healthCheckProtocol = req.Protocol
		if defaults.healthCheckProtocol == "" {
			defaults.healthCheckProtocol = "HTTP"
		}
	}

	if defaults.healthCheckPath == "" && (defaults.healthCheckProtocol == "HTTP" || defaults.healthCheckProtocol == "HTTPS") {
		defaults.healthCheckPath = "/"
	}

	if defaults.healthCheckInterval == 0 {
		defaults.healthCheckInterval = 30
	}

	if defaults.healthCheckTimeout == 0 {
		defaults.healthCheckTimeout = 5
	}

	if defaults.healthyThreshold == 0 {
		defaults.healthyThreshold = 5
	}

	if defaults.unhealthyThreshold == 0 {
		defaults.unhealthyThreshold = 2
	}

	return defaults
}

// CreateTargetGroup creates a new target group.
func (m *MemoryStorage) CreateTargetGroup(_ context.Context, req *CreateTargetGroupRequest) (*TargetGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.checkDuplicateTargetGroupName(req.Name); err != nil {
		return nil, err
	}

	defaults := getTargetGroupDefaults(req)
	tg := m.buildTargetGroup(req, &defaults)
	m.TargetGroups[tg.TargetGroupArn] = tg
	m.Targets[tg.TargetGroupArn] = []Target{}

	return tg, nil
}

// checkDuplicateTargetGroupName checks if a target group with the given name already exists.
func (m *MemoryStorage) checkDuplicateTargetGroupName(name string) error {
	for _, tg := range m.TargetGroups {
		if tg.TargetGroupName == name {
			return &Error{
				Code:    "DuplicateTargetGroupName",
				Message: fmt.Sprintf("A target group with the name '%s' already exists", name),
			}
		}
	}

	return nil
}

// buildTargetGroup constructs a TargetGroup from request and defaults.
func (m *MemoryStorage) buildTargetGroup(req *CreateTargetGroupRequest, defaults *targetGroupDefaults) *TargetGroup {
	tgID := uuid.New().String()[:17]
	arn := fmt.Sprintf("arn:aws:elasticloadbalancing:%s:%s:targetgroup/%s/%s",
		defaultRegion, defaultAccountID, req.Name, tgID)

	return &TargetGroup{
		TargetGroupArn:             arn,
		TargetGroupName:            req.Name,
		Protocol:                   req.Protocol,
		Port:                       req.Port,
		VpcID:                      req.VpcID,
		HealthCheckEnabled:         true,
		HealthCheckIntervalSeconds: defaults.healthCheckInterval,
		HealthCheckPath:            defaults.healthCheckPath,
		HealthCheckPort:            defaults.healthCheckPort,
		HealthCheckProtocol:        defaults.healthCheckProtocol,
		HealthCheckTimeoutSeconds:  defaults.healthCheckTimeout,
		HealthyThresholdCount:      defaults.healthyThreshold,
		UnhealthyThresholdCount:    defaults.unhealthyThreshold,
		TargetType:                 defaults.targetType,
		LoadBalancerArns:           []string{},
		Tags:                       append([]Tag(nil), req.Tags...),
		Attributes:                 map[string]string{},
	}
}

// DeleteTargetGroup deletes a target group.
func (m *MemoryStorage) DeleteTargetGroup(_ context.Context, targetGroupArn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.TargetGroups[targetGroupArn]; !ok {
		return &Error{
			Code:    "TargetGroupNotFound",
			Message: fmt.Sprintf("Target group '%s' not found", targetGroupArn),
		}
	}

	delete(m.TargetGroups, targetGroupArn)
	delete(m.Targets, targetGroupArn)

	return nil
}

// DescribeTargetGroups describes target groups.
func (m *MemoryStorage) DescribeTargetGroups(_ context.Context, arns, names []string, lbArn string) ([]*TargetGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TargetGroup, 0)

	if len(arns) == 0 && len(names) == 0 && lbArn == "" {
		// Return all target groups.
		for _, tg := range m.TargetGroups {
			result = append(result, tg)
		}

		return result, nil
	}

	// Filter by ARNs.
	arnSet := make(map[string]bool)
	for _, arn := range arns {
		arnSet[arn] = true
	}

	// Filter by names.
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	for _, tg := range m.TargetGroups {
		if len(arns) > 0 && arnSet[tg.TargetGroupArn] {
			result = append(result, tg)

			continue
		}

		if len(names) > 0 && nameSet[tg.TargetGroupName] {
			result = append(result, tg)

			continue
		}

		if lbArn != "" && slices.Contains(tg.LoadBalancerArns, lbArn) {
			result = append(result, tg)
		}
	}

	return result, nil
}

// DescribeTargetGroupAttributes describes target group attributes.
func (m *MemoryStorage) DescribeTargetGroupAttributes(_ context.Context, targetGroupArn string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tg, ok := m.TargetGroups[targetGroupArn]
	if !ok {
		return nil, &Error{
			Code:    "TargetGroupNotFound",
			Message: fmt.Sprintf("Target group '%s' not found", targetGroupArn),
		}
	}

	return cloneAttributes(tg.Attributes), nil
}

// DescribeTargetHealth describes target health.
func (m *MemoryStorage) DescribeTargetHealth(_ context.Context, targetGroupArn string, targets []Target) ([]*TargetHealthDescription, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.TargetGroups[targetGroupArn]; !ok {
		return nil, &Error{
			Code:    "TargetGroupNotFound",
			Message: fmt.Sprintf("Target group '%s' not found", targetGroupArn),
		}
	}

	existingTargets := m.Targets[targetGroupArn]
	if len(existingTargets) == 0 {
		return nil, nil
	}

	descriptions := make([]*TargetHealthDescription, 0, len(existingTargets))
	for _, target := range existingTargets {
		if !matchesDescribeTargetHealthTargets(target, targets) {
			continue
		}

		descriptions = append(descriptions, &TargetHealthDescription{
			Target: target,
			TargetHealth: TargetHealth{
				State: "healthy",
			},
		})
	}

	return descriptions, nil
}

// ModifyTargetGroupAttributes modifies target group attributes.
func (m *MemoryStorage) ModifyTargetGroupAttributes(_ context.Context, targetGroupArn string, attributes []Attribute) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tg, ok := m.TargetGroups[targetGroupArn]
	if !ok {
		return nil, &Error{
			Code:    "TargetGroupNotFound",
			Message: fmt.Sprintf("Target group '%s' not found", targetGroupArn),
		}
	}

	if tg.Attributes == nil {
		tg.Attributes = make(map[string]string)
	}

	for _, attribute := range attributes {
		tg.Attributes[attribute.Key] = attribute.Value
	}

	return cloneAttributes(tg.Attributes), nil
}

// RegisterTargets registers targets with a target group.
func (m *MemoryStorage) RegisterTargets(_ context.Context, targetGroupArn string, targets []Target) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.TargetGroups[targetGroupArn]; !ok {
		return &Error{
			Code:    "TargetGroupNotFound",
			Message: fmt.Sprintf("Target group '%s' not found", targetGroupArn),
		}
	}

	existingTargets := m.Targets[targetGroupArn]
	existingSet := make(map[string]bool)

	for _, t := range existingTargets {
		existingSet[t.ID] = true
	}

	for _, t := range targets {
		if !existingSet[t.ID] {
			existingTargets = append(existingTargets, t)
		}
	}

	m.Targets[targetGroupArn] = existingTargets

	return nil
}

// DeregisterTargets deregisters targets from a target group.
func (m *MemoryStorage) DeregisterTargets(_ context.Context, targetGroupArn string, targets []Target) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.TargetGroups[targetGroupArn]; !ok {
		return &Error{
			Code:    "TargetGroupNotFound",
			Message: fmt.Sprintf("Target group '%s' not found", targetGroupArn),
		}
	}

	removeSet := make(map[string]bool)
	for _, t := range targets {
		removeSet[t.ID] = true
	}

	existingTargets := m.Targets[targetGroupArn]
	newTargets := make([]Target, 0, len(existingTargets))

	for _, t := range existingTargets {
		if !removeSet[t.ID] {
			newTargets = append(newTargets, t)
		}
	}

	m.Targets[targetGroupArn] = newTargets

	return nil
}

// CreateListener creates a new listener.
func (m *MemoryStorage) CreateListener(_ context.Context, req *CreateListenerRequest) (*Listener, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.LoadBalancers[req.LoadBalancerArn]
	if !ok {
		return nil, &Error{
			Code:    "LoadBalancerNotFound",
			Message: fmt.Sprintf("Load balancer '%s' not found", req.LoadBalancerArn),
		}
	}

	listenerID := uuid.New().String()[:17]

	// Parse load balancer ID from ARN for listener ARN.
	lbIDStart := len(req.LoadBalancerArn) - 17
	lbID := req.LoadBalancerArn[lbIDStart:]

	// Get load balancer type from the ARN.
	lbType := lb.Type[:3]

	arn := fmt.Sprintf("arn:aws:elasticloadbalancing:%s:%s:listener/%s/%s/%s/%s",
		defaultRegion, defaultAccountID, lbType, lb.LoadBalancerName, lbID, listenerID)

	listener := &Listener{
		ListenerArn:     arn,
		LoadBalancerArn: req.LoadBalancerArn,
		Port:            req.Port,
		Protocol:        req.Protocol,
		DefaultActions:  req.DefaultActions,
		Tags:            append([]Tag(nil), req.Tags...),
		Attributes:      map[string]string{},
	}

	m.Listeners[arn] = listener

	// Update target group's load balancer ARNs.
	for _, action := range req.DefaultActions {
		if action.TargetGroupArn != "" {
			if tg, exists := m.TargetGroups[action.TargetGroupArn]; exists {
				if !slices.Contains(tg.LoadBalancerArns, req.LoadBalancerArn) {
					tg.LoadBalancerArns = append(tg.LoadBalancerArns, req.LoadBalancerArn)
				}
			}
		}
	}

	return listener, nil
}

// CreateRule creates a new listener rule.
func (m *MemoryStorage) CreateRule(_ context.Context, req *CreateRuleRequest) (*Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.Listeners[req.ListenerArn]; !ok {
		return nil, &Error{
			Code:    "ListenerNotFound",
			Message: fmt.Sprintf("Listener '%s' not found", req.ListenerArn),
		}
	}

	ruleID := uuid.New().String()[:17]
	ruleArn := fmt.Sprintf("arn:aws:elasticloadbalancing:%s:%s:listener-rule/%s/%s",
		defaultRegion, defaultAccountID, ruleID, uuid.New().String()[:17])

	rule := &Rule{
		RuleArn:     ruleArn,
		ListenerArn: req.ListenerArn,
		Priority:    req.Priority,
		IsDefault:   false,
		Actions:     append([]Action(nil), req.Actions...),
		Conditions:  append([]RuleCondition(nil), req.Conditions...),
		Tags:        append([]Tag(nil), req.Tags...),
	}

	m.Rules[ruleArn] = rule

	return rule, nil
}

// DescribeListeners describes listeners.
func (m *MemoryStorage) DescribeListeners(_ context.Context, listenerArns []string, loadBalancerArn string) ([]*Listener, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Listener, 0)

	if len(listenerArns) == 0 && loadBalancerArn == "" {
		for _, listener := range m.Listeners {
			result = append(result, listener)
		}

		return result, nil
	}

	listenerArnSet := make(map[string]bool, len(listenerArns))
	for _, listenerArn := range listenerArns {
		listenerArnSet[listenerArn] = true
	}

	for _, listener := range m.Listeners {
		if len(listenerArns) > 0 && listenerArnSet[listener.ListenerArn] {
			result = append(result, listener)

			continue
		}

		if loadBalancerArn != "" && listener.LoadBalancerArn == loadBalancerArn {
			result = append(result, listener)
		}
	}

	return result, nil
}

// DescribeRules describes rules.
func (m *MemoryStorage) DescribeRules(_ context.Context, listenerArn string, ruleArns []string) ([]*Rule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Rule, 0)

	if listenerArn == "" && len(ruleArns) == 0 {
		for _, rule := range m.Rules {
			result = append(result, rule)
		}

		return result, nil
	}

	ruleArnSet := make(map[string]bool, len(ruleArns))
	for _, ruleArn := range ruleArns {
		ruleArnSet[ruleArn] = true
	}

	for _, rule := range m.Rules {
		if len(ruleArns) > 0 && ruleArnSet[rule.RuleArn] {
			result = append(result, rule)

			continue
		}

		if listenerArn != "" && rule.ListenerArn == listenerArn {
			result = append(result, rule)
		}
	}

	return result, nil
}

// DescribeListenerAttributes describes listener attributes.
func (m *MemoryStorage) DescribeListenerAttributes(_ context.Context, listenerArn string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	listener, ok := m.Listeners[listenerArn]
	if !ok {
		return nil, &Error{
			Code:    "ListenerNotFound",
			Message: fmt.Sprintf("Listener '%s' not found", listenerArn),
		}
	}

	return cloneAttributes(listener.Attributes), nil
}

// DeleteListener deletes a listener.
func (m *MemoryStorage) DeleteListener(_ context.Context, listenerArn string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.Listeners[listenerArn]; !ok {
		return &Error{
			Code:    "ListenerNotFound",
			Message: fmt.Sprintf("Listener '%s' not found", listenerArn),
		}
	}

	for ruleArn, rule := range m.Rules {
		if rule.ListenerArn == listenerArn {
			delete(m.Rules, ruleArn)
		}
	}

	delete(m.Listeners, listenerArn)

	return nil
}

func (m *MemoryStorage) getResourceTagsLocked(resourceArn string) ([]Tag, bool) {
	if lb, ok := m.LoadBalancers[resourceArn]; ok {
		return lb.Tags, true
	}

	if tg, ok := m.TargetGroups[resourceArn]; ok {
		return tg.Tags, true
	}

	if listener, ok := m.Listeners[resourceArn]; ok {
		return listener.Tags, true
	}

	if rule, ok := m.Rules[resourceArn]; ok {
		return rule.Tags, true
	}

	return nil, false
}

func (m *MemoryStorage) getMutableResourceTagsLocked(resourceArn string) (*[]Tag, bool) {
	if lb, ok := m.LoadBalancers[resourceArn]; ok {
		return &lb.Tags, true
	}

	if tg, ok := m.TargetGroups[resourceArn]; ok {
		return &tg.Tags, true
	}

	if listener, ok := m.Listeners[resourceArn]; ok {
		return &listener.Tags, true
	}

	if rule, ok := m.Rules[resourceArn]; ok {
		return &rule.Tags, true
	}

	return nil, false
}

func upsertTag(tags *[]Tag, tag Tag) {
	for i := range *tags {
		if (*tags)[i].Key == tag.Key {
			(*tags)[i].Value = tag.Value

			return
		}
	}

	*tags = append(*tags, tag)
}

func removeTags(tags *[]Tag, tagKeys []string) {
	removeSet := make(map[string]struct{}, len(tagKeys))
	for _, tagKey := range tagKeys {
		removeSet[tagKey] = struct{}{}
	}

	filteredTags := (*tags)[:0]
	for _, tag := range *tags {
		if _, ok := removeSet[tag.Key]; ok {
			continue
		}

		filteredTags = append(filteredTags, tag)
	}

	*tags = filteredTags
}

func cloneAttributes(attributes map[string]string) map[string]string {
	if len(attributes) == 0 {
		return map[string]string{}
	}

	cloned := make(map[string]string, len(attributes))
	for key, value := range attributes {
		cloned[key] = value
	}

	return cloned
}

func matchesDescribeTargetHealthTargets(target Target, filters []Target) bool {
	if len(filters) == 0 {
		return true
	}

	for _, filter := range filters {
		if target.ID != filter.ID {
			continue
		}
		if filter.Port != 0 && target.Port != filter.Port {
			continue
		}
		if filter.AvailabilityZone != "" && target.AvailabilityZone != filter.AvailabilityZone {
			continue
		}

		return true
	}

	return false
}
