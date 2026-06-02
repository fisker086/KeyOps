package aliyun

import (
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/rds"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/slb"
	ossSDK "github.com/aliyun/aliyun-oss-go-sdk/oss"
)

func (a *AliyunAdapter) loadRDSDetails() {
	if a.rdsClient == nil {
		return
	}
	request := rds.CreateDescribeDBInstancesRequest()
	request.Scheme = "https"
	request.RegionId = a.region
	request.PageSize = requests.NewInteger(MaxResults)

	pageNum := 1
	for {
		request.PageNumber = requests.NewInteger(pageNum)
		response, err := a.rdsClient.DescribeDBInstances(request)
		if err != nil {
			return
		}

		for _, instance := range response.Items.DBInstance {
			a.rdsDetails[instance.DBInstanceId] = map[string]string{
				"InstanceName": instance.DBInstanceDescription,
			}
		}

		// PageRecordCount 为本页条数，必须用 TotalRecordCount 算总页数
		totalPages := (response.TotalRecordCount + MaxResults - 1) / MaxResults
		if totalPages < 1 {
			totalPages = 1
		}
		if pageNum >= totalPages {
			break
		}
		pageNum++
	}
}

func (a *AliyunAdapter) loadSLBDetails() {
	if a.slbClient == nil {
		return
	}
	request := slb.CreateDescribeLoadBalancersRequest()
	request.Scheme = "https"
	request.RegionId = a.region
	request.PageSize = requests.NewInteger(MaxResults)

	pageNum := 1
	for {
		request.PageNumber = requests.NewInteger(pageNum)
		response, err := a.slbClient.DescribeLoadBalancers(request)
		if err != nil {
			return
		}

		for _, lb := range response.LoadBalancers.LoadBalancer {
			a.slbDetails[lb.LoadBalancerId] = map[string]string{
				"InstanceName": lb.LoadBalancerName,
			}
		}

		totalPages := (response.TotalCount + MaxResults - 1) / MaxResults
		if pageNum >= totalPages {
			break
		}
		pageNum++
	}
}

func (a *AliyunAdapter) loadOSSDetails() {
	ossClient, err := ossSDK.New(a.region, a.accessKeyID, a.secretAccessKey)
	if err != nil {
		return
	}

	resp, err := ossClient.ListBuckets()
	if err != nil {
		return
	}

	for _, bucket := range resp.Buckets {
		a.ossDetails[bucket.Name] = map[string]string{
			"InstanceName": bucket.Name,
		}
	}
}
