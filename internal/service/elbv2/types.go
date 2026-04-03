// Package elbv2 provides ELB v2 service emulation for kumo.
package elbv2

import (
	"encoding/xml"
	"time"
)

const elbXMLNS = "http://elasticloadbalancing.amazonaws.com/doc/2015-12-01/"

// LoadBalancer represents an ELB load balancer.
type LoadBalancer struct {
	LoadBalancerArn       string
	DNSName               string
	CanonicalHostedZoneID string
	CreatedTime           time.Time
	LoadBalancerName      string
	Scheme                string // internet-facing | internal
	VpcID                 string
	State                 LoadBalancerState
	Type                  string // application | network | gateway
	AvailabilityZones     []AvailabilityZone
	SecurityGroups        []string
	IPAddressType         string
	Tags                  []Tag
	Attributes            map[string]string
}

// LoadBalancerState represents the state of a load balancer.
type LoadBalancerState struct {
	Code   string
	Reason string
}

// AvailabilityZone represents an availability zone.
type AvailabilityZone struct {
	ZoneName         string
	SubnetID         string
	LoadBalancerAddr []LoadBalancerAddress
}

// LoadBalancerAddress represents a load balancer address.
type LoadBalancerAddress struct {
	IPAddress    string
	AllocationID string
}

// TargetGroup represents an ELB target group.
type TargetGroup struct {
	TargetGroupArn             string
	TargetGroupName            string
	Protocol                   string
	ProtocolVersion            string
	Port                       int
	VpcID                      string
	IPAddressType              string
	HealthCheckEnabled         bool
	HealthCheckIntervalSeconds int
	HealthCheckPath            string
	HealthCheckPort            string
	HealthCheckProtocol        string
	Matcher                    *Matcher
	HealthCheckTimeoutSeconds  int
	HealthyThresholdCount      int
	UnhealthyThresholdCount    int
	TargetType                 string // instance | ip | lambda | alb
	LoadBalancerArns           []string
	Tags                       []Tag
	Attributes                 map[string]string
}

// Rule represents an ELB listener rule.
type Rule struct {
	RuleArn     string
	ListenerArn string
	Priority    int
	IsDefault   bool
	Actions     []Action
	Conditions  []RuleCondition
	Tags        []Tag
}

// Listener represents an ELB listener.
type Listener struct {
	ListenerArn     string
	LoadBalancerArn string
	Port            int
	Protocol        string
	DefaultActions  []Action
	Tags            []Tag
	Attributes      map[string]string
}

// Action represents a listener action.
type Action struct {
	Type                string
	TargetGroupArn      string
	Order               int
	ForwardConfig       *ForwardConfig
	FixedResponseConfig *FixedResponseConfig
}

// ForwardConfig represents a forward action configuration.
type ForwardConfig struct {
	TargetGroups []TargetGroupTuple
}

// TargetGroupTuple represents a target group tuple in a forward action.
type TargetGroupTuple struct {
	TargetGroupArn string
	Weight         int
}

// FixedResponseConfig represents a fixed response action configuration.
type FixedResponseConfig struct {
	ContentType string
	MessageBody string
	StatusCode  string
}

// RuleCondition represents a rule condition.
type RuleCondition struct {
	Field             string
	Values            []string
	PathPatternConfig *PathPatternConditionConfig
}

// PathPatternConditionConfig represents a path pattern rule condition.
type PathPatternConditionConfig struct {
	Values []string
}

// Target represents a target in a target group.
type Target struct {
	ID               string
	Port             int
	AvailabilityZone string
}

// TargetHealthDescription represents a target with its health status.
type TargetHealthDescription struct {
	Target       Target
	TargetHealth TargetHealth
}

// TargetHealth represents the health of a target.
type TargetHealth struct {
	State       string
	Reason      string
	Description string
}

// Tag represents a resource tag.
type Tag struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// TagDescription represents tags attached to a resource.
type TagDescription struct {
	ResourceArn string
	Tags        []Tag
}

// Matcher represents a target group matcher.
type Matcher struct {
	HTTPCode string `json:"HttpCode,omitempty"`
	GRPCCode string `json:"GrpcCode,omitempty"`
}

// Attribute represents a resource attribute.
type Attribute struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// Request types.

