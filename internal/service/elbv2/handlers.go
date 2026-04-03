package elbv2

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Error codes for ELB.
const (
	errInvalidParameter = "InvalidParameterValue"
	errInternalError    = "InternalError"
	errInvalidAction    = "InvalidAction"
)

// CreateLoadBalancer handles the CreateLoadBalancer action.
func (s *Service) CreateLoadBalancer(w http.ResponseWriter, r *http.Request) {
	var req CreateLoadBalancerRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeELBError(w, errInvalidParameter, "Name is required", http.StatusBadRequest)

		return
	}

	if tags := parseELBTagsFromForm(r.Form, "Tags.member"); tags != nil {
		req.Tags = tags
	}
	if len(req.Subnets) == 0 {
		req.Subnets = parseELBSubnetMappingsFromForm(r.Form, "SubnetMappings.member")
	}

	lb, err := s.storage.CreateLoadBalancer(r.Context(), &req)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLCreateLoadBalancerResponse{
		Xmlns: elbXMLNS,
		Result: XMLCreateLoadBalancerResult{
			LoadBalancers: XMLLoadBalancers{
				Members: []XMLLoadBalancer{convertToXMLLoadBalancer(lb)},
			},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteLoadBalancer handles the DeleteLoadBalancer action.
func (s *Service) DeleteLoadBalancer(w http.ResponseWriter, r *http.Request) {
	var req DeleteLoadBalancerRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LoadBalancerArn == "" {
		writeELBError(w, errInvalidParameter, "LoadBalancerArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteLoadBalancer(r.Context(), req.LoadBalancerArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeleteLoadBalancerResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDeleteLoadBalancerResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeLoadBalancers handles the DescribeLoadBalancers action.
func (s *Service) DescribeLoadBalancers(w http.ResponseWriter, r *http.Request) {
	var req DescribeLoadBalancersRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	lbs, err := s.storage.DescribeLoadBalancers(r.Context(), req.LoadBalancerArns, req.Names)
	if err != nil {
		handleELBError(w, err)

		return
	}

	xmlLbs := make([]XMLLoadBalancer, 0, len(lbs))
	for _, lb := range lbs {
		xmlLbs = append(xmlLbs, convertToXMLLoadBalancer(lb))
	}

	writeELBXMLResponse(w, XMLDescribeLoadBalancersResponse{
		Xmlns: elbXMLNS,
		Result: XMLDescribeLoadBalancersResult{
			LoadBalancers: XMLLoadBalancers{Members: xmlLbs},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}


// DescribeLoadBalancerAttributes handles the DescribeLoadBalancerAttributes action.
func (s *Service) DescribeLoadBalancerAttributes(w http.ResponseWriter, r *http.Request) {
	var req DescribeLoadBalancerAttributesRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	attributes, err := s.storage.DescribeLoadBalancerAttributes(r.Context(), req.LoadBalancerArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDescribeLoadBalancerAttributesResponse{
		Xmlns: elbXMLNS,
		Result: XMLDescribeLoadBalancerAttributesResult{
			Attributes: convertToXMLAttributes(attributes),
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ModifyLoadBalancerAttributes handles the ModifyLoadBalancerAttributes action.
func (s *Service) ModifyLoadBalancerAttributes(w http.ResponseWriter, r *http.Request) {
	var req ModifyLoadBalancerAttributesRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	attributes, err := s.storage.ModifyLoadBalancerAttributes(r.Context(), req.LoadBalancerArn, req.Attributes)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLModifyLoadBalancerAttributesResponse{
		Xmlns: elbXMLNS,
		Result: XMLModifyLoadBalancerAttributesResult{
			Attributes: convertToXMLAttributes(attributes),
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeTags handles the DescribeTags action.
func (s *Service) DescribeTags(w http.ResponseWriter, r *http.Request) {
	var req DescribeTagsRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.ResourceArns) == 0 {
		req.ResourceArns = parseELBStringMembersFromForm(r.Form, "ResourceArns.member")
	}

	descriptions, err := s.storage.DescribeTags(r.Context(), req.ResourceArns)
	if err != nil {
		handleELBError(w, err)

		return
	}

	xmlDescriptions := make([]XMLTagDescription, 0, len(descriptions))
	for _, description := range descriptions {
		xmlDescriptions = append(xmlDescriptions, convertToXMLTagDescription(description))
	}

	writeELBXMLResponse(w, XMLDescribeTagsResponse{
		Xmlns: elbXMLNS,
		Result: XMLDescribeTagsResult{
			TagDescriptions: XMLTagDescriptions{Members: xmlDescriptions},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// AddTags handles the AddTags action.
func (s *Service) AddTags(w http.ResponseWriter, r *http.Request) {
	var req AddTagsRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.ResourceArns) == 0 {
		req.ResourceArns = parseELBStringMembersFromForm(r.Form, "ResourceArns.member")
	}
	if tags := parseELBTagsFromForm(r.Form, "Tags.member"); tags != nil {
		req.Tags = tags
	}

	if err := s.storage.AddTags(r.Context(), req.ResourceArns, req.Tags); err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLAddTagsResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLAddTagsResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// RemoveTags handles the RemoveTags action.
func (s *Service) RemoveTags(w http.ResponseWriter, r *http.Request) {
	var req RemoveTagsRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if len(req.ResourceArns) == 0 {
		req.ResourceArns = parseELBStringMembersFromForm(r.Form, "ResourceArns.member")
	}
	if len(req.TagKeys) == 0 {
		req.TagKeys = parseELBStringMembersFromForm(r.Form, "TagKeys.member")
	}

	if err := s.storage.RemoveTags(r.Context(), req.ResourceArns, req.TagKeys); err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLRemoveTagsResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLRemoveTagsResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// CreateTargetGroup handles the CreateTargetGroup action.
func (s *Service) CreateTargetGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateTargetGroupRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.Name == "" {
		writeELBError(w, errInvalidParameter, "Name is required", http.StatusBadRequest)

		return
	}

	if tags := parseELBTagsFromForm(r.Form, "Tags.member"); tags != nil {
		req.Tags = tags
	}

	tg, err := s.storage.CreateTargetGroup(r.Context(), &req)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLCreateTargetGroupResponse{
		Xmlns: elbXMLNS,
		Result: XMLCreateTargetGroupResult{
			TargetGroups: XMLTargetGroups{
				Members: []XMLTargetGroup{convertToXMLTargetGroup(tg)},
			},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeleteTargetGroup handles the DeleteTargetGroup action.
func (s *Service) DeleteTargetGroup(w http.ResponseWriter, r *http.Request) {
	var req DeleteTargetGroupRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TargetGroupArn == "" {
		writeELBError(w, errInvalidParameter, "TargetGroupArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteTargetGroup(r.Context(), req.TargetGroupArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeleteTargetGroupResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDeleteTargetGroupResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeTargetGroups handles the DescribeTargetGroups action.
func (s *Service) DescribeTargetGroups(w http.ResponseWriter, r *http.Request) {
	var req DescribeTargetGroupsRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	tgs, err := s.storage.DescribeTargetGroups(r.Context(), req.TargetGroupArns, req.Names, req.LoadBalancerArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	xmlTgs := make([]XMLTargetGroup, 0, len(tgs))
	for _, tg := range tgs {
		xmlTgs = append(xmlTgs, convertToXMLTargetGroup(tg))
	}

	writeELBXMLResponse(w, XMLDescribeTargetGroupsResponse{
		Xmlns: elbXMLNS,
		Result: XMLDescribeTargetGroupsResult{
			TargetGroups: XMLTargetGroups{Members: xmlTgs},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}


// DescribeTargetGroupAttributes handles the DescribeTargetGroupAttributes action.
func (s *Service) DescribeTargetGroupAttributes(w http.ResponseWriter, r *http.Request) {
	var req DescribeTargetGroupAttributesRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	attributes, err := s.storage.DescribeTargetGroupAttributes(r.Context(), req.TargetGroupArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDescribeTargetGroupAttributesResponse{
		Xmlns: elbXMLNS,
		Result: XMLDescribeTargetGroupAttributesResult{
			Attributes: convertToXMLAttributes(attributes),
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DescribeTargetHealth handles the DescribeTargetHealth action.
func (s *Service) DescribeTargetHealth(w http.ResponseWriter, r *http.Request) {
	var req DescribeTargetHealthRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}
	if len(req.Targets) == 0 {
		req.Targets = parseELBTargetsFromForm(r.Form, "Targets.member")
	}

	if req.TargetGroupArn == "" {
		writeELBError(w, errInvalidParameter, "TargetGroupArn is required", http.StatusBadRequest)

		return
	}

	descriptions, err := s.storage.DescribeTargetHealth(r.Context(), req.TargetGroupArn, req.Targets)
	if err != nil {
		handleELBError(w, err)

		return
	}

	xmlDescriptions := make([]XMLTargetHealthDescription, 0, len(descriptions))
	for _, description := range descriptions {
		xmlDescriptions = append(xmlDescriptions, convertToXMLTargetHealthDescription(description))
	}

	writeELBXMLResponse(w, XMLDescribeTargetHealthResponse{
		Xmlns: elbXMLNS,
		Result: XMLDescribeTargetHealthResult{
			TargetHealthDescriptions: XMLTargetHealthDescriptions{Members: xmlDescriptions},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// ModifyTargetGroupAttributes handles the ModifyTargetGroupAttributes action.
func (s *Service) ModifyTargetGroupAttributes(w http.ResponseWriter, r *http.Request) {
	var req ModifyTargetGroupAttributesRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	attributes, err := s.storage.ModifyTargetGroupAttributes(r.Context(), req.TargetGroupArn, req.Attributes)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLModifyTargetGroupAttributesResponse{
		Xmlns: elbXMLNS,
		Result: XMLModifyTargetGroupAttributesResult{
			Attributes: convertToXMLAttributes(attributes),
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// RegisterTargets handles the RegisterTargets action.
func (s *Service) RegisterTargets(w http.ResponseWriter, r *http.Request) {
	var req RegisterTargetsRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TargetGroupArn == "" {
		writeELBError(w, errInvalidParameter, "TargetGroupArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.RegisterTargets(r.Context(), req.TargetGroupArn, req.Targets)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLRegisterTargetsResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLRegisterTargetsResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DeregisterTargets handles the DeregisterTargets action.
func (s *Service) DeregisterTargets(w http.ResponseWriter, r *http.Request) {
	var req DeregisterTargetsRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.TargetGroupArn == "" {
		writeELBError(w, errInvalidParameter, "TargetGroupArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeregisterTargets(r.Context(), req.TargetGroupArn, req.Targets)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeregisterTargetsResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDeregisterTargetsResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// CreateListener handles the CreateListener action.
func (s *Service) CreateListener(w http.ResponseWriter, r *http.Request) {
	var req CreateListenerRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.LoadBalancerArn == "" {
		writeELBError(w, errInvalidParameter, "LoadBalancerArn is required", http.StatusBadRequest)

		return
	}


	listener, err := s.storage.CreateListener(r.Context(), &req)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLCreateListenerResponse{
		Xmlns: elbXMLNS,
		Result: XMLCreateListenerResult{
			Listeners: XMLListeners{
				Members: []XMLListener{convertToXMLListener(listener)},
			},
		},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}


// DeleteListener handles the DeleteListener action.
func (s *Service) DeleteListener(w http.ResponseWriter, r *http.Request) {
	var req DeleteListenerRequest
	if err := readELBJSONRequest(r, &req); err != nil {
		writeELBError(w, errInvalidParameter, "Failed to parse request body", http.StatusBadRequest)

		return
	}

	if req.ListenerArn == "" {
		writeELBError(w, errInvalidParameter, "ListenerArn is required", http.StatusBadRequest)

		return
	}

	err := s.storage.DeleteListener(r.Context(), req.ListenerArn)
	if err != nil {
		handleELBError(w, err)

		return
	}

	writeELBXMLResponse(w, XMLDeleteListenerResponse{
		Xmlns:            elbXMLNS,
		Result:           XMLDeleteListenerResult{},
		ResponseMetadata: XMLResponseMetadata{RequestID: uuid.New().String()},
	})
}

// DispatchAction routes the request to the appropriate handler based on Action parameter.
func (s *Service) DispatchAction(w http.ResponseWriter, r *http.Request) {
	action := extractAction(r)
	handler := s.getActionHandler(action)

	if handler == nil {
		writeELBError(w, errInvalidAction, fmt.Sprintf("The action '%s' is not valid", action), http.StatusBadRequest)

		return
	}

	handler(w, r)
}

// getActionHandler returns the handler function for the given action.
func (s *Service) getActionHandler(action string) func(http.ResponseWriter, *http.Request) {
	handlers := map[string]func(http.ResponseWriter, *http.Request){
		"CreateLoadBalancer":             s.CreateLoadBalancer,
		"DeleteLoadBalancer":             s.DeleteLoadBalancer,
		"DescribeLoadBalancers":          s.DescribeLoadBalancers,
		"DescribeLoadBalancerAttributes": s.DescribeLoadBalancerAttributes,
		"ModifyLoadBalancerAttributes":   s.ModifyLoadBalancerAttributes,
		"DescribeTags":                   s.DescribeTags,
		"AddTags":                        s.AddTags,
		"RemoveTags":                     s.RemoveTags,
		"CreateTargetGroup":              s.CreateTargetGroup,
		"DeleteTargetGroup":              s.DeleteTargetGroup,
		"DescribeTargetGroups":           s.DescribeTargetGroups,
		"DescribeTargetGroupAttributes":  s.DescribeTargetGroupAttributes,
		"DescribeTargetHealth":           s.DescribeTargetHealth,
		"ModifyTargetGroupAttributes":    s.ModifyTargetGroupAttributes,
		"RegisterTargets":                s.RegisterTargets,
		"DeregisterTargets":              s.DeregisterTargets,
		"CreateListener":                 s.CreateListener,
		"DeleteListener":                 s.DeleteListener,
	}

	return handlers[action]
}

// Helper functions.

// convertToXMLLoadBalancer converts a LoadBalancer to XMLLoadBalancer.
func convertToXMLLoadBalancer(lb *LoadBalancer) XMLLoadBalancer {
	azs := make([]XMLAvailabilityZone, 0, len(lb.AvailabilityZones))
	for _, az := range lb.AvailabilityZones {
		azs = append(azs, XMLAvailabilityZone{
			ZoneName: az.ZoneName,
			SubnetID: az.SubnetID,
		})
	}

	return XMLLoadBalancer{
		LoadBalancerArn:       lb.LoadBalancerArn,
		DNSName:               lb.DNSName,
		CanonicalHostedZoneID: lb.CanonicalHostedZoneID,
		CreatedTime:           lb.CreatedTime.Format("2006-01-02T15:04:05.000Z"),
		LoadBalancerName:      lb.LoadBalancerName,
		Scheme:                lb.Scheme,
		VpcID:                 lb.VpcID,
		State:                 XMLLoadBalancerState{Code: lb.State.Code, Reason: lb.State.Reason},
		Type:                  lb.Type,
		AvailabilityZones:     XMLAvailabilityZones{Members: azs},
		SecurityGroups:        XMLSecurityGroups{Members: lb.SecurityGroups},
		IPAddressType:         lb.IPAddressType,
	}
}

// convertToXMLTargetGroup converts a TargetGroup to XMLTargetGroup.
func convertToXMLTargetGroup(tg *TargetGroup) XMLTargetGroup {
	return XMLTargetGroup{
		TargetGroupArn:             tg.TargetGroupArn,
		TargetGroupName:            tg.TargetGroupName,
		Protocol:                   tg.Protocol,
		Port:                       tg.Port,
		VpcID:                      tg.VpcID,
		HealthCheckEnabled:         tg.HealthCheckEnabled,
		HealthCheckIntervalSeconds: tg.HealthCheckIntervalSeconds,
		HealthCheckPath:            tg.HealthCheckPath,
		HealthCheckPort:            tg.HealthCheckPort,
		HealthCheckProtocol:        tg.HealthCheckProtocol,
		HealthCheckTimeoutSeconds:  tg.HealthCheckTimeoutSeconds,
		HealthyThresholdCount:      tg.HealthyThresholdCount,
		UnhealthyThresholdCount:    tg.UnhealthyThresholdCount,
		TargetType:                 tg.TargetType,
		LoadBalancerArns:           XMLLoadBalancerArns{Members: tg.LoadBalancerArns},
	}
}


func convertToXMLTargetHealthDescription(description *TargetHealthDescription) XMLTargetHealthDescription {
	return XMLTargetHealthDescription{
		Target: XMLTarget{
			ID:               description.Target.ID,
			Port:             description.Target.Port,
			AvailabilityZone: description.Target.AvailabilityZone,
		},
		TargetHealth: XMLTargetHealth{
			State:       description.TargetHealth.State,
			Reason:      description.TargetHealth.Reason,
			Description: description.TargetHealth.Description,
		},
	}
}

// convertToXMLListener converts a Listener to XMLListener.
func convertToXMLListener(l *Listener) XMLListener {
	actions := make([]XMLAction, 0, len(l.DefaultActions))
	for _, a := range l.DefaultActions {
		actions = append(actions, XMLAction(a))
	}

	return XMLListener{
		ListenerArn:     l.ListenerArn,
		LoadBalancerArn: l.LoadBalancerArn,
		Port:            l.Port,
		Protocol:        l.Protocol,
		DefaultActions:  XMLActions{Members: actions},
	}
}


// convertToXMLTagDescription converts a TagDescription to XMLTagDescription.
func convertToXMLTagDescription(description *TagDescription) XMLTagDescription {
	tags := make([]XMLTag, 0, len(description.Tags))
	for _, tag := range description.Tags {
		tags = append(tags, XMLTag(tag))
	}

	return XMLTagDescription{
		ResourceArn: description.ResourceArn,
		Tags:        XMLTags{Members: tags},
	}
}


// convertToXMLAttributes converts attributes to XMLAttributes.
func convertToXMLAttributes(attributes map[string]string) XMLAttributes {
	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	xmlAttributes := make([]XMLAttribute, 0, len(keys))
	for _, key := range keys {
		xmlAttributes = append(xmlAttributes, XMLAttribute{
			Key:   key,
			Value: attributes[key],
		})
	}

	return XMLAttributes{Members: xmlAttributes}
}

// readELBJSONRequest reads and decodes JSON request body.
func readELBJSONRequest(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	if len(body) == 0 {
		return nil
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

func parseELBStringMembersFromForm(form url.Values, prefix string) []string {
	indexed := make(map[int]string)
	for key, values := range form {
		if !strings.HasPrefix(key, prefix+".") || len(values) == 0 {
			continue
		}

		index, err := strconv.Atoi(strings.TrimPrefix(key, prefix+"."))
		if err != nil {
			continue
		}

		indexed[index] = values[0]
	}

	if len(indexed) == 0 {
		return nil
	}

	indexes := make([]int, 0, len(indexed))
	for index := range indexed {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	values := make([]string, 0, len(indexes))
	for _, index := range indexes {
		values = append(values, indexed[index])
	}

	return values
}

func parseELBTagsFromForm(form url.Values, prefix string) []Tag {
	entries := make(map[int]*Tag)
	for key, values := range form {
		if len(values) == 0 || !strings.HasPrefix(key, prefix+".") {
			continue
		}

		parts := strings.Split(key, ".")
		if len(parts) != 4 {
			continue
		}

		index, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}

		entry := entries[index]
		if entry == nil {
			entry = &Tag{}
			entries[index] = entry
		}

		switch parts[3] {
		case "Key":
			entry.Key = values[0]
		case "Value":
			entry.Value = values[0]
		}
	}

	if len(entries) == 0 {
		return nil
	}

	indexes := make([]int, 0, len(entries))
	for index := range entries {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	tags := make([]Tag, 0, len(indexes))
	for _, index := range indexes {
		tag := entries[index]
		if tag == nil || tag.Key == "" {
			continue
		}

		tags = append(tags, *tag)
	}

	return tags
}

func parseELBSubnetMappingsFromForm(form url.Values, prefix string) []string {
	indexed := make(map[int]string)
	for key, values := range form {
		if len(values) == 0 || !strings.HasPrefix(key, prefix+".") || !strings.HasSuffix(key, ".SubnetId") {
			continue
		}

		parts := strings.Split(key, ".")
		if len(parts) != 4 {
			continue
		}

		index, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}

		indexed[index] = values[0]
	}

	if len(indexed) == 0 {
		return nil
	}

	indexes := make([]int, 0, len(indexed))
	for index := range indexed {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	subnetIDs := make([]string, 0, len(indexes))
	for _, index := range indexes {
		subnetIDs = append(subnetIDs, indexed[index])
	}

	return subnetIDs
}


func parseELBTargetsFromForm(form url.Values, prefix string) []Target {
	entries := make(map[int]*Target)
	for key, values := range form {
		if len(values) == 0 || !strings.HasPrefix(key, prefix+".") {
			continue
		}

		parts := strings.Split(key, ".")
		if len(parts) != 4 {
			continue
		}

		index, err := strconv.Atoi(parts[2])
		if err != nil {
			continue
		}

		entry := entries[index]
		if entry == nil {
			entry = &Target{}
			entries[index] = entry
		}

		switch parts[3] {
		case "Id":
			entry.ID = values[0]
		case "Port":
			port, err := strconv.Atoi(values[0])
			if err == nil {
				entry.Port = port
			}
		case "AvailabilityZone":
			entry.AvailabilityZone = values[0]
		}
	}

	if len(entries) == 0 {
		return nil
	}

	indexes := make([]int, 0, len(entries))
	for index := range entries {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)

	targets := make([]Target, 0, len(indexes))
	for _, index := range indexes {
		target := entries[index]
		if target == nil || target.ID == "" {
			continue
		}

		targets = append(targets, *target)
	}

	return targets
}

// extractAction extracts the action name from the request.
func extractAction(r *http.Request) string {
	// Try X-Amz-Target header (format: "ElasticLoadBalancing.ActionName").
	target := r.Header.Get("X-Amz-Target")
	if target != "" {
		if idx := strings.LastIndex(target, "."); idx >= 0 {
			return target[idx+1:]
		}
	}

	// Fallback to URL query parameter.
	return r.URL.Query().Get("Action")
}

// writeELBXMLResponse writes an XML response with HTTP 200 OK.
func writeELBXMLResponse(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", uuid.New().String())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(v)
}

// writeELBError writes an ELB error response in XML format.
func writeELBError(w http.ResponseWriter, code, message string, status int) {
	requestID := uuid.New().String()

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("x-amzn-RequestId", requestID)
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(XMLErrorResponse{
		Error: XMLError{
			Type:    "Sender",
			Code:    code,
			Message: message,
		},
		RequestID: requestID,
	})
}

// handleELBError handles ELB errors and writes the appropriate response.
func handleELBError(w http.ResponseWriter, err error) {
	var elbErr *Error
	if errors.As(err, &elbErr) {
		writeELBError(w, elbErr.Code, elbErr.Message, http.StatusBadRequest)

		return
	}

	writeELBError(w, errInternalError, "Internal server error", http.StatusInternalServerError)
}
