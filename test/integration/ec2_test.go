//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/sivchari/golden"
)

func newEC2Client(t *testing.T) *ec2.Client {
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

	return ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		o.BaseEndpoint = aws.String("http://127.0.0.1:4566")
	})
}

func TestEC2_RunAndDescribeInstances(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()

	// Run instances
	runResult, err := client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:      aws.String("ami-12345678"),
		InstanceType: types.InstanceTypeT2Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(2),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("InstanceId", "ReservationId", "RequesterId", "OwnerId", "LaunchTime", "PrivateIpAddress", "ResultMetadata")).Assert(t.Name()+"_run", runResult)

	instanceIDs := make([]string, 0, len(runResult.Instances))
	for _, inst := range runResult.Instances {
		instanceIDs = append(instanceIDs, *inst.InstanceId)
	}

	t.Cleanup(func() {
		_, _ = client.TerminateInstances(context.Background(), &ec2.TerminateInstancesInput{
			InstanceIds: instanceIDs,
		})
	})

	// Describe instances
	descResult, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: instanceIDs,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("InstanceId", "ReservationId", "RequesterId", "OwnerId", "LaunchTime", "PrivateIpAddress", "ResultMetadata")).Assert(t.Name()+"_describe", descResult)
}

func TestEC2_StartAndStopInstances(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()

	// Run instance
	runResult, err := client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:      aws.String("ami-12345678"),
		InstanceType: types.InstanceTypeT2Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
	})
	if err != nil {
		t.Fatalf("failed to run instance: %v", err)
	}

	instanceID := *runResult.Instances[0].InstanceId

	t.Cleanup(func() {
		_, _ = client.TerminateInstances(context.Background(), &ec2.TerminateInstancesInput{
			InstanceIds: []string{instanceID},
		})
	})

	// Stop instance
	stopResult, err := client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("InstanceId", "ResultMetadata")).Assert(t.Name()+"_stop", stopResult)

	// Start instance
	startResult, err := client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("InstanceId", "ResultMetadata")).Assert(t.Name()+"_start", startResult)
}

func TestEC2_TerminateInstances(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()

	// Run instance
	runResult, err := client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:      aws.String("ami-12345678"),
		InstanceType: types.InstanceTypeT2Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
	})
	if err != nil {
		t.Fatalf("failed to run instance: %v", err)
	}

	instanceID := *runResult.Instances[0].InstanceId

	// Terminate instance
	termResult, err := client.TerminateInstances(context.Background(), &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("InstanceId", "ResultMetadata")).Assert(t.Name(), termResult)
}

func TestEC2_CreateAndDeleteSecurityGroup(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()
	groupName := "test-security-group"

	// Create security group
	createResult, err := client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(groupName),
		Description: aws.String("Test security group"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("GroupId", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	groupID := *createResult.GroupId

	// Delete security group
	_, err = client.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(groupID),
	})
	if err != nil {
		t.Fatalf("failed to delete security group: %v", err)
	}
}