// CreateLoadBalancerRequest represents a CreateLoadBalancer request.
type CreateLoadBalancerRequest struct {
	Name           string   `json:"Name"`
	Subnets        []string `json:"Subnets,omitempty"`
	SecurityGroups []string `json:"SecurityGroups,omitempty"`
	Scheme         string   `json:"Scheme,omitempty"`
	Type           string   `json:"Type,omitempty"`
	IPAddressType  string   `json:"IpAddressType,omitempty"`
	Tags           []Tag    `json:"Tags,omitempty"`
}

// DeleteLoadBalancerRequest represents a DeleteLoadBalancer request.
type DeleteLoadBalancerRequest struct {
	LoadBalancerArn string `json:"LoadBalancerArn"`
}

// DescribeLoadBalancersRequest represents a DescribeLoadBalancers request.
type DescribeLoadBalancersRequest struct {
	LoadBalancerArns []string `json:"LoadBalancerArns,omitempty"`
	Names            []string `json:"Names,omitempty"`
}

// CreateTargetGroupRequest represents a CreateTargetGroup request.
type CreateTargetGroupRequest struct {
	Name                       string   `json:"Name"`
	Protocol                   string   `json:"Protocol,omitempty"`
	ProtocolVersion            string   `json:"ProtocolVersion,omitempty"`
	Port                       int      `json:"Port,omitempty"`
	VpcID                      string   `json:"VpcId,omitempty"`
	IPAddressType              string   `json:"IpAddressType,omitempty"`
	HealthCheckProtocol        string   `json:"HealthCheckProtocol,omitempty"`
	HealthCheckPort            string   `json:"HealthCheckPort,omitempty"`
	HealthCheckEnabled         bool     `json:"HealthCheckEnabled,omitempty"`
	HealthCheckPath            string   `json:"HealthCheckPath,omitempty"`
	Matcher                    *Matcher `json:"Matcher,omitempty"`
	HealthCheckIntervalSeconds int      `json:"HealthCheckIntervalSeconds,omitempty"`
	HealthCheckTimeoutSeconds  int      `json:"HealthCheckTimeoutSeconds,omitempty"`
	HealthyThresholdCount      int      `json:"HealthyThresholdCount,omitempty"`
	UnhealthyThresholdCount    int      `json:"UnhealthyThresholdCount,omitempty"`
	TargetType                 string   `json:"TargetType,omitempty"`
	Tags                       []Tag    `json:"Tags,omitempty"`
}

// DeleteTargetGroupRequest represents a DeleteTargetGroup request.
type DeleteTargetGroupRequest struct {
	TargetGroupArn string `json:"TargetGroupArn"`
}

// DescribeTargetGroupsRequest represents a DescribeTargetGroups request.
type DescribeTargetGroupsRequest struct {
	TargetGroupArns []string `json:"TargetGroupArns,omitempty"`
	Names           []string `json:"Names,omitempty"`
	LoadBalancerArn string   `json:"LoadBalancerArn,omitempty"`
}

// DescribeTargetHealthRequest represents a DescribeTargetHealth request.
type DescribeTargetHealthRequest struct {
	TargetGroupArn string   `json:"TargetGroupArn"`
	Targets        []Target `json:"Targets,omitempty"`
}

// RegisterTargetsRequest represents a RegisterTargets request.
type RegisterTargetsRequest struct {
	TargetGroupArn string   `json:"TargetGroupArn"`
	Targets        []Target `json:"Targets"`
}

// DeregisterTargetsRequest represents a DeregisterTargets request.
type DeregisterTargetsRequest struct {
	TargetGroupArn string   `json:"TargetGroupArn"`
	Targets        []Target `json:"Targets"`
}

// CreateListenerRequest represents a CreateListener request.
type CreateListenerRequest struct {
	LoadBalancerArn string   `json:"LoadBalancerArn"`
	Port            int      `json:"Port"`
	Protocol        string   `json:"Protocol"`
	DefaultActions  []Action `json:"DefaultActions"`
	Tags            []Tag    `json:"Tags,omitempty"`
}

// CreateRuleRequest represents a CreateRule request.
type CreateRuleRequest struct {
	ListenerArn string          `json:"ListenerArn"`
	Priority    int             `json:"Priority"`
	Actions     []Action        `json:"Actions,omitempty"`
	Conditions  []RuleCondition `json:"Conditions,omitempty"`
	Tags        []Tag           `json:"Tags,omitempty"`
}

