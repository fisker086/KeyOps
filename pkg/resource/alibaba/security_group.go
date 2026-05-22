package alibaba

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/fisker086/keyops/pkg/resource"
)

func (p *Provider) createSecurityGroup(ctx context.Context, r *resource.Resource) (*resource.ResourceState, error) {
	client, err := ecs.NewClientWithAccessKey(p.region, p.accessKey, p.secretKey)
	if err != nil {
		return nil, fmt.Errorf("create ecs client: %w", err)
	}

	request := ecs.CreateCreateSecurityGroupRequest()
	request.Scheme = "https"

	if v, ok := r.Config["name"].(string); ok {
		request.SecurityGroupName = v
	}
	if v, ok := r.Config["description"].(string); ok {
		request.Description = v
	}
	if v, ok := r.Config["vpc_id"].(string); ok {
		request.VpcId = v
	}

	response, err := client.CreateSecurityGroup(request)
	if err != nil {
		return nil, fmt.Errorf("create security group: %w", err)
	}

	props, _ := json.Marshal(map[string]interface{}{
		"security_group_id": response.SecurityGroupId,
		"name":              request.SecurityGroupName,
		"vpc_id":            request.VpcId,
	})

	var properties map[string]interface{}
	json.Unmarshal(props, &properties)

	return &resource.ResourceState{
		Type:       "alicloud_security_group",
		Name:       r.Name,
		ResourceID: response.SecurityGroupId,
		Properties: properties,
		Status:     "created",
	}, nil
}

func (p *Provider) readSecurityGroup(ctx context.Context, state *resource.ResourceState) (*resource.ResourceState, error) {
	client, err := ecs.NewClientWithAccessKey(p.region, p.accessKey, p.secretKey)
	if err != nil {
		return nil, err
	}

	request := ecs.CreateDescribeSecurityGroupAttributeRequest()
	request.Scheme = "https"
	request.SecurityGroupId = state.ResourceID

	response, err := client.DescribeSecurityGroupAttribute(request)
	if err != nil {
		return nil, fmt.Errorf("describe security group: %w", err)
	}

	props, _ := json.Marshal(map[string]interface{}{
		"security_group_id": response.SecurityGroupId,
		"name":              response.SecurityGroupName,
		"description":       response.Description,
		"vpc_id":            response.VpcId,
	})

	var properties map[string]interface{}
	json.Unmarshal(props, &properties)

	state.Properties = properties
	return state, nil
}

func (p *Provider) updateSecurityGroup(ctx context.Context, desired *resource.Resource, current *resource.ResourceState) (*resource.ResourceState, error) {
	client, err := ecs.NewClientWithAccessKey(p.region, p.accessKey, p.secretKey)
	if err != nil {
		return nil, err
	}

	request := ecs.CreateModifySecurityGroupAttributeRequest()
	request.Scheme = "https"
	request.SecurityGroupId = current.ResourceID

	if v, ok := desired.Config["name"].(string); ok {
		request.SecurityGroupName = v
	}
	if v, ok := desired.Config["description"].(string); ok {
		request.Description = v
	}

	if _, err := client.ModifySecurityGroupAttribute(request); err != nil {
		return nil, fmt.Errorf("modify security group: %w", err)
	}

	return p.readSecurityGroup(ctx, &resource.ResourceState{
		Type:       "alicloud_security_group",
		ResourceID: current.ResourceID,
	})
}

func (p *Provider) deleteSecurityGroup(ctx context.Context, state *resource.ResourceState) error {
	client, err := ecs.NewClientWithAccessKey(p.region, p.accessKey, p.secretKey)
	if err != nil {
		return err
	}

	request := ecs.CreateDeleteSecurityGroupRequest()
	request.Scheme = "https"
	request.SecurityGroupId = state.ResourceID

	_, err = client.DeleteSecurityGroup(request)
	return err
}
