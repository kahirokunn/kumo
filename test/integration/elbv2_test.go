//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/sivchari/golden"
)

func newELBv2Client(t *testing.T) *elasticloadbalancingv2.Client {
	t.Helper()

	cfg, err := config.LoadDefaultConfig(t.Context(),
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			"test", "test", "",
		)),
	)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	return elasticloadbalancingv2.NewFromConfig(cfg, func(o *elasticloadbalancingv2.Options) {
		o.BaseEndpoint = aws.String("http://localhost:4566")
	})
}

func TestELBv2_CreateAndDeleteLoadBalancer(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	lbName := "test-load-balancer"

	// Create load balancer
	createResult, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String(lbName),
		Subnets: []string{"subnet-12345678", "subnet-87654321"},
		Type:    types.LoadBalancerTypeEnumApplication,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("LoadBalancerArn", "DNSName", "CanonicalHostedZoneId", "CreatedTime", "VpcId", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	lb := createResult.LoadBalancers[0]

	t.Cleanup(func() {
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lb.LoadBalancerArn,
		})
	})

	// Delete load balancer
	_, err = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
		LoadBalancerArn: lb.LoadBalancerArn,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestELBv2_DescribeLoadBalancers(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	lbName := "test-describe-lb"

	// Create load balancer
	createResult, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String(lbName),
		Subnets: []string{"subnet-12345678"},
	})
	if err != nil {
		t.Fatal(err)
	}

	lbArn := createResult.LoadBalancers[0].LoadBalancerArn

	t.Cleanup(func() {
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lbArn,
		})
	})

	// Describe load balancers by ARN
	descResult, err := client.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{
		LoadBalancerArns: []string{*lbArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("LoadBalancerArn", "DNSName", "CanonicalHostedZoneId", "CreatedTime", "VpcId", "ResultMetadata")).Assert(t.Name()+"_describe", descResult)
}