// DescribeListenersRequest represents a DescribeListeners request.
type DescribeListenersRequest struct {
	LoadBalancerArn string   `json:"LoadBalancerArn,omitempty"`
	ListenerArns    []string `json:"ListenerArns,omitempty"`
}

// DescribeRulesRequest represents a DescribeRules request.
type DescribeRulesRequest struct {
	ListenerArn string   `json:"ListenerArn,omitempty"`
	RuleArns    []string `json:"RuleArns,omitempty"`
}

// DescribeListenerAttributesRequest represents a DescribeListenerAttributes request.
type DescribeListenerAttributesRequest struct {
	ListenerArn string `json:"ListenerArn"`
}

// DeleteListenerRequest represents a DeleteListener request.
type DeleteListenerRequest struct {
	ListenerArn string `json:"ListenerArn"`
}

// DescribeTagsRequest represents a DescribeTags request.
type DescribeTagsRequest struct {
	ResourceArns []string `json:"ResourceArns,omitempty"`
}

// AddTagsRequest represents an AddTags request.
type AddTagsRequest struct {
	ResourceArns []string `json:"ResourceArns,omitempty"`
	Tags         []Tag    `json:"Tags,omitempty"`
}

// RemoveTagsRequest represents a RemoveTags request.
type RemoveTagsRequest struct {
	ResourceArns []string `json:"ResourceArns,omitempty"`
	TagKeys      []string `json:"TagKeys,omitempty"`
}

// DescribeTargetGroupAttributesRequest represents a DescribeTargetGroupAttributes request.
type DescribeTargetGroupAttributesRequest struct {
	TargetGroupArn string `json:"TargetGroupArn"`
}

// ModifyTargetGroupAttributesRequest represents a ModifyTargetGroupAttributes request.
type ModifyTargetGroupAttributesRequest struct {
	TargetGroupArn string      `json:"TargetGroupArn"`
	Attributes     []Attribute `json:"Attributes,omitempty"`
}

// DescribeLoadBalancerAttributesRequest represents a DescribeLoadBalancerAttributes request.
type DescribeLoadBalancerAttributesRequest struct {
	LoadBalancerArn string `json:"LoadBalancerArn"`
}

// ModifyLoadBalancerAttributesRequest represents a ModifyLoadBalancerAttributes request.
type ModifyLoadBalancerAttributesRequest struct {
	LoadBalancerArn string      `json:"LoadBalancerArn"`
	Attributes      []Attribute `json:"Attributes,omitempty"`
}

// XML Response types.

// XMLCreateLoadBalancerResponse is the XML response for CreateLoadBalancer.
type XMLCreateLoadBalancerResponse struct {
	XMLName          xml.Name                    `xml:"CreateLoadBalancerResponse"`
	Xmlns            string                      `xml:"xmlns,attr"`
	Result           XMLCreateLoadBalancerResult `xml:"CreateLoadBalancerResult"`
	ResponseMetadata XMLResponseMetadata         `xml:"ResponseMetadata"`
}

// XMLCreateLoadBalancerResult contains the result of CreateLoadBalancer.
type XMLCreateLoadBalancerResult struct {
	LoadBalancers XMLLoadBalancers `xml:"LoadBalancers"`
}

// XMLDeleteLoadBalancerResponse is the XML response for DeleteLoadBalancer.
type XMLDeleteLoadBalancerResponse struct {
	XMLName          xml.Name                    `xml:"DeleteLoadBalancerResponse"`
	Xmlns            string                      `xml:"xmlns,attr"`
	Result           XMLDeleteLoadBalancerResult `xml:"DeleteLoadBalancerResult"`
	ResponseMetadata XMLResponseMetadata         `xml:"ResponseMetadata"`
}

// XMLDeleteLoadBalancerResult is an empty result for DeleteLoadBalancer.
type XMLDeleteLoadBalancerResult struct{}

// XMLDescribeLoadBalancersResponse is the XML response for DescribeLoadBalancers.
type XMLDescribeLoadBalancersResponse struct {
	XMLName          xml.Name                       `xml:"DescribeLoadBalancersResponse"`
	Xmlns            string                         `xml:"xmlns,attr"`
	Result           XMLDescribeLoadBalancersResult `xml:"DescribeLoadBalancersResult"`
	ResponseMetadata XMLResponseMetadata            `xml:"ResponseMetadata"`
}