func TestEC2_DescribeSecurityGroups(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()

	vpcResult, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.1.0.0/16"),
	})
	if err != nil {
		t.Fatalf("failed to create VPC: %v", err)
	}

	vpcID := aws.ToString(vpcResult.Vpc.VpcId)
	t.Cleanup(func() {
		_, _ = client.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{
			VpcId: aws.String(vpcID),
		})
	})

	createResult, err := client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String("test-describe-security-group"),
		Description: aws.String("Test describe security group"),
		VpcId:       aws.String(vpcID),
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceType("security-group"),
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("test-describe-security-group")},
					{Key: aws.String("elbv2.k8s.aws/cluster"), Value: aws.String("kumo-e2e")},
					{Key: aws.String("elbv2.k8s.aws/resource"), Value: aws.String("backend-sg")},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create security group: %v", err)
	}

	groupID := aws.ToString(createResult.GroupId)
	t.Cleanup(func() {
		_, _ = client.DeleteSecurityGroup(context.Background(), &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(groupID),
		})
	})

	describeByIDResult, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{groupID},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertSingleSecurityGroup(t, describeByIDResult.SecurityGroups, groupID, vpcID, "test-describe-security-group")

	describeByFilterResult, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		Filters: []types.Filter{
			{Name: aws.String("vpc-id"), Values: []string{vpcID}},
			{Name: aws.String("tag:elbv2.k8s.aws/resource"), Values: []string{"backend-sg"}},
			{Name: aws.String("tag-key"), Values: []string{"elbv2.k8s.aws/cluster"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertSingleSecurityGroup(t, describeByFilterResult.SecurityGroups, groupID, vpcID, "test-describe-security-group")
}

func TestEC2_AuthorizeSecurityGroupIngress(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()
	groupName := "test-ingress-group"

	// Create security group
	createResult, err := client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(groupName),
		Description: aws.String("Test ingress security group"),
	})
	if err != nil {
		t.Fatalf("failed to create security group: %v", err)
	}

	groupID := *createResult.GroupId

	t.Cleanup(func() {
		_, _ = client.DeleteSecurityGroup(context.Background(), &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(groupID),
		})
	})

	// Authorize ingress
	ingressResult, err := client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(groupID),
		IpPermissions: []types.IpPermission{
			{
				IpProtocol: aws.String("tcp"),
				FromPort:   aws.Int32(22),
				ToPort:     aws.Int32(22),
				IpRanges: []types.IpRange{
					{
						CidrIp:      aws.String("0.0.0.0/0"),
						Description: aws.String("SSH access"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("ResultMetadata")).Assert(t.Name(), ingressResult)
}

func assertSingleSecurityGroup(t *testing.T, securityGroups []types.SecurityGroup, expectedGroupID, expectedVpcID, expectedName string) {
	t.Helper()

	if len(securityGroups) != 1 {
		t.Fatalf("unexpected security group count: got %d want 1", len(securityGroups))
	}

	securityGroup := securityGroups[0]
	if got := aws.ToString(securityGroup.GroupId); got != expectedGroupID {
		t.Fatalf("unexpected security group ID: got %q want %q", got, expectedGroupID)
	}
	if got := aws.ToString(securityGroup.GroupName); got != expectedName {
		t.Fatalf("unexpected security group name: got %q want %q", got, expectedName)
	}
	if got := aws.ToString(securityGroup.Description); got != "Test describe security group" {
		t.Fatalf("unexpected security group description: got %q", got)
	}
	if got := aws.ToString(securityGroup.OwnerId); got != "000000000000" {
		t.Fatalf("unexpected security group owner ID: got %q want %q", got, "000000000000")
	}
	if got := aws.ToString(securityGroup.VpcId); got != expectedVpcID {
		t.Fatalf("unexpected security group VPC ID: got %q want %q", got, expectedVpcID)
	}
	if len(securityGroup.IpPermissions) != 0 {
		t.Fatalf("unexpected ingress permission count: got %d want 0", len(securityGroup.IpPermissions))
	}
	if len(securityGroup.IpPermissionsEgress) != 0 {
		t.Fatalf("unexpected egress permission count: got %d want 0", len(securityGroup.IpPermissionsEgress))
	}

	expectedTags := map[string]string{
		"Name":                   "test-describe-security-group",
		"elbv2.k8s.aws/cluster":  "kumo-e2e",
		"elbv2.k8s.aws/resource": "backend-sg",
	}
	if len(securityGroup.Tags) != len(expectedTags) {
		t.Fatalf("unexpected tag count: got %d want %d", len(securityGroup.Tags), len(expectedTags))
	}
	for _, tag := range securityGroup.Tags {
		key := aws.ToString(tag.Key)
		if value, ok := expectedTags[key]; !ok || aws.ToString(tag.Value) != value {
			t.Fatalf("unexpected tag: %q=%q", key, aws.ToString(tag.Value))
		}
	}
}

func assertSecurityGroupHasTags(t *testing.T, securityGroups []types.SecurityGroup, expectedGroupID string, expectedTags map[string]string) {
	t.Helper()

	if len(securityGroups) != 1 {
		t.Fatalf("unexpected security group count: got %d want 1", len(securityGroups))
	}

	securityGroup := securityGroups[0]
	if got := aws.ToString(securityGroup.GroupId); got != expectedGroupID {
		t.Fatalf("unexpected security group ID: got %q want %q", got, expectedGroupID)
	}
	if len(securityGroup.Tags) != len(expectedTags) {
		t.Fatalf("unexpected tag count: got %d want %d", len(securityGroup.Tags), len(expectedTags))
	}

	for _, tag := range securityGroup.Tags {
		key := aws.ToString(tag.Key)
		if value, ok := expectedTags[key]; !ok || aws.ToString(tag.Value) != value {
			t.Fatalf("unexpected tag: %q=%q", key, aws.ToString(tag.Value))
		}
	}
}

func TestEC2_CreateAndDeleteTags(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()

	vpcResult, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.2.0.0/16"),
	})
	if err != nil {
		t.Fatalf("failed to create VPC: %v", err)
	}

	vpcID := aws.ToString(vpcResult.Vpc.VpcId)
	t.Cleanup(func() {
		_, _ = client.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{
			VpcId: aws.String(vpcID),
		})
	})

	createResult, err := client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String("test-tag-security-group"),
		Description: aws.String("Test tag security group"),
		VpcId:       aws.String(vpcID),
	})
	if err != nil {
		t.Fatalf("failed to create security group: %v", err)
	}

	groupID := aws.ToString(createResult.GroupId)
	t.Cleanup(func() {
		_, _ = client.DeleteSecurityGroup(context.Background(), &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(groupID),
		})
	})

	_, err = client.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{groupID},
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String("test-tag-security-group")},
			{Key: aws.String("stage"), Value: aws.String("e2e")},
		},
	})
	if err != nil {
		t.Fatalf("failed to create tags: %v", err)
	}

	describeAfterCreateTagsResult, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{groupID},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertSecurityGroupHasTags(t, describeAfterCreateTagsResult.SecurityGroups, groupID, map[string]string{
		"Name":  "test-tag-security-group",
		"stage": "e2e",
	})

	_, err = client.DeleteTags(ctx, &ec2.DeleteTagsInput{
		Resources: []string{groupID},
		Tags: []types.Tag{
			{Key: aws.String("stage"), Value: aws.String("e2e")},
		},
	})
	if err != nil {
		t.Fatalf("failed to delete tags: %v", err)
	}

	describeAfterDeleteTagsResult, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{groupID},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertSecurityGroupHasTags(t, describeAfterDeleteTagsResult.SecurityGroups, groupID, map[string]string{
		"Name": "test-tag-security-group",
	})
}