func TestELBv2_TagOperations(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	lbName := "test-tags-lb"

	createResult, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String(lbName),
		Subnets: []string{"subnet-12345678"},
		Tags: []types.Tag{
			{Key: aws.String("env"), Value: aws.String("test")},
			{Key: aws.String("owner"), Value: aws.String("kumo")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	lbArn := createResult.LoadBalancers[0].LoadBalancerArn

	t.Cleanup(func() {
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lbArn,
		})
	})

	describeResult, err := client.DescribeTags(ctx, &elasticloadbalancingv2.DescribeTagsInput{
		ResourceArns: []string{aws.ToString(lbArn)},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertELBv2Tags(t, describeResult.TagDescriptions, aws.ToString(lbArn), map[string]string{
		"env":   "test",
		"owner": "kumo",
	})

	_, err = client.AddTags(ctx, &elasticloadbalancingv2.AddTagsInput{
		ResourceArns: []string{aws.ToString(lbArn)},
		Tags: []types.Tag{
			{Key: aws.String("owner"), Value: aws.String("platform")},
			{Key: aws.String("team"), Value: aws.String("agent")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	describeAfterAddResult, err := client.DescribeTags(ctx, &elasticloadbalancingv2.DescribeTagsInput{
		ResourceArns: []string{aws.ToString(lbArn)},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertELBv2Tags(t, describeAfterAddResult.TagDescriptions, aws.ToString(lbArn), map[string]string{
		"env":   "test",
		"owner": "platform",
		"team":  "agent",
	})

	_, err = client.RemoveTags(ctx, &elasticloadbalancingv2.RemoveTagsInput{
		ResourceArns: []string{aws.ToString(lbArn)},
		TagKeys:      []string{"env"},
	})
	if err != nil {
		t.Fatal(err)
	}

	describeAfterRemoveResult, err := client.DescribeTags(ctx, &elasticloadbalancingv2.DescribeTagsInput{
		ResourceArns: []string{aws.ToString(lbArn)},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertELBv2Tags(t, describeAfterRemoveResult.TagDescriptions, aws.ToString(lbArn), map[string]string{
		"owner": "platform",
		"team":  "agent",
	})
}

func TestELBv2_CreateAndDeleteTargetGroup(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	tgName := "test-target-group"

	// Create target group
	createResult, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:       aws.String(tgName),
		Protocol:   types.ProtocolEnumHttp,
		Port:       aws.Int32(80),
		VpcId:      aws.String("vpc-12345678"),
		TargetType: types.TargetTypeEnumInstance,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("TargetGroupArn", "LoadBalancerArns", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	tg := createResult.TargetGroups[0]

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tg.TargetGroupArn,
		})
	})

	// Delete target group
	_, err = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
		TargetGroupArn: tg.TargetGroupArn,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertELBv2Tags(t *testing.T, descriptions []types.TagDescription, expectedResourceArn string, expectedTags map[string]string) {
	t.Helper()

	if len(descriptions) != 1 {
		t.Fatalf("unexpected tag description count: got %d want 1", len(descriptions))
	}

	description := descriptions[0]
	if aws.ToString(description.ResourceArn) != expectedResourceArn {
		t.Fatalf("unexpected resource ARN: got %q want %q", aws.ToString(description.ResourceArn), expectedResourceArn)
	}

	if len(description.Tags) != len(expectedTags) {
		t.Fatalf("unexpected tag count: got %d want %d", len(description.Tags), len(expectedTags))
	}

	for _, tag := range description.Tags {
		key := aws.ToString(tag.Key)
		if value, ok := expectedTags[key]; !ok || aws.ToString(tag.Value) != value {
			t.Fatalf("unexpected tag: got %q=%q", key, aws.ToString(tag.Value))
		}
	}
}

func TestELBv2_DescribeTargetGroups(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	tgName := "test-describe-tg"

	// Create target group
	createResult, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:       aws.String(tgName),
		Protocol:   types.ProtocolEnumHttp,
		Port:       aws.Int32(80),
		VpcId:      aws.String("vpc-12345678"),
		TargetType: types.TargetTypeEnumInstance,
	})
	if err != nil {
		t.Fatal(err)
	}

	tgArn := createResult.TargetGroups[0].TargetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
	})

	// Describe target groups
	descResult, err := client.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
		TargetGroupArns: []string{*tgArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("TargetGroupArn", "LoadBalancerArns", "ResultMetadata")).Assert(t.Name()+"_describe", descResult)
}

func TestELBv2_DescribeTags(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()

	createResult, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String("test-describe-tags-lb"),
		Subnets: []string{"subnet-12345678", "subnet-87654321"},
		Tags: []types.Tag{
			{Key: aws.String("Environment"), Value: aws.String("test")},
			{Key: aws.String("Team"), Value: aws.String("platform")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	lbArn := createResult.LoadBalancers[0].LoadBalancerArn
	t.Cleanup(func() {
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lbArn,
		})
	})

	describeResult, err := client.DescribeTags(ctx, &elasticloadbalancingv2.DescribeTagsInput{
		ResourceArns: []string{aws.ToString(lbArn)},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(describeResult.TagDescriptions) != 1 {
		t.Fatalf("unexpected tag description count: got %d want 1", len(describeResult.TagDescriptions))
	}

	tagDescription := describeResult.TagDescriptions[0]
	if aws.ToString(tagDescription.ResourceArn) != aws.ToString(lbArn) {
		t.Fatalf("unexpected resource ARN: got %q want %q", aws.ToString(tagDescription.ResourceArn), aws.ToString(lbArn))
	}

	expectedTags := map[string]string{
		"Environment": "test",
		"Team":        "platform",
	}
	if len(tagDescription.Tags) != len(expectedTags) {
		t.Fatalf("unexpected tag count: got %d want %d", len(tagDescription.Tags), len(expectedTags))
	}

	for _, tag := range tagDescription.Tags {
		key := aws.ToString(tag.Key)
		value := aws.ToString(tag.Value)
		if expectedValue, ok := expectedTags[key]; !ok || value != expectedValue {
			t.Fatalf("unexpected tag: %q=%q", key, value)
		}
	}
}

func TestELBv2_AddAndRemoveTags(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()

	createResult, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:       aws.String("test-add-remove-tags-tg"),
		Protocol:   types.ProtocolEnumHttp,
		Port:       aws.Int32(80),
		VpcId:      aws.String("vpc-12345678"),
		TargetType: types.TargetTypeEnumIp,
	})
	if err != nil {
		t.Fatal(err)
	}

	tgArn := createResult.TargetGroups[0].TargetGroupArn
	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
	})

	_, err = client.AddTags(ctx, &elasticloadbalancingv2.AddTagsInput{
		ResourceArns: []string{aws.ToString(tgArn)},
		Tags: []types.Tag{
			{Key: aws.String("Environment"), Value: aws.String("test")},
			{Key: aws.String("Service"), Value: aws.String("nginx")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	describeAfterAdd, err := client.DescribeTags(ctx, &elasticloadbalancingv2.DescribeTagsInput{
		ResourceArns: []string{aws.ToString(tgArn)},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertELBv2Tags(t, describeAfterAdd.TagDescriptions, aws.ToString(tgArn), map[string]string{
		"Environment": "test",
		"Service":     "nginx",
	})

	_, err = client.RemoveTags(ctx, &elasticloadbalancingv2.RemoveTagsInput{
		ResourceArns: []string{aws.ToString(tgArn)},
		TagKeys:      []string{"Environment"},
	})
	if err != nil {
		t.Fatal(err)
	}

	describeAfterRemove, err := client.DescribeTags(ctx, &elasticloadbalancingv2.DescribeTagsInput{
		ResourceArns: []string{aws.ToString(tgArn)},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertELBv2Tags(t, describeAfterRemove.TagDescriptions, aws.ToString(tgArn), map[string]string{
		"Service": "nginx",
	})
}

func TestELBv2_RegisterAndDeregisterTargets(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	tgName := "test-register-targets"

	// Create target group
	createResult, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:       aws.String(tgName),
		Protocol:   types.ProtocolEnumHttp,
		Port:       aws.Int32(80),
		VpcId:      aws.String("vpc-12345678"),
		TargetType: types.TargetTypeEnumInstance,
	})
	if err != nil {
		t.Fatal(err)
	}

	tgArn := createResult.TargetGroups[0].TargetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
	})

	// Register targets
	_, err = client.RegisterTargets(ctx, &elasticloadbalancingv2.RegisterTargetsInput{
		TargetGroupArn: tgArn,
		Targets: []types.TargetDescription{
			{Id: aws.String("i-12345678"), Port: aws.Int32(80)},
			{Id: aws.String("i-87654321"), Port: aws.Int32(80)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Deregister targets
	_, err = client.DeregisterTargets(ctx, &elasticloadbalancingv2.DeregisterTargetsInput{
		TargetGroupArn: tgArn,
		Targets: []types.TargetDescription{
			{Id: aws.String("i-12345678")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}


func TestELBv2_DescribeTargetHealth(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()

	createResult, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:       aws.String("test-target-health-tg"),
		Protocol:   types.ProtocolEnumHttp,
		Port:       aws.Int32(80),
		VpcId:      aws.String("vpc-12345678"),
		TargetType: types.TargetTypeEnumIp,
	})
	if err != nil {
		t.Fatal(err)
	}

	tgArn := createResult.TargetGroups[0].TargetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
	})

	_, err = client.RegisterTargets(ctx, &elasticloadbalancingv2.RegisterTargetsInput{
		TargetGroupArn: tgArn,
		Targets: []types.TargetDescription{
			{Id: aws.String("10.0.1.10"), Port: aws.Int32(80)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	describeResult, err := client.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: tgArn,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name()+"_describe", describeResult)
}

func TestELBv2_CreateAndDeleteListener(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()
	lbName := "test-listener-lb"
	tgName := "test-listener-tg"

	// Create load balancer
	lbResult, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String(lbName),
		Subnets: []string{"subnet-12345678"},
	})
	if err != nil {
		t.Fatal(err)
	}

	lbArn := lbResult.LoadBalancers[0].LoadBalancerArn

	// Create target group
	tgResult, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:       aws.String(tgName),
		Protocol:   types.ProtocolEnumHttp,
		Port:       aws.Int32(80),
		VpcId:      aws.String("vpc-12345678"),
		TargetType: types.TargetTypeEnumInstance,
	})
	if err != nil {
		t.Fatal(err)
	}

	tgArn := tgResult.TargetGroups[0].TargetGroupArn

	t.Cleanup(func() {
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lbArn,
		})
	})

	// Create listener
	listenerResult, err := client.CreateListener(ctx, &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: lbArn,
		Port:            aws.Int32(80),
		Protocol:        types.ProtocolEnumHttp,
		DefaultActions: []types.Action{
			{
				Type:           types.ActionTypeEnumForward,
				TargetGroupArn: tgArn,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("ListenerArn", "LoadBalancerArn", "TargetGroupArn", "ResultMetadata")).Assert(t.Name()+"_create", listenerResult)

	listenerArn := listenerResult.Listeners[0].ListenerArn

	// Delete listener
	_, err = client.DeleteListener(context.Background(), &elasticloadbalancingv2.DeleteListenerInput{
		ListenerArn: listenerArn,
	})
	if err != nil {
		t.Fatal(err)
	}
}


func TestELBv2_LoadBalancerWithTargetGroupAndListener(t *testing.T) {
	client := newELBv2Client(t)
	ctx := t.Context()

	// Create load balancer
	lbResult, err := client.CreateLoadBalancer(ctx, &elasticloadbalancingv2.CreateLoadBalancerInput{
		Name:    aws.String("test-full-lb"),
		Subnets: []string{"subnet-12345678", "subnet-87654321"},
		Type:    types.LoadBalancerTypeEnumApplication,
	})
	if err != nil {
		t.Fatal(err)
	}

	lbArn := lbResult.LoadBalancers[0].LoadBalancerArn

	// Create target group
	tgResult, err := client.CreateTargetGroup(ctx, &elasticloadbalancingv2.CreateTargetGroupInput{
		Name:       aws.String("test-full-tg"),
		Protocol:   types.ProtocolEnumHttp,
		Port:       aws.Int32(80),
		VpcId:      aws.String("vpc-12345678"),
		TargetType: types.TargetTypeEnumInstance,
	})
	if err != nil {
		t.Fatal(err)
	}

	tgArn := tgResult.TargetGroups[0].TargetGroupArn

	// Register targets
	_, err = client.RegisterTargets(ctx, &elasticloadbalancingv2.RegisterTargetsInput{
		TargetGroupArn: tgArn,
		Targets: []types.TargetDescription{
			{Id: aws.String("i-12345678"), Port: aws.Int32(80)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create listener
	listenerResult, err := client.CreateListener(ctx, &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: lbArn,
		Port:            aws.Int32(80),
		Protocol:        types.ProtocolEnumHttp,
		DefaultActions: []types.Action{
			{
				Type:           types.ActionTypeEnumForward,
				TargetGroupArn: tgArn,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	listenerArn := listenerResult.Listeners[0].ListenerArn

	// Cleanup in reverse order
	t.Cleanup(func() {
		_, _ = client.DeleteListener(context.Background(), &elasticloadbalancingv2.DeleteListenerInput{
			ListenerArn: listenerArn,
		})
		_, _ = client.DeleteTargetGroup(context.Background(), &elasticloadbalancingv2.DeleteTargetGroupInput{
			TargetGroupArn: tgArn,
		})
		_, _ = client.DeleteLoadBalancer(context.Background(), &elasticloadbalancingv2.DeleteLoadBalancerInput{
			LoadBalancerArn: lbArn,
		})
	})

	// Verify everything is created
	descLbResult, err := client.DescribeLoadBalancers(ctx, &elasticloadbalancingv2.DescribeLoadBalancersInput{
		LoadBalancerArns: []string{*lbArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("LoadBalancerArn", "DNSName", "CanonicalHostedZoneId", "CreatedTime", "VpcId", "SubnetId", "ResultMetadata")).Assert(t.Name()+"_describe_lb", descLbResult)

	descTgResult, err := client.DescribeTargetGroups(ctx, &elasticloadbalancingv2.DescribeTargetGroupsInput{
		TargetGroupArns: []string{*tgArn},
	})
	if err != nil {
		t.Fatal(err)
	}

	golden.New(t, golden.WithIgnoreFields("TargetGroupArn", "LoadBalancerArns", "ResultMetadata")).Assert(t.Name()+"_describe_tg", descTgResult)
}