// XMLDescribeLoadBalancersResult contains the result of DescribeLoadBalancers.
type XMLDescribeLoadBalancersResult struct {
	LoadBalancers XMLLoadBalancers `xml:"LoadBalancers"`
}

// XMLLoadBalancers contains a list of load balancers.
type XMLLoadBalancers struct {
	Members []XMLLoadBalancer `xml:"member"`
}

// XMLLoadBalancer represents a load balancer in XML format.
type XMLLoadBalancer struct {
	LoadBalancerArn       string               `xml:"LoadBalancerArn"`
	DNSName               string               `xml:"DNSName"`
	CanonicalHostedZoneID string               `xml:"CanonicalHostedZoneId"`
	CreatedTime           string               `xml:"CreatedTime"`
	LoadBalancerName      string               `xml:"LoadBalancerName"`
	Scheme                string               `xml:"Scheme"`
	VpcID                 string               `xml:"VpcId"`
	State                 XMLLoadBalancerState `xml:"State"`
	Type                  string               `xml:"Type"`
	AvailabilityZones     XMLAvailabilityZones `xml:"AvailabilityZones"`
	SecurityGroups        XMLSecurityGroups    `xml:"SecurityGroups"`
	IPAddressType         string               `xml:"IpAddressType"`
}

// XMLLoadBalancerState represents a load balancer state in XML format.
type XMLLoadBalancerState struct {
	Code   string `xml:"Code"`
	Reason string `xml:"Reason,omitempty"`
}

// XMLAvailabilityZones contains a list of availability zones.
type XMLAvailabilityZones struct {
	Members []XMLAvailabilityZone `xml:"member"`
}

// XMLAvailabilityZone represents an availability zone in XML format.
type XMLAvailabilityZone struct {
	ZoneName string `xml:"ZoneName"`
	SubnetID string `xml:"SubnetId"`
}

// XMLSecurityGroups contains a list of security groups.
type XMLSecurityGroups struct {
	Members []string `xml:"member"`
}

// XMLCreateTargetGroupResponse is the XML response for CreateTargetGroup.
type XMLCreateTargetGroupResponse struct {
	XMLName          xml.Name                   `xml:"CreateTargetGroupResponse"`
	Xmlns            string                     `xml:"xmlns,attr"`
	Result           XMLCreateTargetGroupResult `xml:"CreateTargetGroupResult"`
	ResponseMetadata XMLResponseMetadata        `xml:"ResponseMetadata"`
}

// XMLCreateTargetGroupResult contains the result of CreateTargetGroup.
type XMLCreateTargetGroupResult struct {
	TargetGroups XMLTargetGroups `xml:"TargetGroups"`
}

// XMLDeleteTargetGroupResponse is the XML response for DeleteTargetGroup.
type XMLDeleteTargetGroupResponse struct {
	XMLName          xml.Name                   `xml:"DeleteTargetGroupResponse"`
	Xmlns            string                     `xml:"xmlns,attr"`
	Result           XMLDeleteTargetGroupResult `xml:"DeleteTargetGroupResult"`
	ResponseMetadata XMLResponseMetadata        `xml:"ResponseMetadata"`
}

// XMLDeleteTargetGroupResult is an empty result for DeleteTargetGroup.
type XMLDeleteTargetGroupResult struct{}

// XMLDescribeTargetGroupsResponse is the XML response for DescribeTargetGroups.
type XMLDescribeTargetGroupsResponse struct {
	XMLName          xml.Name                      `xml:"DescribeTargetGroupsResponse"`
	Xmlns            string                        `xml:"xmlns,attr"`
	Result           XMLDescribeTargetGroupsResult `xml:"DescribeTargetGroupsResult"`
	ResponseMetadata XMLResponseMetadata           `xml:"ResponseMetadata"`
}

// XMLDescribeTargetGroupsResult contains the result of DescribeTargetGroups.
type XMLDescribeTargetGroupsResult struct {
	TargetGroups XMLTargetGroups `xml:"TargetGroups"`
}

// XMLDescribeTargetHealthResponse is the XML response for DescribeTargetHealth.
type XMLDescribeTargetHealthResponse struct {
	XMLName          xml.Name                      `xml:"DescribeTargetHealthResponse"`
	Xmlns            string                        `xml:"xmlns,attr"`
	Result           XMLDescribeTargetHealthResult `xml:"DescribeTargetHealthResult"`
	ResponseMetadata XMLResponseMetadata           `xml:"ResponseMetadata"`
}