func TestEC2_CreateAndDeleteKeyPair(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()
	keyName := "test-key-pair"

	// Create key pair
	createResult, err := client.CreateKeyPair(ctx, &ec2.CreateKeyPairInput{
		KeyName: aws.String(keyName),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("KeyPairId", "KeyFingerprint", "KeyMaterial", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	t.Cleanup(func() {
		_, _ = client.DeleteKeyPair(context.Background(), &ec2.DeleteKeyPairInput{
			KeyName: aws.String(keyName),
		})
	})

	// Delete key pair
	_, err = client.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{
		KeyName: aws.String(keyName),
	})
	if err != nil {
		t.Fatalf("failed to delete key pair: %v", err)
	}
}

func TestEC2_DescribeKeyPairs(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()
	keyName := "test-describe-key-pair"

	// Create key pair
	_, err := client.CreateKeyPair(ctx, &ec2.CreateKeyPairInput{
		KeyName: aws.String(keyName),
	})
	if err != nil {
		t.Fatalf("failed to create key pair: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DeleteKeyPair(context.Background(), &ec2.DeleteKeyPairInput{
			KeyName: aws.String(keyName),
		})
	})

	// Describe key pairs
	descResult, err := client.DescribeKeyPairs(ctx, &ec2.DescribeKeyPairsInput{
		KeyNames: []string{keyName},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("KeyPairId", "KeyFingerprint", "ResultMetadata")).Assert(t.Name(), descResult)
}

func TestEC2_CreateAndDeleteVpc(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()

	// Create VPC
	createResult, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("VpcId", "OwnerId", "ResultMetadata")).Assert(t.Name()+"_create", createResult)

	vpcID := *createResult.Vpc.VpcId

	// Delete VPC
	_, err = client.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{
		VpcId: aws.String(vpcID),
	})
	if err != nil {
		t.Fatalf("failed to delete VPC: %v", err)
	}
}