// XMLDescribeTargetHealthResult contains the result of DescribeTargetHealth.
type XMLDescribeTargetHealthResult struct {
	TargetHealthDescriptions XMLTargetHealthDescriptions `xml:"TargetHealthDescriptions"`
}

// XMLTargetGroups contains a list of target groups.
type XMLTargetGroups struct {
	Members []XMLTargetGroup `xml:"member"`
}

// XMLTargetHealthDescriptions contains a list of target health descriptions.
type XMLTargetHealthDescriptions struct {
	Members []XMLTargetHealthDescription `xml:"member"`
}

// XMLTargetHealthDescription represents target health in XML format.
type XMLTargetHealthDescription struct {
	Target       XMLTarget       `xml:"Target"`
	TargetHealth XMLTargetHealth `xml:"TargetHealth"`
}

// XMLTarget represents a target in XML format.
type XMLTarget struct {
	ID               string `xml:"Id"`
	Port             int    `xml:"Port,omitempty"`
	AvailabilityZone string `xml:"AvailabilityZone,omitempty"`
}

// XMLTargetHealth represents target health in XML format.
type XMLTargetHealth struct {
	State       string `xml:"State"`
	Reason      string `xml:"Reason,omitempty"`
	Description string `xml:"Description,omitempty"`
}

// XMLTargetGroup represents a target group in XML format.
type XMLTargetGroup struct {
	TargetGroupArn             string              `xml:"TargetGroupArn"`
	TargetGroupName            string              `xml:"TargetGroupName"`
	Protocol                   string              `xml:"Protocol,omitempty"`
	ProtocolVersion            string              `xml:"ProtocolVersion,omitempty"`
	Port                       int                 `xml:"Port,omitempty"`
	VpcID                      string              `xml:"VpcId,omitempty"`
	IPAddressType              string              `xml:"IpAddressType,omitempty"`
	HealthCheckEnabled         bool                `xml:"HealthCheckEnabled"`
	HealthCheckIntervalSeconds int                 `xml:"HealthCheckIntervalSeconds"`
	HealthCheckPath            string              `xml:"HealthCheckPath,omitempty"`
	HealthCheckPort            string              `xml:"HealthCheckPort"`
	HealthCheckProtocol        string              `xml:"HealthCheckProtocol"`
	Matcher                    *XMLMatcher         `xml:"Matcher,omitempty"`
	HealthCheckTimeoutSeconds  int                 `xml:"HealthCheckTimeoutSeconds"`
	HealthyThresholdCount      int                 `xml:"HealthyThresholdCount"`
	UnhealthyThresholdCount    int                 `xml:"UnhealthyThresholdCount"`
	TargetType                 string              `xml:"TargetType"`
	LoadBalancerArns           XMLLoadBalancerArns `xml:"LoadBalancerArns"`
}

// XMLLoadBalancerArns contains a list of load balancer ARNs.
type XMLLoadBalancerArns struct {
	Members []string `xml:"member"`
}

// XMLRegisterTargetsResponse is the XML response for RegisterTargets.
type XMLRegisterTargetsResponse struct {
	XMLName          xml.Name                 `xml:"RegisterTargetsResponse"`
	Xmlns            string                   `xml:"xmlns,attr"`
	Result           XMLRegisterTargetsResult `xml:"RegisterTargetsResult"`
	ResponseMetadata XMLResponseMetadata      `xml:"ResponseMetadata"`
}

// XMLRegisterTargetsResult is an empty result for RegisterTargets.
type XMLRegisterTargetsResult struct{}

// XMLDeregisterTargetsResponse is the XML response for DeregisterTargets.
type XMLDeregisterTargetsResponse struct {
	XMLName          xml.Name                   `xml:"DeregisterTargetsResponse"`
	Xmlns            string                     `xml:"xmlns,attr"`
	Result           XMLDeregisterTargetsResult `xml:"DeregisterTargetsResult"`
	ResponseMetadata XMLResponseMetadata        `xml:"ResponseMetadata"`
}

// XMLDeregisterTargetsResult is an empty result for DeregisterTargets.
type XMLDeregisterTargetsResult struct{}

// XMLCreateListenerResponse is the XML response for CreateListener.
type XMLCreateListenerResponse struct {
	XMLName          xml.Name                `xml:"CreateListenerResponse"`
	Xmlns            string                  `xml:"xmlns,attr"`
	Result           XMLCreateListenerResult `xml:"CreateListenerResult"`
	ResponseMetadata XMLResponseMetadata     `xml:"ResponseMetadata"`
}

// XMLCreateListenerResult contains the result of CreateListener.
type XMLCreateListenerResult struct {
	Listeners XMLListeners `xml:"Listeners"`
}

// XMLCreateRuleResponse is the XML response for CreateRule.
type XMLCreateRuleResponse struct {
	XMLName          xml.Name            `xml:"CreateRuleResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	Result           XMLCreateRuleResult `xml:"CreateRuleResult"`
	ResponseMetadata XMLResponseMetadata `xml:"ResponseMetadata"`
}

// XMLCreateRuleResult contains the result of CreateRule.
type XMLCreateRuleResult struct {
	Rules XMLRules `xml:"Rules"`
}

// XMLDescribeRulesResponse is the XML response for DescribeRules.
type XMLDescribeRulesResponse struct {
	XMLName          xml.Name               `xml:"DescribeRulesResponse"`
	Xmlns            string                 `xml:"xmlns,attr"`
	Result           XMLDescribeRulesResult `xml:"DescribeRulesResult"`
	ResponseMetadata XMLResponseMetadata    `xml:"ResponseMetadata"`
}

// XMLDescribeRulesResult contains the result of DescribeRules.
type XMLDescribeRulesResult struct {
	Rules XMLRules `xml:"Rules"`
}

// XMLDescribeListenersResponse is the XML response for DescribeListeners.
type XMLDescribeListenersResponse struct {
	XMLName          xml.Name                   `xml:"DescribeListenersResponse"`
	Xmlns            string                     `xml:"xmlns,attr"`
	Result           XMLDescribeListenersResult `xml:"DescribeListenersResult"`
	ResponseMetadata XMLResponseMetadata        `xml:"ResponseMetadata"`
}

// XMLDescribeListenersResult contains the result of DescribeListeners.
type XMLDescribeListenersResult struct {
	Listeners XMLListeners `xml:"Listeners"`
}

// XMLDescribeListenerAttributesResponse is the XML response for DescribeListenerAttributes.
type XMLDescribeListenerAttributesResponse struct {
	XMLName          xml.Name                            `xml:"DescribeListenerAttributesResponse"`
	Xmlns            string                              `xml:"xmlns,attr"`
	Result           XMLDescribeListenerAttributesResult `xml:"DescribeListenerAttributesResult"`
	ResponseMetadata XMLResponseMetadata                 `xml:"ResponseMetadata"`
}

// XMLDescribeListenerAttributesResult contains the result of DescribeListenerAttributes.
type XMLDescribeListenerAttributesResult struct {
	Attributes XMLAttributes `xml:"Attributes"`
}

// XMLRules contains a list of rules.
type XMLRules struct {
	Members []XMLRule `xml:"member"`
}

// XMLRule represents a rule in XML format.
type XMLRule struct {
	RuleArn    string            `xml:"RuleArn"`
	Priority   string            `xml:"Priority"`
	IsDefault  bool              `xml:"IsDefault"`
	Actions    XMLActions        `xml:"Actions"`
	Conditions XMLRuleConditions `xml:"Conditions"`
}

// XMLDeleteListenerResponse is the XML response for DeleteListener.
type XMLDeleteListenerResponse struct {
	XMLName          xml.Name                `xml:"DeleteListenerResponse"`
	Xmlns            string                  `xml:"xmlns,attr"`
	Result           XMLDeleteListenerResult `xml:"DeleteListenerResult"`
	ResponseMetadata XMLResponseMetadata     `xml:"ResponseMetadata"`
}

// XMLDeleteListenerResult is an empty result for DeleteListener.
type XMLDeleteListenerResult struct{}

// XMLDescribeTagsResponse is the XML response for DescribeTags.
type XMLDescribeTagsResponse struct {
	XMLName          xml.Name              `xml:"DescribeTagsResponse"`
	Xmlns            string                `xml:"xmlns,attr"`
	Result           XMLDescribeTagsResult `xml:"DescribeTagsResult"`
	ResponseMetadata XMLResponseMetadata   `xml:"ResponseMetadata"`
}

// XMLDescribeTagsResult contains the result of DescribeTags.
type XMLDescribeTagsResult struct {
	TagDescriptions XMLTagDescriptions `xml:"TagDescriptions"`
}

// XMLTagDescriptions contains a list of resource tag descriptions.
type XMLTagDescriptions struct {
	Members []XMLTagDescription `xml:"member"`
}

// XMLTagDescription represents a resource's tags in XML format.
type XMLTagDescription struct {
	ResourceArn string  `xml:"ResourceArn"`
	Tags        XMLTags `xml:"Tags"`
}

// XMLTags contains a list of tags.
type XMLTags struct {
	Members []XMLTag `xml:"member"`
}

// XMLTag represents a tag in XML format.
type XMLTag struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// XMLMatcher represents a matcher in XML format.
type XMLMatcher struct {
	HTTPCode string `xml:"HttpCode,omitempty"`
	GRPCCode string `xml:"GrpcCode,omitempty"`
}

// XMLAddTagsResponse is the XML response for AddTags.
type XMLAddTagsResponse struct {
	XMLName          xml.Name            `xml:"AddTagsResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	Result           XMLAddTagsResult    `xml:"AddTagsResult"`
	ResponseMetadata XMLResponseMetadata `xml:"ResponseMetadata"`
}