func TestEC2_DescribeVpcs(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()

	// Create VPC
	createResult, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
	})
	if err != nil {
		t.Fatalf("failed to create VPC: %v", err)
	}

	vpcID := *createResult.Vpc.VpcId

	t.Cleanup(func() {
		_, _ = client.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{
			VpcId: aws.String(vpcID),
		})
	})

	// Describe VPCs
	descResult, err := client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []string{vpcID},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("VpcId", "OwnerId", "ResultMetadata")).Assert(t.Name(), descResult)
}

func TestEC2_CreateAndDeleteSubnet(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()

	// Create VPC first
	vpcResult, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
	})
	if err != nil {
		t.Fatalf("failed to create VPC: %v", err)
	}

	vpcID := *vpcResult.Vpc.VpcId

	t.Cleanup(func() {
		_, _ = client.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{
			VpcId: aws.String(vpcID),
		})
	})

	// Create Subnet
	subnetResult, err := client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId:     aws.String(vpcID),
		CidrBlock: aws.String("10.0.1.0/24"),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("SubnetId", "SubnetArn", "VpcId", "OwnerId", "AvailabilityZoneId", "ResultMetadata")).Assert(t.Name()+"_create", subnetResult)

	subnetID := *subnetResult.Subnet.SubnetId

	// Delete Subnet
	_, err = client.DeleteSubnet(ctx, &ec2.DeleteSubnetInput{
		SubnetId: aws.String(subnetID),
	})
	if err != nil {
		t.Fatalf("failed to delete subnet: %v", err)
	}
}

func TestEC2_CreateInternetGatewayAndAttach(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()

	// Create VPC first
	vpcResult, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
	})
	if err != nil {
		t.Fatalf("failed to create VPC: %v", err)
	}

	vpcID := *vpcResult.Vpc.VpcId

	// Create Internet Gateway
	igwResult, err := client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("InternetGatewayId", "OwnerId", "ResultMetadata")).Assert(t.Name()+"_create_igw", igwResult)

	igwID := *igwResult.InternetGateway.InternetGatewayId

	t.Cleanup(func() {
		_, _ = client.DetachInternetGateway(context.Background(), &ec2.DetachInternetGatewayInput{
			InternetGatewayId: aws.String(igwID),
			VpcId:             aws.String(vpcID),
		})
		_, _ = client.DeleteInternetGateway(context.Background(), &ec2.DeleteInternetGatewayInput{
			InternetGatewayId: aws.String(igwID),
		})
		_, _ = client.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{
			VpcId: aws.String(vpcID),
		})
	})

	// Attach Internet Gateway to VPC
	_, err = client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(igwID),
		VpcId:             aws.String(vpcID),
	})
	if err != nil {
		t.Fatalf("failed to attach internet gateway: %v", err)
	}

	// Describe Internet Gateways
	descResult, err := client.DescribeInternetGateways(ctx, &ec2.DescribeInternetGatewaysInput{
		InternetGatewayIds: []string{igwID},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("InternetGatewayId", "OwnerId", "VpcId", "ResultMetadata")).Assert(t.Name()+"_describe", descResult)
}

func TestEC2_CreateRouteTableAndAssociate(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()

	// Create VPC first
	vpcResult, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
	})
	if err != nil {
		t.Fatalf("failed to create VPC: %v", err)
	}

	vpcID := *vpcResult.Vpc.VpcId

	// Create Subnet
	subnetResult, err := client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId:     aws.String(vpcID),
		CidrBlock: aws.String("10.0.1.0/24"),
	})
	if err != nil {
		t.Fatalf("failed to create subnet: %v", err)
	}

	subnetID := *subnetResult.Subnet.SubnetId

	t.Cleanup(func() {
		_, _ = client.DeleteSubnet(context.Background(), &ec2.DeleteSubnetInput{
			SubnetId: aws.String(subnetID),
		})
		_, _ = client.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{
			VpcId: aws.String(vpcID),
		})
	})

	// Create Route Table
	rtResult, err := client.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
		VpcId: aws.String(vpcID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("RouteTableId", "VpcId", "OwnerId", "ResultMetadata")).Assert(t.Name()+"_create_rt", rtResult)

	rtID := *rtResult.RouteTable.RouteTableId

	// Associate Route Table with Subnet
	assocResult, err := client.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
		RouteTableId: aws.String(rtID),
		SubnetId:     aws.String(subnetID),
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("AssociationId", "ResultMetadata")).Assert(t.Name()+"_associate", assocResult)
}

func TestEC2_CreateRoute(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()

	// Create VPC first
	vpcResult, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
	})
	if err != nil {
		t.Fatalf("failed to create VPC: %v", err)
	}

	vpcID := *vpcResult.Vpc.VpcId

	// Create Internet Gateway
	igwResult, err := client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{})
	if err != nil {
		t.Fatalf("failed to create internet gateway: %v", err)
	}

	igwID := *igwResult.InternetGateway.InternetGatewayId

	// Attach Internet Gateway to VPC
	_, err = client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: aws.String(igwID),
		VpcId:             aws.String(vpcID),
	})
	if err != nil {
		t.Fatalf("failed to attach internet gateway: %v", err)
	}

	t.Cleanup(func() {
		_, _ = client.DetachInternetGateway(context.Background(), &ec2.DetachInternetGatewayInput{
			InternetGatewayId: aws.String(igwID),
			VpcId:             aws.String(vpcID),
		})
		_, _ = client.DeleteInternetGateway(context.Background(), &ec2.DeleteInternetGatewayInput{
			InternetGatewayId: aws.String(igwID),
		})
		_, _ = client.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{
			VpcId: aws.String(vpcID),
		})
	})

	// Create Route Table
	rtResult, err := client.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
		VpcId: aws.String(vpcID),
	})
	if err != nil {
		t.Fatalf("failed to create route table: %v", err)
	}

	rtID := *rtResult.RouteTable.RouteTableId

	// Create Route
	_, err = client.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:         aws.String(rtID),
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            aws.String(igwID),
	})
	if err != nil {
		t.Fatalf("failed to create route: %v", err)
	}

	// Describe Route Tables
	descResult, err := client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		RouteTableIds: []string{rtID},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("RouteTableId", "VpcId", "OwnerId", "GatewayId", "ResultMetadata")).Assert(t.Name()+"_describe", descResult)
}

func TestEC2_CreateNatGateway(t *testing.T) {
	client := newEC2Client(t)
	ctx := t.Context()

	// Create VPC first
	vpcResult, err := client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: aws.String("10.0.0.0/16"),
	})
	if err != nil {
		t.Fatalf("failed to create VPC: %v", err)
	}

	vpcID := *vpcResult.Vpc.VpcId

	// Create Subnet
	subnetResult, err := client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId:     aws.String(vpcID),
		CidrBlock: aws.String("10.0.1.0/24"),
	})
	if err != nil {
		t.Fatalf("failed to create subnet: %v", err)
	}

	subnetID := *subnetResult.Subnet.SubnetId

	t.Cleanup(func() {
		_, _ = client.DeleteSubnet(context.Background(), &ec2.DeleteSubnetInput{
			SubnetId: aws.String(subnetID),
		})
		_, _ = client.DeleteVpc(context.Background(), &ec2.DeleteVpcInput{
			VpcId: aws.String(vpcID),
		})
	})

	// Create NAT Gateway (private connectivity type - no EIP required)
	natgwResult, err := client.CreateNatGateway(ctx, &ec2.CreateNatGatewayInput{
		SubnetId:         aws.String(subnetID),
		ConnectivityType: types.ConnectivityTypePrivate,
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("NatGatewayId", "SubnetId", "VpcId", "CreateTime", "ResultMetadata")).Assert(t.Name()+"_create", natgwResult)

	natgwID := *natgwResult.NatGateway.NatGatewayId

	// Describe NAT Gateways
	descResult, err := client.DescribeNatGateways(ctx, &ec2.DescribeNatGatewaysInput{
		NatGatewayIds: []string{natgwID},
	})
	if err != nil {
		t.Fatal(err)
	}
	golden.New(t, golden.WithIgnoreFields("NatGatewayId", "SubnetId", "VpcId", "CreateTime", "ResultMetadata")).Assert(t.Name()+"_describe", descResult)
}