// XMLAddTagsResult is an empty result for AddTags.
type XMLAddTagsResult struct{}

// XMLRemoveTagsResponse is the XML response for RemoveTags.
type XMLRemoveTagsResponse struct {
	XMLName          xml.Name            `xml:"RemoveTagsResponse"`
	Xmlns            string              `xml:"xmlns,attr"`
	Result           XMLRemoveTagsResult `xml:"RemoveTagsResult"`
	ResponseMetadata XMLResponseMetadata `xml:"ResponseMetadata"`
}

// XMLRemoveTagsResult is an empty result for RemoveTags.
type XMLRemoveTagsResult struct{}

// XMLDescribeTargetGroupAttributesResponse is the XML response for DescribeTargetGroupAttributes.
type XMLDescribeTargetGroupAttributesResponse struct {
	XMLName          xml.Name                               `xml:"DescribeTargetGroupAttributesResponse"`
	Xmlns            string                                 `xml:"xmlns,attr"`
	Result           XMLDescribeTargetGroupAttributesResult `xml:"DescribeTargetGroupAttributesResult"`
	ResponseMetadata XMLResponseMetadata                    `xml:"ResponseMetadata"`
}

// XMLDescribeTargetGroupAttributesResult contains the result of DescribeTargetGroupAttributes.
type XMLDescribeTargetGroupAttributesResult struct {
	Attributes XMLAttributes `xml:"Attributes"`
}

// XMLModifyTargetGroupAttributesResponse is the XML response for ModifyTargetGroupAttributes.
type XMLModifyTargetGroupAttributesResponse struct {
	XMLName          xml.Name                             `xml:"ModifyTargetGroupAttributesResponse"`
	Xmlns            string                               `xml:"xmlns,attr"`
	Result           XMLModifyTargetGroupAttributesResult `xml:"ModifyTargetGroupAttributesResult"`
	ResponseMetadata XMLResponseMetadata                  `xml:"ResponseMetadata"`
}

// XMLModifyTargetGroupAttributesResult contains the result of ModifyTargetGroupAttributes.
type XMLModifyTargetGroupAttributesResult struct {
	Attributes XMLAttributes `xml:"Attributes"`
}

// XMLDescribeLoadBalancerAttributesResponse is the XML response for DescribeLoadBalancerAttributes.
type XMLDescribeLoadBalancerAttributesResponse struct {
	XMLName          xml.Name                                `xml:"DescribeLoadBalancerAttributesResponse"`
	Xmlns            string                                  `xml:"xmlns,attr"`
	Result           XMLDescribeLoadBalancerAttributesResult `xml:"DescribeLoadBalancerAttributesResult"`
	ResponseMetadata XMLResponseMetadata                     `xml:"ResponseMetadata"`
}

// XMLDescribeLoadBalancerAttributesResult contains the result of DescribeLoadBalancerAttributes.
type XMLDescribeLoadBalancerAttributesResult struct {
	Attributes XMLAttributes `xml:"Attributes"`
}

// XMLModifyLoadBalancerAttributesResponse is the XML response for ModifyLoadBalancerAttributes.
type XMLModifyLoadBalancerAttributesResponse struct {
	XMLName          xml.Name                              `xml:"ModifyLoadBalancerAttributesResponse"`
	Xmlns            string                                `xml:"xmlns,attr"`
	Result           XMLModifyLoadBalancerAttributesResult `xml:"ModifyLoadBalancerAttributesResult"`
	ResponseMetadata XMLResponseMetadata                   `xml:"ResponseMetadata"`
}

// XMLModifyLoadBalancerAttributesResult contains the result of ModifyLoadBalancerAttributes.
type XMLModifyLoadBalancerAttributesResult struct {
	Attributes XMLAttributes `xml:"Attributes"`
}

// XMLAttributes contains a list of attributes.
type XMLAttributes struct {
	Members []XMLAttribute `xml:"member"`
}

// XMLAttribute represents an attribute in XML format.
type XMLAttribute struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// XMLListeners contains a list of listeners.
type XMLListeners struct {
	Members []XMLListener `xml:"member"`
}

// XMLListener represents a listener in XML format.
type XMLListener struct {
	ListenerArn     string     `xml:"ListenerArn"`
	LoadBalancerArn string     `xml:"LoadBalancerArn"`
	Port            int        `xml:"Port"`
	Protocol        string     `xml:"Protocol"`
	DefaultActions  XMLActions `xml:"DefaultActions"`
}

// XMLActions contains a list of actions.
type XMLActions struct {
	Members []XMLAction `xml:"member"`
}

// XMLAction represents an action in XML format.
type XMLAction struct {
	Type                string                  `xml:"Type"`
	TargetGroupArn      string                  `xml:"TargetGroupArn,omitempty"`
	Order               int                     `xml:"Order,omitempty"`
	ForwardConfig       *XMLForwardConfig       `xml:"ForwardConfig,omitempty"`
	FixedResponseConfig *XMLFixedResponseConfig `xml:"FixedResponseConfig,omitempty"`
}

// XMLForwardConfig represents a forward action configuration in XML.
type XMLForwardConfig struct {
	TargetGroups XMLTargetGroupTuples `xml:"TargetGroups"`
}

// XMLTargetGroupTuples contains a list of target group tuples.
type XMLTargetGroupTuples struct {
	Members []XMLTargetGroupTuple `xml:"member"`
}

// XMLTargetGroupTuple represents a target group tuple in XML.
type XMLTargetGroupTuple struct {
	TargetGroupArn string `xml:"TargetGroupArn"`
	Weight         int    `xml:"Weight,omitempty"`
}

// XMLFixedResponseConfig represents a fixed response action configuration in XML.
type XMLFixedResponseConfig struct {
	ContentType string `xml:"ContentType,omitempty"`
	MessageBody string `xml:"MessageBody,omitempty"`
	StatusCode  string `xml:"StatusCode,omitempty"`
}

// XMLRuleConditions contains a list of rule conditions.
type XMLRuleConditions struct {
	Members []XMLRuleCondition `xml:"member"`
}

// XMLRuleCondition represents a rule condition in XML.
type XMLRuleCondition struct {
	Field             string                         `xml:"Field"`
	Values            XMLStringMembers               `xml:"Values,omitempty"`
	PathPatternConfig *XMLPathPatternConditionConfig `xml:"PathPatternConfig,omitempty"`
}

// XMLPathPatternConditionConfig represents a path pattern condition config in XML.
type XMLPathPatternConditionConfig struct {
	Values XMLStringMembers `xml:"Values"`
}

// XMLStringMembers contains a list of string members.
type XMLStringMembers struct {
	Members []string `xml:"member"`
}

// XMLResponseMetadata contains response metadata.
type XMLResponseMetadata struct {
	RequestID string `xml:"RequestId"`
}

// XMLErrorResponse is the XML error response.
type XMLErrorResponse struct {
	XMLName   xml.Name `xml:"ErrorResponse"`
	Error     XMLError `xml:"Error"`
	RequestID string   `xml:"RequestId"`
}

// XMLError represents an error in XML format.
type XMLError struct {
	Type    string `xml:"Type"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

// Error represents an ELB error.
type Error struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}
